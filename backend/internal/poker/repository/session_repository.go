// Package repository implementa con GORM la persistencia del módulo de Planning Poker.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// errNotImplemented es el error de las firmas mínimas de T014 que todavía no tienen implementación;
// las tareas de cada historia reemplazan cada cuerpo por su implementación.
var errNotImplemented = errors.New("not implemented")

// SessionRepository implementa domain.SessionRepository con GORM sobre estimation_sessions,
// estimation_rounds, estimation_participants y estimation_votes.
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository crea el repositorio de sesiones de estimación sobre la conexión recibida.
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// FindAccessible resuelve RequireEstimationAccess con una sola consulta (research.md sección 4): une
// estimation_sessions con su historia en user_stories y con la membresía vigente del titular en
// project_members. Si no hay fila, la sesión no existe o el titular no integra el proyecto de su
// historia (nunca lo integró o fue quitado, porque la baja borra la fila), y ambos casos se reportan
// igual con ErrSessionNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil).
func (r *SessionRepository) FindAccessible(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionAccess, error) {
	var access domain.SessionAccess
	err := r.db.WithContext(ctx).
		Table("estimation_sessions").
		Select(`estimation_sessions.id AS session_id, estimation_sessions.story_id AS story_id,
			user_stories.project_id AS project_id`).
		Joins("JOIN user_stories ON user_stories.id = estimation_sessions.story_id").
		Joins(`JOIN project_members ON project_members.project_id = user_stories.project_id
			AND project_members.user_id = ?`, userID).
		Where("estimation_sessions.id = ?", sessionID).
		Take(&access).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso a la sesión de estimación: %w", err)
	}
	return &access, nil
}

// Open abre la sesión en una sola transacción (research.md sección 6): bloquea la historia con
// lockOpenableStory, que rechaza la historia completada y el proyecto finalizado; con la historia ya
// serializada busca su sesión abierta y, si existe, devuelve OpenSessionExistsError con ella. Si no,
// inserta la sesión open con quien abre como facilitador y como quien la abrió, con
// last_activity_at = now y version 0, su ronda 1 en votación y a quien abre como participante
// conectado (FR-001 a FR-004, RN-02, RN-03). Los momentos de apertura salen de now, el reloj del
// servicio. El INSERT de la sesión va en un punto de guardado para que, si el índice parcial
// idx_estimation_sessions_one_open_per_story lo rechaza (gorm.ErrDuplicatedKey, gracias a
// TranslateError), la transacción siga utilizable y se relea la sesión abierta para devolver el mismo
// OpenSessionExistsError (SC-012).
func (r *SessionRepository) Open(ctx context.Context, storyID, openerID uuid.UUID, now time.Time) (*domain.EstimationSession, error) {
	session := domain.EstimationSession{
		ID:             uuid.New(),
		StoryID:        storyID,
		FacilitatorID:  openerID,
		Status:         domain.SessionStatusOpen,
		OpenedByID:     openerID,
		LastActivityAt: now,
		CreatedAt:      now,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOpenableStory(tx, storyID); err != nil {
			return err
		}
		if err := ensureNoOpenSession(tx, storyID); err != nil {
			return err
		}

		// La sesión se inserta en un punto de guardado; la violación del índice parcial se traduce a
		// la sesión abierta releída.
		err := tx.Transaction(func(inner *gorm.DB) error {
			return inner.Create(&session).Error
		})
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			if openErr := ensureNoOpenSession(tx, storyID); openErr != nil {
				return openErr
			}
			return fmt.Errorf("crear la sesión de estimación: %w", err)
		}
		if err != nil {
			return fmt.Errorf("crear la sesión de estimación: %w", err)
		}

		// La ronda 1 nace en votación y quien abre queda como participante conectado.
		round := domain.EstimationRound{
			ID:        uuid.New(),
			SessionID: session.ID,
			Number:    1,
			Status:    domain.RoundStatusVoting,
			CreatedAt: now,
		}
		if err := tx.Create(&round).Error; err != nil {
			return fmt.Errorf("crear la ronda 1 de la sesión: %w", err)
		}
		participant := domain.EstimationParticipant{
			ID:        uuid.New(),
			SessionID: session.ID,
			UserID:    openerID,
			Connected: true,
			CreatedAt: now,
		}
		if err := tx.Create(&participant).Error; err != nil {
			return fmt.Errorf("incorporar a quien abre la sesión: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// openableStoryRow es la fila que lee lockOpenableStory: el estado de la historia y el de su
// proyecto, que llega por el JOIN a projects con el alias project_status.
type openableStoryRow struct {
	StoryStatus   backlogdomain.StoryStatus
	ProjectStatus projectsdomain.ProjectStatus
}

// lockOpenableStory bloquea con FOR UPDATE, dentro de la transacción recibida, solo la fila de
// user_stories (no la del proyecto, que se lee por el JOIN), de modo que las aperturas simultáneas
// de la misma historia se serialicen, y con el estado releído rechaza la historia completada con
// ErrStoryCompleted (FR-007, RN-05) y el proyecto finalizado con ErrProjectFinished (FR-008). Si la
// historia ya no existe, porque se borró después de RequireStoryAccess, devuelve el ErrStoryNotFound
// de la 003.
func lockOpenableStory(tx *gorm.DB, storyID uuid.UUID) error {
	var row openableStoryRow
	err := tx.
		Table("user_stories").
		Select("user_stories.status AS story_status, projects.status AS project_status").
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("user_stories.id = ?", storyID).
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "user_stories"}}).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return backlogdomain.ErrStoryNotFound
	}
	if err != nil {
		return fmt.Errorf("bloquear la historia de la sesión: %w", err)
	}
	if row.StoryStatus == backlogdomain.StoryStatusCompleted {
		return domain.ErrStoryCompleted
	}
	if row.ProjectStatus == projectsdomain.ProjectStatusFinished {
		return domain.ErrProjectFinished
	}
	return nil
}

// ensureNoOpenSession busca, dentro de la transacción recibida, la sesión open de la historia y, si
// existe, devuelve OpenSessionExistsError con su identificador (FR-006, RN-04). Se lee con Find y
// LIMIT 1 para que la ausencia, que es el caso normal, no pase por gorm.ErrRecordNotFound. La usa
// Open antes del INSERT y al traducir la violación del índice parcial.
func ensureNoOpenSession(tx *gorm.DB, storyID uuid.UUID) error {
	var open []domain.EstimationSession
	err := tx.
		Select("id").
		Where("story_id = ? AND status = ?", storyID, domain.SessionStatusOpen).
		Limit(1).
		Find(&open).Error
	if err != nil {
		return fmt.Errorf("buscar la sesión abierta de la historia: %w", err)
	}
	if len(open) > 0 {
		return &domain.OpenSessionExistsError{SessionID: open[0].ID}
	}
	return nil
}

// Connect da de alta o reconecta al participante en una transacción que bloquea la sesión con
// lockOpenSession (FR-011 a FR-013, FR-015). Si la persona nunca estuvo en la sesión se inserta su
// fila conectada; si ya estaba, se reutiliza su fila (RN-34) marcándola conectada y borrando
// disconnected_at. Con la presencia ya actualizada evalúa la revelación automática (FR-031). La
// incorporación es un hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Connect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockOpenSession(tx, sessionID); err != nil {
			return err
		}

		// Se busca la fila previa de la persona con Find y LIMIT 1, porque no tenerla es un caso
		// normal.
		var existing []domain.EstimationParticipant
		err := tx.
			Select("id").
			Where("session_id = ? AND user_id = ?", sessionID, userID).
			Limit(1).
			Find(&existing).Error
		if err != nil {
			return fmt.Errorf("buscar al participante de la sesión: %w", err)
		}

		// Sin fila previa se inserta el participante conectado; con fila, se reconecta con un map
		// para que GORM no descarte ningún campo.
		if len(existing) == 0 {
			participant := domain.EstimationParticipant{
				ID:        uuid.New(),
				SessionID: sessionID,
				UserID:    userID,
				Connected: true,
				CreatedAt: now,
			}
			if err := tx.Create(&participant).Error; err != nil {
				return fmt.Errorf("incorporar al participante: %w", err)
			}
		} else {
			err := tx.
				Model(&domain.EstimationParticipant{}).
				Where("id = ?", existing[0].ID).
				Updates(map[string]any{"connected": true, "disconnected_at": nil}).Error
			if err != nil {
				return fmt.Errorf("reconectar al participante: %w", err)
			}
		}
		if err := autoRevealCurrentRound(tx, sessionID, now); err != nil {
			return err
		}
		return touchSession(tx, sessionID, now)
	})
}

// Disconnect marca desconectado al participante en una transacción que bloquea la sesión con
// lockOpenSession (FR-013, FR-014): fija connected = false con un map, porque GORM descarta el false
// de un struct, y disconnected_at = now. Si la persona no es participante de la sesión devuelve
// ErrNotParticipant. Sin la persona entre los conectados evalúa la revelación automática (FR-031).
// La desconexión es un hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Disconnect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockOpenSession(tx, sessionID); err != nil {
			return err
		}
		result := tx.
			Model(&domain.EstimationParticipant{}).
			Where("session_id = ? AND user_id = ?", sessionID, userID).
			Updates(map[string]any{"connected": false, "disconnected_at": now})
		if result.Error != nil {
			return fmt.Errorf("desconectar al participante: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotParticipant
		}
		if err := autoRevealCurrentRound(tx, sessionID, now); err != nil {
			return err
		}
		return touchSession(tx, sessionID, now)
	})
}

// lockOpenSession bloquea con SELECT … FOR UPDATE, dentro de la transacción recibida, la fila de la
// sesión, de modo que todas las acciones sobre ella se serialicen (research.md sección 6), y con el
// estado releído aplica EnsureOpen, que rechaza la sesión cerrada con SessionClosedError (RN-22). Si
// la sesión no existe devuelve ErrSessionNotFound. La comparten todas las escrituras sobre una
// sesión abierta; devuelve la sesión releída con su facilitator_id para las acciones reservadas y su
// story_id para el registro de la estimación.
func lockOpenSession(tx *gorm.DB, sessionID uuid.UUID) (*domain.EstimationSession, error) {
	var session domain.EstimationSession
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id, story_id, facilitator_id, status, cancel_reason, agreed_story_points").
		Where("id = ?", sessionID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloquear la sesión de estimación: %w", err)
	}
	if err := session.Status.EnsureOpen(session.CancelReason, session.AgreedStoryPoints); err != nil {
		return nil, err
	}
	return &session, nil
}

// touchSession registra un hecho de la sesión dentro de la transacción recibida: fija
// last_activity_at = now y sube version en uno en la misma sentencia (data-model.md sección 7).
func touchSession(tx *gorm.DB, sessionID uuid.UUID, now time.Time) error {
	err := tx.
		Model(&domain.EstimationSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{"last_activity_at": now, "version": gorm.Expr("version + 1")}).Error
	if err != nil {
		return fmt.Errorf("registrar el hecho de la sesión: %w", err)
	}
	return nil
}

// Vote registra o cambia la carta de un participante en una transacción que bloquea la sesión con
// lockOpenSession (FR-020 a FR-022, RN-07). Exige que quien vota sea participante conectado (si no,
// ErrNotParticipant) y que la ronda vigente, la de mayor número, siga en votación (si no,
// ErrRoundRevealed). El voto se guarda con un upsert sobre el índice único (round_id, participant_id):
// el primero inserta la fila y los siguientes actualizan card y updated_at sobre la misma fila,
// conservando su id y su created_at. Con el voto escrito evalúa la revelación automática (FR-031).
// El voto es un hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Vote(ctx context.Context, sessionID, userID uuid.UUID, card domain.Card, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockOpenSession(tx, sessionID); err != nil {
			return err
		}
		participantID, err := findConnectedParticipant(tx, sessionID, userID)
		if err != nil {
			return err
		}

		// La ronda vigente solo admite votos mientras está en votación.
		round, err := findCurrentRound(tx, sessionID)
		if err != nil {
			return err
		}
		if round.Status != domain.RoundStatusVoting {
			return domain.ErrRoundRevealed
		}

		// El upsert deja una sola fila por participante y ronda; ante el conflicto solo cambian la
		// carta y el momento de emisión.
		vote := domain.EstimationVote{
			ID:            uuid.New(),
			RoundID:       round.ID,
			ParticipantID: participantID,
			Card:          card,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		err = tx.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "round_id"}, {Name: "participant_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"card", "updated_at"}),
			}).
			Create(&vote).Error
		if err != nil {
			return fmt.Errorf("registrar el voto del participante: %w", err)
		}
		if err := autoRevealRound(tx, sessionID, round.ID, now); err != nil {
			return err
		}
		return touchSession(tx, sessionID, now)
	})
}

// Reveal revela a mano la ronda vigente en una transacción que bloquea la sesión con
// lockOpenSession (FR-032, FR-033, RN-10, RN-15). Exige que quien la pide sea participante conectado
// (si no, ErrNotParticipant) y el facilitador de la sesión (si no, ErrFacilitatorOnly), que la ronda
// vigente siga en votación (si no, ErrRoundRevealed) y que CanRevealManually admita sus votos, que
// cuentan aunque su autor esté desconectado. La revelación es un hecho, así que touchSession fija
// last_activity_at = now y sube version.
func (r *SessionRepository) Reveal(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := lockOpenSession(tx, sessionID)
		if err != nil {
			return err
		}
		if _, err := findConnectedParticipant(tx, sessionID, userID); err != nil {
			return err
		}
		if session.FacilitatorID != userID {
			return domain.ErrFacilitatorOnly
		}

		// La ronda vigente tiene que estar en votación y tener al menos un voto.
		round, err := findCurrentRound(tx, sessionID)
		if err != nil {
			return err
		}
		if round.Status != domain.RoundStatusVoting {
			return domain.ErrRoundRevealed
		}
		votes, err := findRoundVotes(tx, round.ID)
		if err != nil {
			return err
		}
		if err := domain.CanRevealManually(votes); err != nil {
			return err
		}

		if err := revealRound(tx, round.ID, now); err != nil {
			return err
		}
		return touchSession(tx, sessionID, now)
	})
}

// findConnectedParticipant devuelve, dentro de la transacción recibida, el identificador de la fila
// conectada de la persona en la sesión, o ErrNotParticipant si no la tiene (FR-028). Se lee con Find
// y LIMIT 1, porque no tenerla es un rechazo esperado y no un error de la base.
func findConnectedParticipant(tx *gorm.DB, sessionID, userID uuid.UUID) (uuid.UUID, error) {
	var participants []domain.EstimationParticipant
	err := tx.
		Select("id").
		Where("session_id = ? AND user_id = ? AND connected = ?", sessionID, userID, true).
		Limit(1).
		Find(&participants).Error
	if err != nil {
		return uuid.Nil, fmt.Errorf("buscar al participante conectado: %w", err)
	}
	if len(participants) == 0 {
		return uuid.Nil, domain.ErrNotParticipant
	}
	return participants[0].ID, nil
}

// findCurrentRound lee, dentro de la transacción recibida, la ronda vigente de la sesión, que es la
// de mayor número, con su identificador, su número y su estado.
func findCurrentRound(tx *gorm.DB, sessionID uuid.UUID) (*domain.EstimationRound, error) {
	var round domain.EstimationRound
	err := tx.
		Select("id, number, status").
		Where("session_id = ?", sessionID).
		Order("number DESC").
		Take(&round).Error
	if err != nil {
		return nil, fmt.Errorf("leer la ronda vigente de la sesión: %w", err)
	}
	return &round, nil
}

// findRoundVotes lee, dentro de la transacción recibida, los autores de los votos de la ronda; la
// lista vacía, y no nil, indica que nadie votó.
func findRoundVotes(tx *gorm.DB, roundID uuid.UUID) ([]domain.EstimationVote, error) {
	votes := []domain.EstimationVote{}
	err := tx.
		Select("participant_id").
		Where("round_id = ?", roundID).
		Find(&votes).Error
	if err != nil {
		return nil, fmt.Errorf("leer los votos de la ronda vigente: %w", err)
	}
	return votes, nil
}

// autoRevealCurrentRound evalúa la revelación automática sobre la ronda vigente de la sesión si
// sigue en votación; una ronda ya revelada no cambia. La usan Connect y Disconnect, que no leen la
// ronda por su cuenta.
func autoRevealCurrentRound(tx *gorm.DB, sessionID uuid.UUID, now time.Time) error {
	round, err := findCurrentRound(tx, sessionID)
	if err != nil {
		return err
	}
	if round.Status != domain.RoundStatusVoting {
		return nil
	}
	return autoRevealRound(tx, sessionID, round.ID, now)
}

// autoRevealRound revela la ronda en votación recibida si ShouldAutoReveal lo indica con la
// presencia y los votos releídos dentro de la transacción (FR-030, FR-031). No toca la versión: la
// revelación queda dentro del mismo hecho que la disparó.
func autoRevealRound(tx *gorm.DB, sessionID, roundID uuid.UUID, now time.Time) error {
	var participants []domain.EstimationParticipant
	err := tx.
		Select("id, connected").
		Where("session_id = ?", sessionID).
		Find(&participants).Error
	if err != nil {
		return fmt.Errorf("leer la presencia de la sesión: %w", err)
	}
	votes, err := findRoundVotes(tx, roundID)
	if err != nil {
		return err
	}
	if !domain.ShouldAutoReveal(participants, votes) {
		return nil
	}
	return revealRound(tx, roundID, now)
}

// revealRound escribe la revelación de la ronda dentro de la transacción recibida: status =
// revealed y revealed_at = now. La comparten la revelación manual y la automática.
func revealRound(tx *gorm.DB, roundID uuid.UUID, now time.Time) error {
	err := tx.
		Model(&domain.EstimationRound{}).
		Where("id = ?", roundID).
		Updates(map[string]any{"status": domain.RoundStatusRevealed, "revealed_at": now}).Error
	if err != nil {
		return fmt.Errorf("revelar la ronda vigente: %w", err)
	}
	return nil
}

// StartRound abre la ronda siguiente en una transacción que bloquea la sesión con lockOpenSession
// (FR-043 a FR-048, RN-11). Exige que quien la pide sea participante conectado (si no,
// ErrNotParticipant) y el facilitador de la sesión (si no, ErrFacilitatorOnly), y que la ronda
// vigente ya esté revelada (si no, ErrRoundNotRevealed). La ronda nueva se inserta con el número
// siguiente, en votación y sin votos; las anteriores quedan intactas como historial. Abrir la ronda
// es un hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) StartRound(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := lockOpenSession(tx, sessionID)
		if err != nil {
			return err
		}
		if _, err := findConnectedParticipant(tx, sessionID, userID); err != nil {
			return err
		}
		if session.FacilitatorID != userID {
			return domain.ErrFacilitatorOnly
		}

		// La ronda vigente tiene que estar revelada para abrir la siguiente.
		current, err := findCurrentRound(tx, sessionID)
		if err != nil {
			return err
		}
		if current.Status != domain.RoundStatusRevealed {
			return domain.ErrRoundNotRevealed
		}

		// La ronda siguiente nace en votación con el número que sigue al de la vigente.
		round := domain.EstimationRound{
			ID:        uuid.New(),
			SessionID: sessionID,
			Number:    current.Number + 1,
			Status:    domain.RoundStatusVoting,
			CreatedAt: now,
		}
		if err := tx.Create(&round).Error; err != nil {
			return fmt.Errorf("crear la ronda siguiente de la sesión: %w", err)
		}
		return touchSession(tx, sessionID, now)
	})
}

// Finalize cierra la sesión con el valor acordado en una transacción que bloquea la sesión con
// lockOpenSession (FR-049 a FR-056). Exige que quien registra sea participante conectado (si no,
// ErrNotParticipant) y el facilitador de la sesión (si no, ErrFacilitatorOnly), y que la sesión tenga
// al menos una ronda revelada (si no, ErrNoRevealedRound). Recién después bloquea la fila de la
// historia, respetando el orden sesión → historia (research.md sección 6). Cierra la sesión como
// finished con el valor acordado, quien la cerró y closed_at = now, y sube version sin tocar
// last_activity_at, porque el registro cierra la sesión (data-model.md sección 7). En la misma
// transacción escribe story_points y updated_at en la historia (SC-013).
func (r *SessionRepository) Finalize(ctx context.Context, sessionID, userID uuid.UUID, storyPoints int, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := lockOpenSession(tx, sessionID)
		if err != nil {
			return err
		}
		if _, err := findConnectedParticipant(tx, sessionID, userID); err != nil {
			return err
		}
		if session.FacilitatorID != userID {
			return domain.ErrFacilitatorOnly
		}
		if err := ensureRevealedRound(tx, sessionID); err != nil {
			return err
		}
		if err := lockSessionStory(tx, session.StoryID); err != nil {
			return err
		}

		// La sesión se cierra con un map para escribir todos los campos del cierre y subir version en
		// la misma sentencia.
		err = tx.
			Model(&domain.EstimationSession{}).
			Where("id = ?", sessionID).
			Updates(map[string]any{
				"status":              domain.SessionStatusFinished,
				"agreed_story_points": storyPoints,
				"closed_by_id":        userID,
				"closed_at":           now,
				"version":             gorm.Expr("version + 1"),
			}).Error
		if err != nil {
			return fmt.Errorf("cerrar la sesión con el valor acordado: %w", err)
		}

		// El valor acordado reemplaza los Story Points de la historia en la misma transacción.
		err = tx.
			Table("user_stories").
			Where("id = ?", session.StoryID).
			Updates(map[string]any{"story_points": storyPoints, "updated_at": now}).Error
		if err != nil {
			return fmt.Errorf("escribir el valor acordado en la historia: %w", err)
		}
		return nil
	})
}

// ensureRevealedRound verifica, dentro de la transacción recibida, que la sesión tenga al menos una
// ronda revelada, o devuelve ErrNoRevealedRound (FR-051). Se lee con Find y LIMIT 1, porque no
// tenerla es un rechazo esperado y no un error de la base.
func ensureRevealedRound(tx *gorm.DB, sessionID uuid.UUID) error {
	var rounds []domain.EstimationRound
	err := tx.
		Select("id").
		Where("session_id = ? AND status = ?", sessionID, domain.RoundStatusRevealed).
		Limit(1).
		Find(&rounds).Error
	if err != nil {
		return fmt.Errorf("buscar una ronda revelada de la sesión: %w", err)
	}
	if len(rounds) == 0 {
		return domain.ErrNoRevealedRound
	}
	return nil
}

// lockSessionStory bloquea con FOR UPDATE, dentro de la transacción recibida, la fila de la historia
// de la sesión. Si la historia ya no existe devuelve el ErrStoryNotFound de la 003.
func lockSessionStory(tx *gorm.DB, storyID uuid.UUID) error {
	var stories []backlogdomain.UserStory
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ?", storyID).
		Find(&stories).Error
	if err != nil {
		return fmt.Errorf("bloquear la historia de la sesión: %w", err)
	}
	if len(stories) == 0 {
		return backlogdomain.ErrStoryNotFound
	}
	return nil
}

// Cancel cancela la sesión a pedido del facilitador. Firma mínima de T014.
func (r *SessionRepository) Cancel(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// TakeFacilitator transfiere el rol de facilitador. Firma mínima de T014.
func (r *SessionRepository) TakeFacilitator(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// AutoCancel cancela la sesión si corresponde una cancelación del sistema. Firma mínima de T014.
func (r *SessionRepository) AutoCancel(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	return false, errNotImplemented
}

// ResetPresence marca desconectados, al arrancar el servidor, a los participantes que figuran
// conectados en sesiones open, con disconnected_at = now, porque ninguna conexión sobrevive al
// reinicio (research.md sección 9). Los que ya estaban desconectados conservan su disconnected_at.
// No es un hecho: no toca last_activity_at ni version de las sesiones (data-model.md sección 7).
func (r *SessionRepository) ResetPresence(ctx context.Context, now time.Time) error {
	db := r.db.WithContext(ctx)
	openSessions := db.
		Model(&domain.EstimationSession{}).
		Select("id").
		Where("status = ?", domain.SessionStatusOpen)
	err := db.
		Model(&domain.EstimationParticipant{}).
		Where("connected = ? AND session_id IN (?)", true, openSessions).
		Updates(map[string]any{"connected": false, "disconnected_at": now}).Error
	if err != nil {
		return fmt.Errorf("reiniciar la presencia de las sesiones abiertas: %w", err)
	}
	return nil
}

// ListOpenForSweep devuelve las sesiones abiertas para el barrido. Firma mínima de T014.
func (r *SessionRepository) ListOpenForSweep(ctx context.Context) ([]domain.OpenSessionStatus, error) {
	return nil, errNotImplemented
}

// ListRevokedParticipants devuelve los participantes conectados que ya no son integrantes. Firma
// mínima de T014.
func (r *SessionRepository) ListRevokedParticipants(ctx context.Context) ([]domain.ParticipantRef, error) {
	return nil, errNotImplemented
}

// FindSnapshot lee la instantánea completa de la sesión, entrada de ViewFor (research.md sección 7),
// en una transacción de solo lectura REPEATABLE READ para que todas las consultas vean el mismo
// estado aunque otra acción confirme en el medio. Lee la sesión (si no existe devuelve
// ErrSessionNotFound), la historia con sus criterios en orden y sin story_points (FR-012, FR-075),
// el facilitador, quien abrió y quien cerró, los participantes y las rondas de la 1 a la vigente con
// sus votos. Toda persona se lee con un JOIN o una consulta a users que selecciona solo users.id y
// users.full_name, de modo que el hash de la contraseña nunca entra al módulo (Principio VI).
func (r *SessionRepository) FindSnapshot(ctx context.Context, sessionID uuid.UUID) (*domain.SessionSnapshot, error) {
	var snapshot domain.SessionSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("id = ?", sessionID).Take(&snapshot.Session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrSessionNotFound
		}
		if err != nil {
			return fmt.Errorf("leer la sesión de estimación: %w", err)
		}

		story, err := findSessionStory(tx, snapshot.Session.StoryID)
		if err != nil {
			return err
		}
		snapshot.Story = *story

		if err := fillSessionPeople(tx, &snapshot); err != nil {
			return err
		}
		participants, err := findSnapshotParticipants(tx, sessionID)
		if err != nil {
			return err
		}
		snapshot.Participants = participants

		rounds, err := findSnapshotRounds(tx, sessionID)
		if err != nil {
			return err
		}
		snapshot.Rounds = rounds
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// sessionStoryRow es la fila de user_stories que lee findSessionStory, sin story_points (FR-075).
type sessionStoryRow struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Title       string
	Description *string
}

// findSessionStory lee, dentro de la transacción recibida, el texto vigente de la historia y los
// textos de sus criterios de aceptación ordenados por posición (FR-012, FR-074). Una historia sin
// criterios devuelve la lista vacía y no nil.
func findSessionStory(tx *gorm.DB, storyID uuid.UUID) (*domain.SessionStory, error) {
	var row sessionStoryRow
	err := tx.
		Table("user_stories").
		Select("id, project_id, title, description").
		Where("id = ?", storyID).
		Take(&row).Error
	if err != nil {
		return nil, fmt.Errorf("leer la historia de la sesión: %w", err)
	}

	// Los criterios se leen en su orden de posición.
	criteria := []string{}
	err = tx.
		Model(&backlogdomain.AcceptanceCriterion{}).
		Where("story_id = ?", storyID).
		Order("position ASC").
		Pluck("text", &criteria).Error
	if err != nil {
		return nil, fmt.Errorf("leer los criterios de la historia de la sesión: %w", err)
	}
	return &domain.SessionStory{
		ID:                 row.ID,
		ProjectID:          row.ProjectID,
		Title:              row.Title,
		Description:        row.Description,
		AcceptanceCriteria: criteria,
	}, nil
}

// fillSessionPeople completa en la instantánea el facilitador, quien abrió y, si la sesión la cerró
// una persona, quien cerró, leyendo de users solo id y full_name en una sola consulta.
func fillSessionPeople(tx *gorm.DB, snapshot *domain.SessionSnapshot) error {
	session := snapshot.Session
	ids := []uuid.UUID{session.FacilitatorID, session.OpenedByID}
	if session.ClosedByID != nil {
		ids = append(ids, *session.ClosedByID)
	}
	var refs []domain.UserRef
	err := tx.
		Table("users").
		Select("id, full_name").
		Where("id IN ?", ids).
		Find(&refs).Error
	if err != nil {
		return fmt.Errorf("leer las personas de la sesión: %w", err)
	}

	// Cada rol se resuelve desde el índice por identificador; una persona ausente es una
	// inconsistencia de la base, porque las claves foráneas la impiden.
	byID := make(map[uuid.UUID]domain.UserRef, len(refs))
	for _, ref := range refs {
		byID[ref.ID] = ref
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("leer las personas de la sesión: falta el usuario %s", id)
		}
	}
	snapshot.Facilitator = byID[session.FacilitatorID]
	snapshot.OpenedBy = byID[session.OpenedByID]
	if session.ClosedByID != nil {
		closedBy := byID[*session.ClosedByID]
		snapshot.ClosedBy = &closedBy
	}
	return nil
}

// snapshotParticipantRow es la fila que lee findSnapshotParticipants: el participante con el
// identificador y el nombre de la persona, que llegan por el JOIN a users con alias.
type snapshotParticipantRow struct {
	UserID         uuid.UUID
	UserFullName   string
	Connected      bool
	DisconnectedAt *time.Time
}

// findSnapshotParticipants lee, dentro de la transacción recibida, los participantes de la sesión en
// su orden de incorporación (y luego por id, para que el orden sea determinista), con un JOIN a
// users que selecciona solo id y full_name.
func findSnapshotParticipants(tx *gorm.DB, sessionID uuid.UUID) ([]domain.SnapshotParticipant, error) {
	var rows []snapshotParticipantRow
	err := tx.
		Table("estimation_participants").
		Select(`users.id AS user_id, users.full_name AS user_full_name,
			estimation_participants.connected, estimation_participants.disconnected_at`).
		Joins("JOIN users ON users.id = estimation_participants.user_id").
		Where("estimation_participants.session_id = ?", sessionID).
		Order("estimation_participants.created_at ASC, estimation_participants.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer los participantes de la sesión: %w", err)
	}
	participants := make([]domain.SnapshotParticipant, 0, len(rows))
	for _, row := range rows {
		participants = append(participants, domain.SnapshotParticipant{
			User:           domain.UserRef{ID: row.UserID, FullName: row.UserFullName},
			Connected:      row.Connected,
			DisconnectedAt: row.DisconnectedAt,
		})
	}
	return participants, nil
}

// snapshotVoteRow es la fila que lee findSnapshotRounds para los votos: la ronda, la carta y el
// identificador y el nombre de su autor, que llegan por el JOIN a users con alias.
type snapshotVoteRow struct {
	RoundID      uuid.UUID
	UserID       uuid.UUID
	UserFullName string
	Card         domain.Card
}

// findSnapshotRounds lee, dentro de la transacción recibida, las rondas de la sesión ordenadas por
// número y, en una segunda consulta, todos sus votos con su autor por un JOIN a
// estimation_participants y a users que selecciona solo id y full_name, sin ocultar ninguno: el
// secreto del voto lo aplica ViewFor (research.md sección 8). Los votos de cada ronda quedan en el
// orden en que se emitieron.
func findSnapshotRounds(tx *gorm.DB, sessionID uuid.UUID) ([]domain.SnapshotRound, error) {
	var rounds []domain.EstimationRound
	err := tx.
		Select("id, number, status, revealed_at, created_at").
		Where("session_id = ?", sessionID).
		Order("number ASC").
		Find(&rounds).Error
	if err != nil {
		return nil, fmt.Errorf("leer las rondas de la sesión: %w", err)
	}

	// Los votos de todas las rondas se leen juntos y se reparten por ronda.
	var votes []snapshotVoteRow
	err = tx.
		Table("estimation_votes").
		Select(`estimation_votes.round_id, users.id AS user_id, users.full_name AS user_full_name,
			estimation_votes.card`).
		Joins("JOIN estimation_rounds ON estimation_rounds.id = estimation_votes.round_id").
		Joins("JOIN estimation_participants ON estimation_participants.id = estimation_votes.participant_id").
		Joins("JOIN users ON users.id = estimation_participants.user_id").
		Where("estimation_rounds.session_id = ?", sessionID).
		Order("estimation_votes.created_at ASC, estimation_votes.id ASC").
		Find(&votes).Error
	if err != nil {
		return nil, fmt.Errorf("leer los votos de la sesión: %w", err)
	}
	votesByRound := make(map[uuid.UUID][]domain.SessionVote, len(rounds))
	for _, v := range votes {
		votesByRound[v.RoundID] = append(votesByRound[v.RoundID], domain.SessionVote{
			User: domain.UserRef{ID: v.UserID, FullName: v.UserFullName},
			Card: v.Card,
		})
	}

	// Cada ronda se arma con su número, su estado, sus momentos y sus votos.
	snapshotRounds := make([]domain.SnapshotRound, 0, len(rounds))
	for _, round := range rounds {
		snapshotRounds = append(snapshotRounds, domain.SnapshotRound{
			Number:     round.Number,
			Status:     round.Status,
			OpenedAt:   round.CreatedAt,
			RevealedAt: round.RevealedAt,
			Votes:      votesByRound[round.ID],
		})
	}
	return snapshotRounds, nil
}

// ListForStory devuelve las sesiones cerradas de la historia. Firma mínima de T014.
func (r *SessionRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.SessionSummary, error) {
	return nil, errNotImplemented
}

// FindOpenForStory devuelve la sesión abierta de la historia. Firma mínima de T014.
func (r *SessionRepository) FindOpenForStory(ctx context.Context, storyID uuid.UUID) (*domain.OpenSessionSummary, error) {
	return nil, errNotImplemented
}

// Verificación en compilación de que el repositorio cumple el contrato del dominio.
var _ domain.SessionRepository = (*SessionRepository)(nil)

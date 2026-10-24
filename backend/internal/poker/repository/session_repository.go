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

// openableStoryRow es la fila que leen lockOpenableStory y autoCancelLocked: el estado de la historia
// y el de su proyecto, que llega por el JOIN a projects con el alias project_status.
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

// Connect da de alta o reconecta al participante con el paso común de withOpenSession (FR-011 a
// FR-013, FR-015). Si la persona nunca estuvo en la sesión se inserta su fila conectada; si ya
// estaba, se reutiliza su fila (RN-34) marcándola conectada y borrando disconnected_at. Con la
// presencia ya actualizada evalúa la revelación automática (FR-031). La incorporación es un hecho,
// así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Connect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, _ *domain.EstimationSession) error {
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

// Disconnect marca desconectado al participante con el paso común de withOpenSession (FR-013,
// FR-014): fija connected = false con un map, porque GORM descarta el false de un struct, y
// disconnected_at = now. Si la persona no es participante de la sesión devuelve ErrNotParticipant.
// Sin la persona entre los conectados evalúa la revelación automática (FR-031). La desconexión es un
// hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Disconnect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, _ *domain.EstimationSession) error {
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

// withOpenSession es el paso común de todas las escrituras sobre una sesión abierta (data-model.md
// sección 9): en una transacción bloquea la sesión con lockOpenSession, que aplica EnsureOpen, y con
// autoCancelLocked reevalúa AutoCancelReason. Si corresponde una cancelación del sistema, la
// transacción confirma esa cancelación sin ejecutar la acción y se devuelve SessionClosedError con el
// motivo (research.md sección 13); si no, ejecuta la acción con la sesión releída dentro de la misma
// transacción.
func (r *SessionRepository) withOpenSession(
	ctx context.Context,
	sessionID uuid.UUID,
	now time.Time,
	action func(tx *gorm.DB, session *domain.EstimationSession) error,
) error {
	var closedErr error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := lockOpenSession(tx, sessionID)
		if err != nil {
			return err
		}
		reason, canceled, err := autoCancelLocked(tx, session, now)
		if err != nil {
			return err
		}
		if canceled {
			closedErr = &domain.SessionClosedError{Status: domain.SessionStatusCanceled, CancelReason: &reason}
			return nil
		}
		return action(tx, session)
	})
	if err != nil {
		return err
	}
	return closedErr
}

// autoCancelLocked lee, dentro de la transacción que ya bloqueó la sesión, el estado de su historia y
// el de su proyecto, y con el last_activity_at releído decide con AutoCancelReason si el sistema debe
// cancelarla (FR-061, FR-063). Si corresponde, la cancela sin quien cerró y con el motivo, y devuelve
// el motivo y verdadero. Si la historia ya no existe devuelve el ErrStoryNotFound de la 003.
func autoCancelLocked(tx *gorm.DB, session *domain.EstimationSession, now time.Time) (domain.CancelReason, bool, error) {
	var row openableStoryRow
	err := tx.
		Table("user_stories").
		Select("user_stories.status AS story_status, projects.status AS project_status").
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("user_stories.id = ?", session.StoryID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, backlogdomain.ErrStoryNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("leer el estado de la historia de la sesión: %w", err)
	}

	// La regla pura decide el motivo; sin motivo la sesión sigue abierta.
	projectFinished := row.ProjectStatus == projectsdomain.ProjectStatusFinished
	reason, cancel := domain.AutoCancelReason(row.StoryStatus, projectFinished, session.LastActivityAt, now)
	if !cancel {
		return "", false, nil
	}
	if err := cancelSession(tx, session.ID, nil, &reason, now); err != nil {
		return "", false, err
	}
	return reason, true, nil
}

// cancelSession cierra la sesión como canceled dentro de la transacción recibida, con closed_at = now,
// y sube version sin tocar last_activity_at, porque la cancelación cierra la sesión (data-model.md
// sección 7). En la cancelación del facilitador recibe closedByID y ningún motivo; en la del sistema,
// el motivo y ningún closedByID, que quedan en NULL.
func cancelSession(tx *gorm.DB, sessionID uuid.UUID, closedByID *uuid.UUID, reason *domain.CancelReason, now time.Time) error {
	values := map[string]any{
		"status":        domain.SessionStatusCanceled,
		"closed_by_id":  nil,
		"cancel_reason": nil,
		"closed_at":     now,
		"version":       gorm.Expr("version + 1"),
	}
	if closedByID != nil {
		values["closed_by_id"] = *closedByID
	}
	if reason != nil {
		values["cancel_reason"] = *reason
	}
	err := tx.
		Model(&domain.EstimationSession{}).
		Where("id = ?", sessionID).
		Updates(values).Error
	if err != nil {
		return fmt.Errorf("cancelar la sesión de estimación: %w", err)
	}
	return nil
}

// lockOpenSession bloquea con SELECT … FOR UPDATE, dentro de la transacción recibida, la fila de la
// sesión, de modo que todas las acciones sobre ella se serialicen (research.md sección 6), y con el
// estado releído aplica EnsureOpen, que rechaza la sesión cerrada con SessionClosedError (RN-22). Si
// la sesión no existe devuelve ErrSessionNotFound. Devuelve la sesión releída con su facilitator_id
// para las acciones reservadas, su story_id para el registro de la estimación y su last_activity_at
// para la cancelación por inactividad.
func lockOpenSession(tx *gorm.DB, sessionID uuid.UUID) (*domain.EstimationSession, error) {
	var session domain.EstimationSession
	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id, story_id, facilitator_id, status, cancel_reason, agreed_story_points, last_activity_at").
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

// Vote registra o cambia la carta de un participante con el paso común de withOpenSession (FR-020 a
// FR-022, RN-07). Exige que quien vota sea participante conectado (si no,
// ErrNotParticipant) y que la ronda vigente, la de mayor número, siga en votación (si no,
// ErrRoundRevealed). El voto se guarda con un upsert sobre el índice único (round_id, participant_id):
// el primero inserta la fila y los siguientes actualizan card y updated_at sobre la misma fila,
// conservando su id y su created_at. Con el voto escrito evalúa la revelación automática (FR-031).
// El voto es un hecho, así que touchSession fija last_activity_at = now y sube version.
func (r *SessionRepository) Vote(ctx context.Context, sessionID, userID uuid.UUID, card domain.Card, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, _ *domain.EstimationSession) error {
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

// Reveal revela a mano la ronda vigente con el paso común de withOpenSession (FR-032, FR-033, RN-10,
// RN-15). Exige que quien la pide sea participante conectado (si no, ErrNotParticipant) y el
// facilitador de la sesión (si no, ErrFacilitatorOnly), que la ronda vigente siga en votación (si
// no, ErrRoundRevealed) y que CanRevealManually admita sus votos, que cuentan aunque su autor esté
// desconectado. La revelación es un hecho, así que touchSession fija last_activity_at = now y sube
// version.
func (r *SessionRepository) Reveal(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, session *domain.EstimationSession) error {
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

// StartRound abre la ronda siguiente con el paso común de withOpenSession (FR-043 a FR-048, RN-11).
// Exige que quien la pide sea participante conectado (si no, ErrNotParticipant) y el facilitador de
// la sesión (si no, ErrFacilitatorOnly), y que la ronda vigente ya esté revelada (si no,
// ErrRoundNotRevealed). La ronda nueva se inserta con el número siguiente, en votación y sin votos;
// las anteriores quedan intactas como historial. Abrir la ronda es un hecho, así que touchSession
// fija last_activity_at = now y sube version.
func (r *SessionRepository) StartRound(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, session *domain.EstimationSession) error {
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

// Finalize cierra la sesión con el valor acordado con el paso común de withOpenSession (FR-049 a
// FR-056). Exige que quien registra sea participante conectado (si no, ErrNotParticipant) y el
// facilitador de la sesión (si no, ErrFacilitatorOnly), y que la sesión tenga al menos una ronda
// revelada (si no, ErrNoRevealedRound). Recién después bloquea la fila de la historia, respetando el
// orden sesión → historia (research.md sección 6). Cierra la sesión como finished con el valor
// acordado, quien la cerró y closed_at = now, y sube version sin tocar last_activity_at, porque el
// registro cierra la sesión (data-model.md sección 7). En la misma transacción escribe story_points y
// updated_at en la historia (SC-013).
func (r *SessionRepository) Finalize(ctx context.Context, sessionID, userID uuid.UUID, storyPoints int, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, session *domain.EstimationSession) error {
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
		err := tx.
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

// Cancel cancela la sesión a pedido del facilitador con el paso común de withOpenSession (FR-057 a
// FR-060). Exige que quien la pide sea participante conectado (si no, ErrNotParticipant) y el
// facilitador de la sesión (si no, ErrFacilitatorOnly). Cierra la sesión como canceled con
// closed_by_id = titular, cancel_reason = NULL y closed_at = now, y sube version; la historia, sus
// rondas y sus votos no se tocan (FR-058).
func (r *SessionRepository) Cancel(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, session *domain.EstimationSession) error {
		if _, err := findConnectedParticipant(tx, sessionID, userID); err != nil {
			return err
		}
		if session.FacilitatorID != userID {
			return domain.ErrFacilitatorOnly
		}
		return cancelSession(tx, sessionID, &userID, nil, now)
	})
}

// TakeFacilitator transfiere el rol de facilitador a quien lo pide con el paso común de
// withOpenSession (FR-068 a FR-072, RN-25). Exige que quien lo pide sea participante conectado (si
// no, ErrNotParticipant) y relee, bajo el bloqueo, la fila del facilitador vigente para que
// FacilitatorTakeover decida con su presencia actual: entre dos pedidos simultáneos, el segundo ya
// lee al nuevo facilitador conectado y recibe FacilitatorUnavailableError con su nombre (FR-072). Si
// lo admite, fija facilitator_id = titular; el anterior queda como participante común (FR-070). El
// cambio de facilitador es un hecho, así que touchSession fija last_activity_at = now y sube
// version.
func (r *SessionRepository) TakeFacilitator(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error {
	return r.withOpenSession(ctx, sessionID, now, func(tx *gorm.DB, session *domain.EstimationSession) error {
		if _, err := findConnectedParticipant(tx, sessionID, userID); err != nil {
			return err
		}
		facilitator, err := findFacilitatorParticipant(tx, sessionID, session.FacilitatorID)
		if err != nil {
			return err
		}
		if err := domain.FacilitatorTakeover(now, *facilitator, userID); err != nil {
			return err
		}

		// El rol pasa a quien lo pidió en la misma transacción que lo verificó.
		err = tx.
			Model(&domain.EstimationSession{}).
			Where("id = ?", sessionID).
			Update("facilitator_id", userID).Error
		if err != nil {
			return fmt.Errorf("transferir el rol de facilitador: %w", err)
		}
		return touchSession(tx, sessionID, now)
	})
}

// findFacilitatorParticipant lee, dentro de la transacción recibida, la fila de participante del
// facilitador vigente con su presencia y, por un JOIN a users que selecciona solo id y full_name, su
// nombre, que FacilitatorUnavailableError expone. Quien abre la sesión queda como participante y solo
// un participante puede tomar el rol, así que la ausencia de la fila es una inconsistencia de la base.
func findFacilitatorParticipant(tx *gorm.DB, sessionID, facilitatorID uuid.UUID) (*domain.SnapshotParticipant, error) {
	var rows []snapshotParticipantRow
	err := tx.
		Table("estimation_participants").
		Select(`users.id AS user_id, users.full_name AS user_full_name,
			estimation_participants.connected, estimation_participants.disconnected_at`).
		Joins("JOIN users ON users.id = estimation_participants.user_id").
		Where("estimation_participants.session_id = ? AND estimation_participants.user_id = ?", sessionID, facilitatorID).
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("leer al facilitador de la sesión: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("leer al facilitador de la sesión: falta el participante %s", facilitatorID)
	}
	return &domain.SnapshotParticipant{
		User:           domain.UserRef{ID: rows[0].UserID, FullName: rows[0].UserFullName},
		Connected:      rows[0].Connected,
		DisconnectedAt: rows[0].DisconnectedAt,
	}, nil
}

// AutoCancel aplica, a pedido del barrido, la cancelación del sistema (FR-061, FR-063; research.md
// sección 13): en una transacción bloquea la sesión y reevalúa AutoCancelReason con autoCancelLocked
// sobre el estado releído, porque entre la lista del barrido y el bloqueo pudo ocurrir un hecho.
// Informa si la canceló. Una sesión que ya se cerró en el medio no tiene nada que cancelar y devuelve
// falso sin error.
func (r *SessionRepository) AutoCancel(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error) {
	var canceled bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := lockOpenSession(tx, sessionID)
		var closed *domain.SessionClosedError
		if errors.As(err, &closed) {
			return nil
		}
		if err != nil {
			return err
		}
		_, canceled, err = autoCancelLocked(tx, session, now)
		return err
	})
	if err != nil {
		return false, err
	}
	return canceled, nil
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

// ListOpenForSweep devuelve las sesiones abiertas para el barrido (research.md sección 13): el
// identificador y last_activity_at de la sesión y, por un JOIN a user_stories y a projects, el estado
// de la historia, si el proyecto está finalizado, que alimentan AutoCancelReason (FR-061, FR-063), y
// el updated_at de la historia, que el servicio compara con el último visto (FR-074). Sin sesiones
// abiertas devuelve la lista vacía.
func (r *SessionRepository) ListOpenForSweep(ctx context.Context) ([]domain.OpenSessionStatus, error) {
	sessions := []domain.OpenSessionStatus{}
	err := r.db.WithContext(ctx).
		Table("estimation_sessions").
		Select(`estimation_sessions.id AS session_id, user_stories.status AS story_status,
			projects.status = ? AS project_finished, estimation_sessions.last_activity_at,
			user_stories.updated_at AS story_updated_at`, projectsdomain.ProjectStatusFinished).
		Joins("JOIN user_stories ON user_stories.id = estimation_sessions.story_id").
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("estimation_sessions.status = ?", domain.SessionStatusOpen).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("listar las sesiones abiertas para el barrido: %w", err)
	}
	return sessions, nil
}

// ListRevokedParticipants devuelve los participantes conectados de las sesiones abiertas que ya no
// integran el proyecto de la historia, para que el barrido los desconecte y cierre sus canales
// (FR-019; research.md sección 13). Une cada participante con su sesión y su historia, y con un LEFT
// JOIN a project_members detecta la membresía ausente, porque la baja de la 002 borra la fila. Solo
// selecciona los identificadores de la sesión y de la persona; sin revocados devuelve la lista vacía.
func (r *SessionRepository) ListRevokedParticipants(ctx context.Context) ([]domain.ParticipantRef, error) {
	refs := []domain.ParticipantRef{}
	err := r.db.WithContext(ctx).
		Table("estimation_participants").
		Select("estimation_participants.session_id, estimation_participants.user_id").
		Joins("JOIN estimation_sessions ON estimation_sessions.id = estimation_participants.session_id").
		Joins("JOIN user_stories ON user_stories.id = estimation_sessions.story_id").
		Joins(`LEFT JOIN project_members ON project_members.project_id = user_stories.project_id
			AND project_members.user_id = estimation_participants.user_id`).
		Where("estimation_sessions.status = ? AND estimation_participants.connected = ?", domain.SessionStatusOpen, true).
		Where("project_members.user_id IS NULL").
		Find(&refs).Error
	if err != nil {
		return nil, fmt.Errorf("listar los participantes con la membresía revocada: %w", err)
	}
	return refs, nil
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

// sessionSummaryRow es la fila que lee ListForStory: la sesión cerrada con el nombre de quien la abrió
// y de quien la cerró, que llegan por los JOIN a users con alias, y su cantidad de rondas.
type sessionSummaryRow struct {
	ID                uuid.UUID
	Status            domain.SessionStatus
	AgreedStoryPoints *int
	CancelReason      *domain.CancelReason
	OpenedByID        uuid.UUID
	OpenedByFullName  string
	OpenedAt          time.Time
	ClosedByID        *uuid.UUID
	ClosedByFullName  *string
	ClosedAt          time.Time
	RoundCount        int
}

// ListForStory devuelve las sesiones cerradas de la historia, de la más reciente a la más antigua por
// su momento de apertura y luego por id, para que el orden sea determinista (FR-079, FR-080;
// research.md sección 16). Se resuelve en una sola consulta: los nombres de quien abrió y de quien
// cerró salen de users con un JOIN (LEFT JOIN para quien cerró, nulo si cerró el sistema) que
// selecciona solo id y full_name y no filtra por membresía, de modo que una ex integrante conserva su
// nombre (FR-084); la cantidad de rondas sale de una subconsulta. Sin sesiones cerradas devuelve la
// lista vacía.
func (r *SessionRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.SessionSummary, error) {
	var rows []sessionSummaryRow
	err := r.db.WithContext(ctx).
		Table("estimation_sessions").
		Select(`estimation_sessions.id, estimation_sessions.status, estimation_sessions.agreed_story_points,
			estimation_sessions.cancel_reason, opener.id AS opened_by_id, opener.full_name AS opened_by_full_name,
			estimation_sessions.created_at AS opened_at, closer.id AS closed_by_id,
			closer.full_name AS closed_by_full_name, estimation_sessions.closed_at,
			(SELECT COUNT(*) FROM estimation_rounds
				WHERE estimation_rounds.session_id = estimation_sessions.id) AS round_count`).
		Joins("JOIN users AS opener ON opener.id = estimation_sessions.opened_by_id").
		Joins("LEFT JOIN users AS closer ON closer.id = estimation_sessions.closed_by_id").
		Where("estimation_sessions.story_id = ? AND estimation_sessions.status <> ?", storyID, domain.SessionStatusOpen).
		Order("estimation_sessions.created_at DESC, estimation_sessions.id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listar las sesiones cerradas de la historia: %w", err)
	}

	// Cada fila se convierte al resumen del dominio; quien cerró queda nil si la cerró el sistema.
	summaries := make([]domain.SessionSummary, 0, len(rows))
	for _, row := range rows {
		summary := domain.SessionSummary{
			ID:                row.ID,
			Status:            row.Status,
			AgreedStoryPoints: row.AgreedStoryPoints,
			CancelReason:      row.CancelReason,
			OpenedBy:          domain.UserRef{ID: row.OpenedByID, FullName: row.OpenedByFullName},
			OpenedAt:          row.OpenedAt,
			ClosedAt:          row.ClosedAt,
			RoundCount:        row.RoundCount,
		}
		if row.ClosedByID != nil && row.ClosedByFullName != nil {
			summary.ClosedBy = &domain.UserRef{ID: *row.ClosedByID, FullName: *row.ClosedByFullName}
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// openSessionSummaryRow es la fila que lee FindOpenForStory: la sesión abierta con el nombre de su
// facilitador y el número y el estado de su ronda vigente, que llegan por los JOIN con alias.
type openSessionSummaryRow struct {
	ID                  uuid.UUID
	FacilitatorID       uuid.UUID
	FacilitatorFullName string
	RoundNumber         int
	RoundStatus         domain.RoundStatus
}

// FindOpenForStory devuelve la sesión abierta de la historia con su facilitador y el número y el
// estado de la ronda vigente, la de mayor número, sin ningún voto (FR-081, FR-082). El facilitador se
// lee de users seleccionando solo id y full_name. Si la historia no tiene una sesión abierta devuelve
// nil sin error, porque es opcional.
func (r *SessionRepository) FindOpenForStory(ctx context.Context, storyID uuid.UUID) (*domain.OpenSessionSummary, error) {
	var row openSessionSummaryRow
	err := r.db.WithContext(ctx).
		Table("estimation_sessions").
		Select(`estimation_sessions.id, users.id AS facilitator_id, users.full_name AS facilitator_full_name,
			estimation_rounds.number AS round_number, estimation_rounds.status AS round_status`).
		Joins("JOIN users ON users.id = estimation_sessions.facilitator_id").
		Joins(`JOIN estimation_rounds ON estimation_rounds.session_id = estimation_sessions.id
			AND estimation_rounds.number = (SELECT MAX(latest.number) FROM estimation_rounds AS latest
				WHERE latest.session_id = estimation_sessions.id)`).
		Where("estimation_sessions.story_id = ? AND estimation_sessions.status = ?", storyID, domain.SessionStatusOpen).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer la sesión abierta de la historia: %w", err)
	}
	return &domain.OpenSessionSummary{
		ID:          row.ID,
		Facilitator: domain.UserRef{ID: row.FacilitatorID, FullName: row.FacilitatorFullName},
		RoundNumber: row.RoundNumber,
		RoundStatus: row.RoundStatus,
	}, nil
}

// Verificación en compilación de que el repositorio cumple el contrato del dominio.
var _ domain.SessionRepository = (*SessionRepository)(nil)

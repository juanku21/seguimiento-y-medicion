// Package service implementa la lógica del módulo de Planning Poker: sesiones, presencia, difusión
// y barrido periódico.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// SessionService orquesta las sesiones de estimación sobre el repositorio, con el reloj inyectado
// para que las pruebas controlen la gracia, el relevo y la caducidad (research.md secciones 7, 9 y
// 13). Guarda en memoria la presencia por (sesión, usuario), el hub de suscripciones y un mutex por
// sesión que ordena las publicaciones de una misma sesión. El barrido anota además las personas cuya
// membresía revocó, para que sus canales envíen removed al cerrarse, y el último updated_at visto de
// la historia de cada sesión abierta, para detectar sus cambios (research.md sección 13).
type SessionService struct {
	repo     domain.SessionRepository
	now      func() time.Time
	presence *Presence
	hub      *Hub

	locksMu sync.Mutex
	locks   map[uuid.UUID]*sync.Mutex

	revokedMu sync.Mutex
	revoked   map[PresenceKey]struct{}

	storySeenMu sync.Mutex
	storySeen   map[uuid.UUID]time.Time
}

// Connection es una conexión abierta de una persona al canal de una sesión: Views entrega las vistas
// de la sesión para esa persona y se cierra cuando el hub da de baja la suscripción. Se libera con
// Leave.
type Connection struct {
	SessionID   uuid.UUID
	UserID      uuid.UUID
	Views       <-chan domain.SessionView
	unsubscribe func()
	removed     func() bool
}

// Removed informa si el barrido cerró la conexión porque la persona dejó de ser integrante del
// proyecto (FR-019); el canal lo consulta al cerrarse Views para enviar el evento removed.
func (c *Connection) Removed() bool {
	return c.removed()
}

// NewSessionService crea el servicio de sesiones sobre el repositorio y el reloj recibidos, con la
// presencia y el hub vacíos.
func NewSessionService(repo domain.SessionRepository, now func() time.Time) *SessionService {
	return &SessionService{
		repo:      repo,
		now:       now,
		presence:  NewPresence(),
		hub:       NewHub(),
		locks:     map[uuid.UUID]*sync.Mutex{},
		revoked:   map[PresenceKey]struct{}{},
		storySeen: map[uuid.UUID]time.Time{},
	}
}

// Open abre la sesión de estimación de la historia con quien la pide como facilitador y
// participante conectado, y devuelve la vista para esa persona. Su presencia queda pendiente desde
// la apertura: si no abre el canal dentro de la gracia, el barrido la desconecta (FR-002). No se
// publica, porque una sesión recién creada todavía no tiene suscripciones.
func (s *SessionService) Open(ctx context.Context, storyID, openerID uuid.UUID) (*domain.SessionView, error) {
	now := s.now()
	session, err := s.repo.Open(ctx, storyID, openerID, now)
	if err != nil {
		return nil, err
	}
	s.presence.MarkPending(session.ID, openerID, now)

	snapshot, err := s.repo.FindSnapshot(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("reading opened session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, openerID)
	return &view, nil
}

// Join abre una conexión de la persona al canal de la sesión: la suscribe al hub y cuenta la
// conexión en la presencia. Si es una llegada (su primera conexión fuera de la gracia), registra la
// incorporación con Connect. Siempre publica después, para que la conexión nueva reciba el estado
// vigente como primera vista (FR-077). Quien llega pasó el control de membresía, así que se borra
// una revocación anterior. Si algo falla, deshace la suscripción y la conexión contada.
func (s *SessionService) Join(ctx context.Context, access *domain.SessionAccess, userID uuid.UUID) (*Connection, error) {
	sessionID := access.SessionID
	key := PresenceKey{SessionID: sessionID, UserID: userID}
	s.setRevoked(key, false)
	views, unsubscribe := s.hub.Subscribe(sessionID, userID)
	conn := &Connection{
		SessionID:   sessionID,
		UserID:      userID,
		Views:       views,
		unsubscribe: unsubscribe,
		removed:     func() bool { return s.isRevoked(key) },
	}

	if s.presence.Connect(sessionID, userID) {
		if err := s.repo.Connect(ctx, sessionID, userID, s.now()); err != nil {
			s.Leave(conn)
			return nil, s.rejectAction(ctx, sessionID, err)
		}
	}
	if err := s.publish(ctx, sessionID); err != nil {
		s.Leave(conn)
		return nil, err
	}
	return conn, nil
}

// Vote registra o cambia la carta de la persona en la ronda vigente y devuelve la sesión resultante
// vista por ella. Recién después de que el repositorio confirma la transacción publica la
// instantánea, para que nadie reciba un voto que no quedó escrito (research.md sección 7). Los
// errores del repositorio se devuelven tal cual, para que el manejador los traduzca.
func (s *SessionService) Vote(ctx context.Context, sessionID, userID uuid.UUID, card domain.Card) (*domain.SessionView, error) {
	if err := s.repo.Vote(ctx, sessionID, userID, card, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.publish(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading voted session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// Reveal revela a mano la ronda vigente a pedido del facilitador y devuelve la sesión resultante
// vista por quien la pidió. Igual que Vote, publica recién después de que el repositorio confirma la
// transacción (research.md sección 7) y devuelve tal cual los errores del repositorio, para que el
// manejador los traduzca.
func (s *SessionService) Reveal(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionView, error) {
	if err := s.repo.Reveal(ctx, sessionID, userID, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.publish(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading revealed session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// StartRound abre la ronda siguiente a pedido del facilitador y devuelve la sesión resultante vista
// por quien la pidió. Igual que Vote y Reveal, publica recién después de que el repositorio confirma
// la transacción (research.md sección 7) y devuelve tal cual los errores del repositorio, para que el
// manejador los traduzca.
func (s *SessionService) StartRound(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionView, error) {
	if err := s.repo.StartRound(ctx, sessionID, userID, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.publish(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading started round session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// Finalize registra el valor acordado a pedido del facilitador: cierra la sesión como finished y deja
// los Story Points de la historia en ese valor, y devuelve la sesión resultante vista por quien la
// cerró. Después de que el repositorio confirma la transacción publica el estado final y recién
// entonces cierra las suscripciones de la sesión, así cada canal recibe el último state antes de
// terminar (research.md sección 7). Los errores del repositorio se devuelven tal cual, para que el
// manejador los traduzca.
func (s *SessionService) Finalize(ctx context.Context, sessionID, userID uuid.UUID, storyPoints int) (*domain.SessionView, error) {
	if err := s.repo.Finalize(ctx, sessionID, userID, storyPoints, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.closeSession(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading finalized session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// Cancel cancela la sesión a pedido del facilitador sin registrar estimación: la historia no se toca
// y las rondas y los votos se conservan (FR-057, FR-058). Igual que Finalize, publica el estado final
// después de que el repositorio confirma la transacción y recién entonces cierra las suscripciones
// de la sesión, y devuelve la sesión resultante vista por quien la canceló. Los errores del
// repositorio se devuelven tal cual, para que el manejador los traduzca.
func (s *SessionService) Cancel(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionView, error) {
	if err := s.repo.Cancel(ctx, sessionID, userID, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.closeSession(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading canceled session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// TakeFacilitator transfiere el rol de facilitador a quien lo pide, si el facilitador vigente lleva
// 5 minutos o más desconectado (FR-068, FR-070), y devuelve la sesión resultante vista por quien lo
// tomó. Igual que las demás acciones, publica recién después de que el repositorio confirma la
// transacción (research.md sección 7) y devuelve tal cual los errores del repositorio, incluido
// FacilitatorUnavailableError, para que el manejador los traduzca.
func (s *SessionService) TakeFacilitator(ctx context.Context, sessionID, userID uuid.UUID) (*domain.SessionView, error) {
	if err := s.repo.TakeFacilitator(ctx, sessionID, userID, s.now()); err != nil {
		return nil, s.rejectAction(ctx, sessionID, err)
	}
	if err := s.publish(ctx, sessionID); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("reading taken facilitator session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, userID)
	return &view, nil
}

// Get devuelve la sesión a la que el middleware ya dio acceso, con todas sus rondas, vista por quien
// la consulta. Es solo lectura: no une a la persona a la sesión ni publica nada.
func (s *SessionService) Get(ctx context.Context, access *domain.SessionAccess, viewerID uuid.UUID) (*domain.SessionView, error) {
	snapshot, err := s.repo.FindSnapshot(ctx, access.SessionID)
	if err != nil {
		return nil, fmt.Errorf("reading session snapshot: %w", err)
	}
	view := domain.ViewFor(*snapshot, viewerID)
	return &view, nil
}

// Leave da de baja la suscripción de la conexión y la descuenta de la presencia; si era la última
// de la persona, empieza a correr la gracia de FR-014.
func (s *SessionService) Leave(conn *Connection) {
	conn.unsubscribe()
	s.presence.Release(conn.SessionID, conn.UserID, s.now())
}

// publish lee la instantánea de la sesión y entrega a cada suscripción la vista de su usuario. Lo
// hace bajo el mutex de la sesión, así dos publicaciones seguidas leen y entregan estados en orden
// (research.md sección 7).
func (s *SessionService) publish(ctx context.Context, sessionID uuid.UUID) error {
	lock := s.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	snapshot, err := s.repo.FindSnapshot(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("reading session snapshot to publish: %w", err)
	}
	s.hub.Publish(sessionID, func(userID uuid.UUID) domain.SessionView {
		return domain.ViewFor(*snapshot, userID)
	})
	return nil
}

// closeSession publica el estado final de una sesión que acaba de cerrarse y después cierra sus
// suscripciones, así cada canal recibe el último state antes de terminar (research.md sección 7).
func (s *SessionService) closeSession(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.publish(ctx, sessionID); err != nil {
		return err
	}
	s.hub.CloseSession(sessionID)
	return nil
}

// closeIfAutoCanceled publica y cierra los canales de la sesión cuando err es el SessionClosedError
// de una cancelación del sistema, que el repositorio pudo confirmar en esa misma operación
// (data-model.md sección 9), para que los conectados se enteren sin esperar al barrido. Devuelve solo
// el error de la publicación; ante cualquier otro err no hace nada.
func (s *SessionService) closeIfAutoCanceled(ctx context.Context, sessionID uuid.UUID, err error) error {
	var closed *domain.SessionClosedError
	if !errors.As(err, &closed) || closed.CancelReason == nil {
		return nil
	}
	return s.closeSession(ctx, sessionID)
}

// rejectAction avisa la cancelación del sistema que pudo producir una acción rechazada y devuelve el
// rechazo original, que es lo que el manejador traduce. Una falla al avisar se registra con slog en
// lugar de devolverse, porque la respuesta a quien actuó no depende de ella.
func (s *SessionService) rejectAction(ctx context.Context, sessionID uuid.UUID, err error) error {
	if closeErr := s.closeIfAutoCanceled(ctx, sessionID, err); closeErr != nil {
		slog.ErrorContext(ctx, "notifying estimation session auto cancel failed", "error", closeErr)
	}
	return err
}

// setRevoked anota o borra la revocación de la membresía de la persona en la sesión.
func (s *SessionService) setRevoked(key PresenceKey, revoked bool) {
	s.revokedMu.Lock()
	defer s.revokedMu.Unlock()

	if revoked {
		s.revoked[key] = struct{}{}
		return
	}
	delete(s.revoked, key)
}

// isRevoked informa si el barrido revocó la membresía de la persona en la sesión.
func (s *SessionService) isRevoked(key PresenceKey) bool {
	s.revokedMu.Lock()
	defer s.revokedMu.Unlock()

	_, ok := s.revoked[key]
	return ok
}

// sessionLock devuelve el mutex de publicación de la sesión, creándolo la primera vez.
func (s *SessionService) sessionLock(sessionID uuid.UUID) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()

	lock, ok := s.locks[sessionID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[sessionID] = lock
	}
	return lock
}

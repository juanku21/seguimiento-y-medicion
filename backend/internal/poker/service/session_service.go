// Package service implementa la lógica del módulo de Planning Poker: sesiones, presencia, difusión
// y barrido periódico.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// SessionService orquesta las sesiones de estimación sobre el repositorio, con el reloj inyectado
// para que las pruebas controlen la gracia, el relevo y la caducidad (research.md secciones 7, 9 y
// 13). Guarda en memoria la presencia por (sesión, usuario), el hub de suscripciones y un mutex por
// sesión que ordena las publicaciones de una misma sesión.
type SessionService struct {
	repo     domain.SessionRepository
	now      func() time.Time
	presence *Presence
	hub      *Hub

	locksMu sync.Mutex
	locks   map[uuid.UUID]*sync.Mutex
}

// Connection es una conexión abierta de una persona al canal de una sesión: Views entrega las vistas
// de la sesión para esa persona y se cierra cuando el hub da de baja la suscripción. Se libera con
// Leave.
type Connection struct {
	SessionID   uuid.UUID
	UserID      uuid.UUID
	Views       <-chan domain.SessionView
	unsubscribe func()
}

// NewSessionService crea el servicio de sesiones sobre el repositorio y el reloj recibidos, con la
// presencia y el hub vacíos.
func NewSessionService(repo domain.SessionRepository, now func() time.Time) *SessionService {
	return &SessionService{
		repo:     repo,
		now:      now,
		presence: NewPresence(),
		hub:      NewHub(),
		locks:    map[uuid.UUID]*sync.Mutex{},
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
// vigente como primera vista (FR-077). Si algo falla, deshace la suscripción y la conexión contada.
func (s *SessionService) Join(ctx context.Context, access *domain.SessionAccess, userID uuid.UUID) (*Connection, error) {
	sessionID := access.SessionID
	views, unsubscribe := s.hub.Subscribe(sessionID, userID)
	conn := &Connection{SessionID: sessionID, UserID: userID, Views: views, unsubscribe: unsubscribe}

	if s.presence.Connect(sessionID, userID) {
		if err := s.repo.Connect(ctx, sessionID, userID, s.now()); err != nil {
			s.Leave(conn)
			return nil, err
		}
	}
	if err := s.publish(ctx, sessionID); err != nil {
		s.Leave(conn)
		return nil, err
	}
	return conn, nil
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

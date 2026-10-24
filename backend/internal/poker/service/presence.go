package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// PresenceGrace es la gracia de FR-014: quien pierde su última conexión sigue contando como
// conectado hasta que pasan 30 segundos desde la caída.
const PresenceGrace = 30 * time.Second

// PresenceKey identifica a una persona dentro de una sesión: la presencia se cuenta por par
// (sesión, usuario), de modo que la misma persona en dos sesiones son dos entradas distintas.
type PresenceKey struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

// presenceEntry guarda las conexiones abiertas de un par y, si no le queda ninguna, el instante
// en que cayó la última.
type presenceEntry struct {
	connections int
	dropped     bool
	droppedAt   time.Time
}

// Presence cuenta las conexiones abiertas por par (sesión, usuario) y recuerda el momento en que
// cayó la última, para aplicar la gracia de FR-014 (research.md sección 9, RN-23 y RN-34).
type Presence struct {
	mu      sync.Mutex
	entries map[PresenceKey]*presenceEntry
}

// NewPresence crea un registro de presencia vacío.
func NewPresence() *Presence {
	return &Presence{entries: map[PresenceKey]*presenceEntry{}}
}

// Connect cuenta una conexión nueva del par e informa si es una llegada: la primera conexión de
// alguien sin una caída anotada dentro de la gracia. Si el par ya figuraba, la conexión borra la
// caída pendiente sin informar llegada.
func (p *Presence) Connect(sessionID, userID uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := PresenceKey{SessionID: sessionID, UserID: userID}
	entry, ok := p.entries[key]
	if !ok {
		p.entries[key] = &presenceEntry{connections: 1}
		return true
	}
	entry.connections++
	entry.dropped = false
	return false
}

// Release descuenta una conexión del par; si era la última, anota el instante de la caída. Un par
// desconocido o sin conexiones se ignora.
func (p *Presence) Release(sessionID, userID uuid.UUID, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.entries[PresenceKey{SessionID: sessionID, UserID: userID}]
	if !ok || entry.connections == 0 {
		return
	}
	entry.connections--
	if entry.connections == 0 {
		entry.dropped = true
		entry.droppedAt = now
	}
}

// MarkPending anota una caída en el instante dado para quien figura conectado sin canal abierto,
// como quien abre la sesión (FR-002). Si el par ya tiene conexiones vivas no cambia nada.
func (p *Presence) MarkPending(sessionID, userID uuid.UUID, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := PresenceKey{SessionID: sessionID, UserID: userID}
	entry, ok := p.entries[key]
	if !ok {
		entry = &presenceEntry{}
		p.entries[key] = entry
	}
	if entry.connections == 0 {
		entry.dropped = true
		entry.droppedAt = now
	}
}

// Expired devuelve y quita los pares cuya última conexión cayó hace la gracia o más; la siguiente
// conexión de esos pares vuelve a contar como llegada.
func (p *Presence) Expired(now time.Time) []PresenceKey {
	p.mu.Lock()
	defer p.mu.Unlock()

	expired := []PresenceKey{}
	for key, entry := range p.entries {
		if entry.dropped && now.Sub(entry.droppedAt) >= PresenceGrace {
			expired = append(expired, key)
			delete(p.entries, key)
		}
	}
	return expired
}

// Forget quita el par del registro, con sus conexiones y su caída pendiente, para que el barrido no
// vuelva a desconectar a quien ya desconectó por otra vía, como la membresía revocada (FR-019). Las
// conexiones que se liberen después se ignoran, porque Release no conoce el par.
func (p *Presence) Forget(sessionID, userID uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.entries, PresenceKey{SessionID: sessionID, UserID: userID})
}

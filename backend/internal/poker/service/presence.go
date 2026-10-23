package service

import (
	"time"

	"github.com/google/uuid"
)

// PresenceKey identifica a una persona dentro de una sesión: la presencia se cuenta por par
// (sesión, usuario), de modo que la misma persona en dos sesiones son dos entradas distintas.
type PresenceKey struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

// Presence cuenta las conexiones abiertas por par (sesión, usuario) y recuerda el momento en que
// cayó la última, para aplicar la gracia de FR-014 (research.md sección 9, RN-23 y RN-34).
type Presence struct{}

// NewPresence crea un registro de presencia vacío.
func NewPresence() *Presence {
	return &Presence{}
}

// Connect cuenta una conexión nueva del par e informa si es una llegada: la primera conexión de
// alguien sin una caída anotada dentro de la gracia.
func (p *Presence) Connect(sessionID, userID uuid.UUID) bool {
	return false
}

// Release descuenta una conexión del par; si era la última, anota el instante de la caída.
func (p *Presence) Release(sessionID, userID uuid.UUID, now time.Time) {}

// MarkPending anota una caída en el instante dado para quien figura conectado sin canal abierto,
// como quien abre la sesión (FR-002).
func (p *Presence) MarkPending(sessionID, userID uuid.UUID, now time.Time) {}

// Expired devuelve y quita los pares cuya última conexión cayó hace la gracia o más.
func (p *Presence) Expired(now time.Time) []PresenceKey {
	return nil
}

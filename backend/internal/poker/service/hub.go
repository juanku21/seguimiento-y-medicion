package service

import (
	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Hub reparte las instantáneas de cada sesión entre sus suscripciones abiertas. Cada suscripción
// tiene un canal de capacidad 1 con semántica "la última gana": si hay una vista pendiente sin leer,
// la nueva la reemplaza (research.md sección 7).
type Hub struct{}

// NewHub crea un hub sin suscripciones.
func NewHub() *Hub {
	return &Hub{}
}

// Subscribe registra una suscripción de la persona en la sesión y devuelve su canal y la función
// para darse de baja, que puede llamarse más de una vez.
func (h *Hub) Subscribe(sessionID, userID uuid.UUID) (<-chan domain.SessionView, func()) {
	return nil, func() {}
}

// Publish entrega a cada suscripción de la sesión la vista que viewFor arma para su usuario.
func (h *Hub) Publish(sessionID uuid.UUID, viewFor func(userID uuid.UUID) domain.SessionView) {}

// CloseSession cierra y quita todas las suscripciones de la sesión.
func (h *Hub) CloseSession(sessionID uuid.UUID) {}

// CloseUser cierra y quita las suscripciones de una persona en una sesión.
func (h *Hub) CloseUser(sessionID, userID uuid.UUID) {}

package service

import (
	"sync"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// subscription es una conexión abierta de una persona en una sesión, con su canal de capacidad 1.
type subscription struct {
	userID uuid.UUID
	ch     chan domain.SessionView
}

// Hub reparte las instantáneas de cada sesión entre sus suscripciones abiertas. Cada suscripción
// tiene un canal de capacidad 1 con semántica "la última gana": si hay una vista pendiente sin leer,
// la nueva la reemplaza (research.md sección 7). Una suscripción figura en el mapa mientras su canal
// está abierto; quitarla y cerrar su canal ocurren juntos bajo el mutex, así que nunca se envía a un
// canal cerrado ni se lo cierra dos veces.
type Hub struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]map[*subscription]struct{}
}

// NewHub crea un hub sin suscripciones.
func NewHub() *Hub {
	return &Hub{sessions: map[uuid.UUID]map[*subscription]struct{}{}}
}

// Subscribe registra una suscripción de la persona en la sesión y devuelve su canal y la función
// para darse de baja, que puede llamarse más de una vez. La baja cierra el canal si seguía abierto.
func (h *Hub) Subscribe(sessionID, userID uuid.UUID) (<-chan domain.SessionView, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub := &subscription{userID: userID, ch: make(chan domain.SessionView, 1)}
	subs, ok := h.sessions[sessionID]
	if !ok {
		subs = map[*subscription]struct{}{}
		h.sessions[sessionID] = subs
	}
	subs[sub] = struct{}{}

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(sessionID, sub)
	}
	return sub.ch, unsubscribe
}

// Publish entrega a cada suscripción de la sesión la vista que viewFor arma para su usuario, sin
// bloquearse: si el canal tiene una vista pendiente, la descarta y deja la nueva.
func (h *Hub) Publish(sessionID uuid.UUID, viewFor func(userID uuid.UUID) domain.SessionView) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.sessions[sessionID] {
		view := viewFor(sub.userID)
		select {
		case sub.ch <- view:
			continue
		default:
		}
		// El canal está lleno: se vacía la vista pendiente (si el lector no la tomó antes) y se
		// deja la nueva. Solo Publish envía y lo hace bajo el mutex, así que el segundo envío
		// siempre encuentra lugar.
		select {
		case <-sub.ch:
		default:
		}
		sub.ch <- view
	}
}

// CloseSession cierra y quita todas las suscripciones de la sesión.
func (h *Hub) CloseSession(sessionID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.sessions[sessionID] {
		close(sub.ch)
	}
	delete(h.sessions, sessionID)
}

// CloseUser cierra y quita las suscripciones de una persona en una sesión.
func (h *Hub) CloseUser(sessionID, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.sessions[sessionID] {
		if sub.userID == userID {
			h.remove(sessionID, sub)
		}
	}
}

// remove quita la suscripción y cierra su canal si todavía figuraba; borra la sesión del mapa
// cuando se queda sin suscripciones. Debe llamarse con el mutex tomado.
func (h *Hub) remove(sessionID uuid.UUID, sub *subscription) {
	subs, ok := h.sessions[sessionID]
	if !ok {
		return
	}
	if _, ok := subs[sub]; !ok {
		return
	}
	delete(subs, sub)
	close(sub.ch)
	if len(subs) == 0 {
		delete(h.sessions, sessionID)
	}
}

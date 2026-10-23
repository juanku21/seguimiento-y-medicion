package delivery

import (
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/service"
)

// Handler agrupa los manejadores HTTP del módulo de Planning Poker sobre el servicio de sesiones.
// Firma mínima de T015: los manejadores los implementa T026.
type Handler struct{}

// NewHandler crea los manejadores sobre el servicio de sesiones recibido. Firma mínima de T015.
func NewHandler(sessions *service.SessionService) *Handler {
	return &Handler{}
}

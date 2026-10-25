package delivery

import (
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
)

// Handler reúne los manejadores HTTP del registro de esfuerzo. Firma mínima de T015: la
// implementación corresponde a T029.
type Handler struct{}

// NewHandler compone los manejadores sobre el servicio de esfuerzo. Firma mínima de T015: la
// implementación corresponde a T029.
func NewHandler(efforts *service.EffortService) *Handler {
	return nil
}

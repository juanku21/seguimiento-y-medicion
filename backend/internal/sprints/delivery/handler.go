package delivery

import (
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Handler agrupa los manejadores HTTP del módulo de sprints sobre el servicio de sprints. Firma
// mínima de T013: los manejadores se implementan desde T020.
type Handler struct {
	sprints *service.SprintService
}

// NewHandler compone los manejadores de sprints sobre el servicio recibido.
func NewHandler(sprints *service.SprintService) *Handler {
	return &Handler{sprints: sprints}
}

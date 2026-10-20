// Package delivery expone por HTTP el backlog de un proyecto.
package delivery

import (
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
)

// Handler reúne los manejadores HTTP del backlog. Firma mínima para las pruebas de integración de
// T015; los campos y el manejador createStory corresponden a T024.
type Handler struct{}

// NewHandler crea los manejadores del backlog sobre el servicio de historias. Firma mínima para las
// pruebas de T015.
func NewHandler(stories *service.StoryService) *Handler {
	return &Handler{}
}

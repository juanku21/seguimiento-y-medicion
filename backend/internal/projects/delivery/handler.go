package delivery

import "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"

// Handler agrupa los manejadores HTTP del módulo de proyectos sobre el servicio. Firma mínima de
// T019: el manejador createProject lo implementa T027.
type Handler struct {
	projects *service.ProjectService
}

// NewHandler crea el conjunto de manejadores con el servicio de proyectos recibido.
func NewHandler(projects *service.ProjectService) *Handler {
	return &Handler{projects: projects}
}

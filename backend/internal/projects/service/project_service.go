// Package service reúne los casos de uso del módulo de proyectos.
package service

import "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"

// ProjectService reúne los casos de uso de los proyectos sobre el repositorio y los puertos de
// backlog y de sprints del dominio. Firma mínima de T019: la creación la implementa T026.
type ProjectService struct {
	projects domain.ProjectRepository
	backlog  domain.BacklogSummaryProvider
	sprints  domain.SprintDirectory
}

// NewProjectService compone el servicio con el repositorio de proyectos y los dos puertos.
func NewProjectService(projects domain.ProjectRepository, backlog domain.BacklogSummaryProvider, sprints domain.SprintDirectory) *ProjectService {
	return &ProjectService{projects: projects, backlog: backlog, sprints: sprints}
}

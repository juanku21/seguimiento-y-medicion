// Package service reúne los casos de uso del módulo de proyectos.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectService reúne los casos de uso de los proyectos sobre los repositorios de proyectos y de
// integrantes y los puertos de backlog y de sprints del dominio.
type ProjectService struct {
	projects domain.ProjectRepository
	members  domain.ProjectMemberRepository
	backlog  domain.BacklogSummaryProvider
	sprints  domain.SprintDirectory
}

// NewProjectService compone el servicio con los repositorios de proyectos y de integrantes y los dos
// puertos.
func NewProjectService(projects domain.ProjectRepository, members domain.ProjectMemberRepository, backlog domain.BacklogSummaryProvider, sprints domain.SprintDirectory) *ProjectService {
	return &ProjectService{projects: projects, members: members, backlog: backlog, sprints: sprints}
}

// CreateProjectInput son los datos de un proyecto nuevo ya validados y normalizados por delivery
// antes de llegar al servicio (Principio VII). NameNormalized llega calculado para que la regla de
// normalización del nombre viva en un solo lugar (RN-05), la descripción ya es nula si quedó vacía
// (FR-003) y el factor está expresado en centésimas de hora (RN-07).
type CreateProjectInput struct {
	Name                         string
	NameNormalized               string
	Description                  *string
	StartDate                    time.Time
	PlannedEndDate               time.Time
	HoursPerStoryPointHundredths int
}

// Create crea el proyecto del propietario en estado planned y sin fecha de finalización real, junto
// con la membresía que lo deja como primer integrante (FR-007, FR-008, RN-01), y devuelve su vista.
// Los dos registros se persisten en una sola transacción del repositorio (FR-010); un nombre ya
// usado por el mismo propietario llega como ErrProjectNameTaken y se propaga envuelto para que
// delivery lo reconozca con errors.Is (FR-004, FR-011).
func (s *ProjectService) Create(ctx context.Context, ownerID uuid.UUID, input CreateProjectInput) (*ProjectView, error) {
	project := &domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         input.Name,
		NameNormalized:               input.NameNormalized,
		Description:                  input.Description,
		StartDate:                    input.StartDate,
		PlannedEndDate:               input.PlannedEndDate,
		ActualEndDate:                nil,
		HoursPerStoryPointHundredths: input.HoursPerStoryPointHundredths,
		Status:                       domain.ProjectStatusPlanned,
	}
	ownerMembership := &domain.ProjectMember{
		ID:        uuid.New(),
		ProjectID: project.ID,
		UserID:    ownerID,
	}
	if err := s.projects.Create(ctx, project, ownerMembership); err != nil {
		return nil, fmt.Errorf("crear proyecto: %w", err)
	}

	// El propietario es el único integrante del proyecto recién creado y su fecha de incorporación
	// es la de la membresía, que GORM completó al insertarla.
	members := []domain.MemberView{{
		UserID:   ownerID,
		IsOwner:  true,
		JoinedAt: ownerMembership.CreatedAt,
	}}
	return s.buildView(ctx, project, members)
}

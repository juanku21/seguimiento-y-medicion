// Package service contiene la lógica de negocio del backlog: crea, lista, consulta, estima,
// modifica y elimina historias de usuario apoyándose en las interfaces de backlog/domain
// (Principio III).
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// StoryService reúne los casos de uso del backlog sobre el repositorio de historias y los cinco
// puertos hacia otras features: sprints, esfuerzo, defectos, sesiones de estimación y proyectos.
type StoryService struct {
	stories  domain.StoryRepository
	sprints  domain.SprintAssignments
	effort   domain.EffortRecords
	defects  domain.DefectRecords
	sessions domain.EstimationSessions
	projects domain.ProjectDirectory
}

// NewStoryService recibe el repositorio de historias y los cinco puertos hacia otras features, de
// modo que su firma no cambie en las historias siguientes.
func NewStoryService(
	stories domain.StoryRepository,
	sprints domain.SprintAssignments,
	effort domain.EffortRecords,
	defects domain.DefectRecords,
	sessions domain.EstimationSessions,
	projects domain.ProjectDirectory,
) *StoryService {
	return &StoryService{
		stories:  stories,
		sprints:  sprints,
		effort:   effort,
		defects:  defects,
		sessions: sessions,
		projects: projects,
	}
}

// CreateStoryInput son los datos ya validados y normalizados por delivery para crear una historia.
// No lleva autor, proyecto ni estado: el autor es el actor y el estado inicial lo fija el servicio
// (RN-09, FR-049).
type CreateStoryInput struct {
	Title              string
	Description        *string
	Priority           domain.StoryPriority
	StoryPoints        *int
	AcceptanceCriteria []string
}

// StoryView es el detalle de una historia que el servicio entrega a delivery para armar StoryDetail
// del contrato: la historia con sus criterios y autor, si está lista para planificar y qué le falta,
// y si estuvo alguna vez en un sprint (FR-039).
type StoryView struct {
	Detail               *domain.StoryDetail
	Ready                bool
	Missing              []string
	EverAssignedToSprint bool
}

// Create registra una historia en el backlog del proyecto atribuida al actor y devuelve su detalle.
// Un proyecto Finalizado se rechaza con ErrBacklogReadOnly antes de escribir (FR-048, RN-15). La
// historia nace pending con un UUID nuevo y el actor como autor, sin consultar la propiedad del
// proyecto: cualquier integrante crea historias (RN-01, RN-02, RN-09, FR-049). La historia y sus
// criterios se persisten en una sola transacción del repositorio.
func (s *StoryService) Create(ctx context.Context, projectID, actorID uuid.UUID, input CreateStoryInput) (*StoryView, error) {
	finished, err := s.projects.IsFinished(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("consultar estado del proyecto: %w", err)
	}
	if finished {
		return nil, domain.ErrBacklogReadOnly
	}

	story := &domain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       input.Title,
		Description: input.Description,
		Priority:    input.Priority,
		Status:      domain.StoryStatusPending,
		StoryPoints: input.StoryPoints,
		CreatedByID: actorID,
	}
	if err := s.stories.Create(ctx, story, input.AcceptanceCriteria); err != nil {
		return nil, fmt.Errorf("crear historia: %w", err)
	}

	return s.buildDetail(ctx, story.ID)
}

// Get devuelve el detalle de una historia (FR-039). El acceso ya lo resolvió RequireStoryAccess
// antes de llegar acá, así que solo arma la vista con buildDetail; si la historia no existe, el
// error envuelve ErrStoryNotFound del repositorio.
func (s *StoryService) Get(ctx context.Context, storyID uuid.UUID) (*StoryView, error) {
	return s.buildDetail(ctx, storyID)
}

// buildDetail arma la vista de detalle de una historia: lee la historia con sus criterios y su
// autor, calcula con Readiness si está lista para planificar y qué le falta (RN-12, FR-028), y
// consulta el puerto de sprints para informar si estuvo alguna vez en un sprint (FR-039). Lo usan
// todas las operaciones que responden con el detalle.
func (s *StoryService) buildDetail(ctx context.Context, storyID uuid.UUID) (*StoryView, error) {
	detail, err := s.stories.FindDetail(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("leer detalle de historia: %w", err)
	}

	assignment, err := s.sprints.Find(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("consultar asignación a sprints: %w", err)
	}

	ready, missing := domain.Readiness(detail.Story.StoryPoints, len(detail.AcceptanceCriteria))
	return &StoryView{
		Detail:               detail,
		Ready:                ready,
		Missing:              missing,
		EverAssignedToSprint: assignment.EverAssigned,
	}, nil
}

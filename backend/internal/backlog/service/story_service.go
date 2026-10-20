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

// EmptyReason explica por qué el listado vino vacío, para que la pantalla distinga el backlog vacío
// de los filtros sin coincidencias (FR-035, FR-036). Viaja como texto en el campo emptyReason del
// contrato; su ausencia (nil) significa que hay resultados.
type EmptyReason string

// Motivos posibles de un listado vacío: el proyecto no tiene historias, aunque se haya filtrado, o
// las tiene pero ninguna cumple los filtros.
const (
	EmptyReasonBacklogEmpty EmptyReason = "backlog_empty"
	EmptyReasonNoMatches    EmptyReason = "no_matches"
)

// StoryListEntry es un elemento del listado que el servicio entrega a delivery: el tipo de lectura
// del repositorio más si la historia está lista para planificar, calculado con Readiness (FR-033).
type StoryListEntry struct {
	Item        domain.StoryListItem
	ReadyToPlan bool
}

// StoryListView es el resultado del listado: las historias en el orden de FR-032, nunca nil, y el
// motivo del vacío, nil cuando hay resultados.
type StoryListView struct {
	Stories     []StoryListEntry
	EmptyReason *EmptyReason
}

// List devuelve el backlog del proyecto filtrado y ordenado, con la condición de lista para
// planificar de cada historia (FR-031 a FR-036, RN-12). Solo cuando el resultado filtrado viene
// vacío consulta HasStories para elegir entre backlog_empty y no_matches: un proyecto sin historias
// es backlog_empty aunque haya filtros (research.md sección 9).
func (s *StoryService) List(ctx context.Context, projectID uuid.UUID, filter domain.StoryFilter) (*StoryListView, error) {
	items, err := s.stories.List(ctx, projectID, filter)
	if err != nil {
		return nil, fmt.Errorf("listar historias: %w", err)
	}

	// Cada elemento se completa con la regla única de Readiness a partir de sus Story Points y de su
	// conteo de criterios.
	entries := make([]StoryListEntry, 0, len(items))
	for _, item := range items {
		ready, _ := domain.Readiness(item.StoryPoints, item.CriteriaCount)
		entries = append(entries, StoryListEntry{Item: item, ReadyToPlan: ready})
	}
	if len(entries) > 0 {
		return &StoryListView{Stories: entries}, nil
	}

	// Con el resultado vacío, la consulta de existencia distingue el backlog vacío del filtro sin
	// coincidencias.
	hasStories, err := s.stories.HasStories(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("consultar existencia de historias: %w", err)
	}
	reason := EmptyReasonBacklogEmpty
	if hasStories {
		reason = EmptyReasonNoMatches
	}
	return &StoryListView{Stories: entries, EmptyReason: &reason}, nil
}

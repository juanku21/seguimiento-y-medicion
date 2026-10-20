// Package service contiene la lógica de negocio del backlog: crea, lista, consulta, estima,
// modifica y elimina historias de usuario apoyándose en las interfaces de backlog/domain
// (Principio III).
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// StoryService reúne los casos de uso del backlog. Firma mínima para las pruebas de T014; los
// campos y la implementación corresponden a T022.
type StoryService struct{}

// NewStoryService recibe el repositorio de historias y los cinco puertos hacia otras features, de
// modo que su firma no cambie en las historias siguientes. Firma mínima para las pruebas de T014.
func NewStoryService(
	stories domain.StoryRepository,
	sprints domain.SprintAssignments,
	effort domain.EffortRecords,
	defects domain.DefectRecords,
	sessions domain.EstimationSessions,
	projects domain.ProjectDirectory,
) *StoryService {
	return &StoryService{}
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

// Create registra una historia en el backlog del proyecto atribuida al actor. Firma mínima para las
// pruebas de T014; la implementación corresponde a T022.
func (s *StoryService) Create(ctx context.Context, projectID, actorID uuid.UUID, input CreateStoryInput) (*StoryView, error) {
	return nil, errors.New("not implemented")
}

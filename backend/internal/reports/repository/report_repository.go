// Package repository implementa la persistencia del módulo de reportes sobre GORM (Principio III).
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// ReportRepository lee de la base los datos con los que se arma el reporte.
type ReportRepository struct {
	db *gorm.DB
}

// NewReportRepository crea el repositorio de lecturas del reporte sobre la conexión dada.
func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// errNotImplemented es el error del esqueleto: las operaciones se implementan en tareas posteriores.
var errNotImplemented = errors.New("no implementado")

// LoadHeader lee el proyecto y el titular de la sesión para el encabezado.
func (r *ReportRepository) LoadHeader(ctx context.Context, projectID, userID uuid.UUID) (domain.ProjectInfo, domain.UserInfo, error) {
	return domain.ProjectInfo{}, domain.UserInfo{}, errNotImplemented
}

// LoadSprint lee un sprint del proyecto.
func (r *ReportRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (domain.SprintRef, error) {
	return domain.SprintRef{}, errNotImplemented
}

// ListSprints lista los sprints del proyecto.
func (r *ReportRepository) ListSprints(ctx context.Context, projectID uuid.UUID) ([]domain.SprintRef, error) {
	return nil, errNotImplemented
}

// LoadSprintStories lee las historias de un sprint.
func (r *ReportRepository) LoadSprintStories(ctx context.Context, sprint domain.SprintRef) ([]domain.StoryLine, error) {
	return nil, errNotImplemented
}

// LoadUnplannedStories lee las historias del proyecto sin sprint.
func (r *ReportRepository) LoadUnplannedStories(ctx context.Context, projectID uuid.UUID) ([]domain.StoryLine, error) {
	return nil, errNotImplemented
}

// LoadEstimationSessions lee las sesiones de Planning Poker de las historias dadas.
func (r *ReportRepository) LoadEstimationSessions(ctx context.Context, storyIDs []uuid.UUID) (map[uuid.UUID][]domain.SessionRecord, error) {
	return nil, errNotImplemented
}

// LoadEffort lee el esfuerzo agregado de los sprints dados.
func (r *ReportRepository) LoadEffort(ctx context.Context, sprintIDs []uuid.UUID) ([]domain.EffortRow, error) {
	return nil, errNotImplemented
}

// LoadDefects lee los defectos del proyecto, o los de un sprint si se indica.
func (r *ReportRepository) LoadDefects(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID) ([]domain.DefectLine, error) {
	return nil, errNotImplemented
}

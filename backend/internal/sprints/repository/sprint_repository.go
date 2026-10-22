package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// errNotImplemented es el error de las firmas mínimas que crearon las pruebas de integración de T013;
// cada operación se implementa en la tarea de su historia (T017 en adelante).
var errNotImplemented = errors.New("not implemented")

// SprintRepository implementa domain.SprintRepository con GORM sobre las tablas sprints y
// sprint_stories. Firma mínima de T013: las operaciones se implementan desde T017.
type SprintRepository struct {
	db *gorm.DB
}

// NewSprintRepository crea el repositorio de sprints sobre la conexión recibida.
func NewSprintRepository(db *gorm.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

// FindAccessible resuelve el acceso de un usuario a un sprint. Firma mínima.
func (r *SprintRepository) FindAccessible(ctx context.Context, sprintID, userID uuid.UUID) (*domain.SprintAccess, error) {
	return nil, errNotImplemented
}

// Create registra un sprint. Firma mínima: se implementa en T017.
func (r *SprintRepository) Create(ctx context.Context, sprint *domain.Sprint) error {
	return errNotImplemented
}

// Update modifica un sprint planificado. Firma mínima.
func (r *SprintRepository) Update(ctx context.Context, sprintID uuid.UUID, input domain.SprintInput) error {
	return errNotImplemented
}

// Delete elimina un sprint planificado. Firma mínima.
func (r *SprintRepository) Delete(ctx context.Context, sprintID uuid.UUID) error {
	return errNotImplemented
}

// Start inicia un sprint. Firma mínima.
func (r *SprintRepository) Start(ctx context.Context, sprintID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// Close cierra un sprint. Firma mínima.
func (r *SprintRepository) Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// AssignStories compromete historias en un sprint. Firma mínima.
func (r *SprintRepository) AssignStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// RemoveStories quita historias de un sprint. Firma mínima.
func (r *SprintRepository) RemoveStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID) error {
	return errNotImplemented
}

// ChangeStoryProgress cambia el estado de una historia del sprint activo. Firma mínima.
func (r *SprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error {
	return errNotImplemented
}

// List devuelve los sprints de un proyecto. Firma mínima.
func (r *SprintRepository) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) ([]domain.SprintListItem, error) {
	return nil, errNotImplemented
}

// HasSprints indica si el proyecto tiene sprints. Firma mínima.
func (r *SprintRepository) HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errNotImplemented
}

// FindDetail devuelve el detalle de un sprint. Firma mínima: se implementa en T017.
func (r *SprintRepository) FindDetail(ctx context.Context, sprintID uuid.UUID) (*domain.SprintDetail, error) {
	return nil, errNotImplemented
}

// ListForStory devuelve el historial de sprints de una historia. Firma mínima.
func (r *SprintRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	return nil, errNotImplemented
}

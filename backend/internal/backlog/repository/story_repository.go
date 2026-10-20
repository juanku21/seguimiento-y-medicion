package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// StoryRepository implementa domain.StoryRepository con GORM sobre user_stories y
// acceptance_criteria. Firma mínima para las pruebas de integración de T015; los campos y la
// implementación de Create y FindDetail corresponden a T021, y las demás operaciones a las
// historias que las usan.
type StoryRepository struct{}

// NewStoryRepository crea el repositorio de historias sobre la conexión recibida. Firma mínima para
// las pruebas de T015.
func NewStoryRepository(db *gorm.DB) *StoryRepository {
	return &StoryRepository{}
}

// FindAccessible resuelve el acceso a una historia para RequireStoryAccess. Firma mínima.
func (r *StoryRepository) FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*domain.StoryAccess, error) {
	return nil, errors.New("not implemented")
}

// Create persiste la historia y sus criterios en una transacción. Firma mínima.
func (r *StoryRepository) Create(ctx context.Context, story *domain.UserStory, criteria []string) error {
	return errors.New("not implemented")
}

// FindDetail lee la historia, sus criterios ordenados y su autor. Firma mínima.
func (r *StoryRepository) FindDetail(ctx context.Context, storyID uuid.UUID) (*domain.StoryDetail, error) {
	return nil, errors.New("not implemented")
}

// List devuelve el backlog filtrado de un proyecto. Firma mínima.
func (r *StoryRepository) List(ctx context.Context, projectID uuid.UUID, filter domain.StoryFilter) ([]domain.StoryListItem, error) {
	return nil, errors.New("not implemented")
}

// HasStories indica si el proyecto tiene alguna historia. Firma mínima.
func (r *StoryRepository) HasStories(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

// Update aplica los cambios presentes de un PATCH. Firma mínima.
func (r *StoryRepository) Update(ctx context.Context, storyID uuid.UUID, changes domain.StoryChanges) error {
	return errors.New("not implemented")
}

// UpdateStoryPoints guarda la estimación de la historia. Firma mínima.
func (r *StoryRepository) UpdateStoryPoints(ctx context.Context, storyID uuid.UUID, storyPoints *int) error {
	return errors.New("not implemented")
}

// Delete borra la historia y sus criterios en una transacción. Firma mínima.
func (r *StoryRepository) Delete(ctx context.Context, storyID uuid.UUID) error {
	return errors.New("not implemented")
}

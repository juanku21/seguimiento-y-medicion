package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectRepository implementa domain.ProjectRepository con GORM sobre la tabla projects. Firma
// mínima de T019: Create la implementa T024 y las demás operaciones las historias que las usan.
type ProjectRepository struct {
	db *gorm.DB
}

// NewProjectRepository crea el repositorio de proyectos sobre la conexión recibida.
func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// Create persiste el proyecto y la membresía del propietario en una sola transacción (FR-007,
// FR-010). Firma mínima de T019: la implementa T024.
func (r *ProjectRepository) Create(ctx context.Context, project *domain.Project, ownerMembership *domain.ProjectMember) error {
	return errors.New("not implemented")
}

// FindByID devuelve el proyecto por su identificador. Firma mínima de T019.
func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	return nil, errors.New("not implemented")
}

// Update modifica los datos y el estado del proyecto. Firma mínima de T019.
func (r *ProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	return errors.New("not implemented")
}

// ListForMember devuelve los proyectos de los que el usuario es integrante. Firma mínima de T019.
func (r *ProjectRepository) ListForMember(ctx context.Context, userID uuid.UUID) ([]domain.ProjectListItem, error) {
	return nil, errors.New("not implemented")
}

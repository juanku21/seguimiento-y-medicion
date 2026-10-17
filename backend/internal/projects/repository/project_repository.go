package repository

import (
	"context"
	"errors"
	"fmt"

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

// Create persiste el proyecto y la membresía del propietario en una sola transacción, de modo que
// si falla cualquiera de los dos INSERT no queda ni el proyecto ni la membresía (FR-007, FR-010,
// RN-01). No se consulta antes de insertar: la unicidad del nombre por propietario la garantiza el
// índice (owner_id, name_normalized), y su violación (gorm.ErrDuplicatedKey, gracias a
// TranslateError) se traduce a ErrProjectNameTaken, también ante creaciones simultáneas (FR-011,
// SC-007).
func (r *ProjectRepository) Create(ctx context.Context, project *domain.Project, ownerMembership *domain.ProjectMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(project).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrProjectNameTaken
			}
			return fmt.Errorf("insertar proyecto: %w", err)
		}
		if err := tx.Create(ownerMembership).Error; err != nil {
			return fmt.Errorf("insertar membresía del propietario: %w", err)
		}
		return nil
	})
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

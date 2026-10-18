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

// FindByID devuelve el proyecto por su identificador. Si no hay fila devuelve ErrProjectNotFound,
// nunca gorm.ErrRecordNotFound ni (nil, nil), para que el proyecto inexistente se responda igual
// que el proyecto ajeno (FR-049).
func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	var project domain.Project
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar el proyecto por identificador: %w", err)
	}
	return &project, nil
}

// Update modifica los datos y el estado del proyecto. Firma mínima de T019.
func (r *ProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	return errors.New("not implemented")
}

// ListForMember devuelve los proyectos de los que el usuario es integrante con una sola consulta:
// une project_members, filtrada por el usuario, con projects, y cuenta los integrantes de cada
// proyecto con una subconsulta escalar para evitar el N+1 (research.md sección 11). Como el filtro
// es la membresía del usuario, un proyecto ajeno no puede aparecer (FR-033). IsOwner compara
// projects.owner_id con el usuario y el orden es fecha de inicio descendente y, a igual fecha,
// nombre normalizado ascendente (FR-035). Sin proyectos devuelve una lista vacía, nunca nil.
func (r *ProjectRepository) ListForMember(ctx context.Context, userID uuid.UUID) ([]domain.ProjectListItem, error) {
	items := []domain.ProjectListItem{}
	err := r.db.WithContext(ctx).
		Table("project_members").
		Select(`projects.id, projects.name, projects.status, projects.start_date, projects.planned_end_date,
			(SELECT COUNT(*) FROM project_members AS pm WHERE pm.project_id = projects.id) AS member_count,
			projects.owner_id = ? AS is_owner`, userID).
		Joins("JOIN projects ON projects.id = project_members.project_id").
		Where("project_members.user_id = ?", userID).
		Order("projects.start_date DESC, projects.name_normalized ASC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("listar proyectos del integrante: %w", err)
	}
	return items, nil
}

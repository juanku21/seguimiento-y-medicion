package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectMemberRepository implementa domain.ProjectMemberRepository con GORM sobre la tabla
// project_members.
type ProjectMemberRepository struct {
	db *gorm.DB
}

// NewProjectMemberRepository crea el repositorio de integrantes sobre la conexión recibida.
func NewProjectMemberRepository(db *gorm.DB) *ProjectMemberRepository {
	return &ProjectMemberRepository{db: db}
}

// FindByProjectAndUser devuelve la membresía del usuario en el proyecto con una sola consulta sobre
// project_members. Como project_id es clave foránea hacia projects, que exista la fila prueba a la
// vez que el proyecto existe y que el usuario es integrante; si no hay fila, el proyecto inexistente
// y el ajeno se reportan igual con ErrProjectNotFound (FR-049, research.md sección 5).
func (r *ProjectMemberRepository) FindByProjectAndUser(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectMember, error) {
	var member domain.ProjectMember
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Take(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar la membresía del usuario en el proyecto: %w", err)
	}
	return &member, nil
}

// Create da de alta la membresía recibida. No se consulta antes de insertar: la unicidad la
// garantiza el índice único (project_id, user_id), y su violación (gorm.ErrDuplicatedKey, gracias
// a TranslateError) se traduce a ErrAlreadyMember, también ante altas simultáneas de la misma
// persona (FR-022, research.md sección 6).
func (r *ProjectMemberRepository) Create(ctx context.Context, member *domain.ProjectMember) error {
	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyMember
		}
		return fmt.Errorf("insertar membresía: %w", err)
	}
	return nil
}

// Delete da de baja la membresía del usuario en el proyecto con un borrado físico de la fila: el
// modelo no usa gorm.DeletedAt, porque una fila borrada lógicamente seguiría ocupando el índice
// único e impediría la nueva alta de la misma persona (FR-029, research.md secciones 6 y 8). Si el
// DELETE no afecta ninguna fila, la persona no era integrante y se devuelve ErrNotMember (FR-026).
func (r *ProjectMemberRepository) Delete(ctx context.Context, projectID, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&domain.ProjectMember{})
	if result.Error != nil {
		return fmt.Errorf("borrar membresía: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotMember
	}
	return nil
}

// ListMembers devuelve los integrantes del proyecto con su nombre y su correo, ordenados por fecha
// de incorporación. Resuelve los datos de cada persona con un solo JOIN a users que enumera las
// columnas y nunca selecciona password_hash (FR-030, research.md sección 8). IsOwner se deriva de
// comparar al integrante con projects.owner_id mediante una subconsulta escalar y JoinedAt es el
// created_at de la membresía (research.md sección 7). Un proyecto sin integrantes devuelve una
// lista vacía, nunca nil.
func (r *ProjectMemberRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]domain.MemberView, error) {
	members := []domain.MemberView{}
	err := r.db.WithContext(ctx).
		Table("project_members").
		Select(`users.id AS user_id, users.full_name, users.email,
			project_members.user_id = (SELECT projects.owner_id FROM projects WHERE projects.id = project_members.project_id) AS is_owner,
			project_members.created_at AS joined_at`).
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.project_id = ?", projectID).
		Order("project_members.created_at").
		Scan(&members).Error
	if err != nil {
		return nil, fmt.Errorf("listar integrantes del proyecto: %w", err)
	}
	return members, nil
}

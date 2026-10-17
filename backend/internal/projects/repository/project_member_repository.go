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
// project_members. Firma mínima de T087: ListMembers la implementa T088 y las demás operaciones las
// historias que las usan.
type ProjectMemberRepository struct {
	db *gorm.DB
}

// NewProjectMemberRepository crea el repositorio de integrantes sobre la conexión recibida.
func NewProjectMemberRepository(db *gorm.DB) *ProjectMemberRepository {
	return &ProjectMemberRepository{db: db}
}

// FindByProjectAndUser devuelve la membresía del usuario en el proyecto. Firma mínima de T087.
func (r *ProjectMemberRepository) FindByProjectAndUser(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectMember, error) {
	return nil, errors.New("not implemented")
}

// Create da de alta la membresía recibida. Firma mínima de T087.
func (r *ProjectMemberRepository) Create(ctx context.Context, member *domain.ProjectMember) error {
	return errors.New("not implemented")
}

// Delete da de baja la membresía del usuario en el proyecto. Firma mínima de T087.
func (r *ProjectMemberRepository) Delete(ctx context.Context, projectID, userID uuid.UUID) error {
	return errors.New("not implemented")
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

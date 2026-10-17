package repository

import (
	"context"
	"errors"

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

// ListMembers devuelve los integrantes del proyecto con su nombre y su correo. Firma mínima de T087.
func (r *ProjectMemberRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]domain.MemberView, error) {
	return nil, errors.New("not implemented")
}

package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// MemberService reúne el alta y la baja de integrantes sobre el repositorio de proyectos, el de
// integrantes y el directorio de usuarios del dominio (FR-019 a FR-029).
type MemberService struct {
	projects domain.ProjectRepository
	members  domain.ProjectMemberRepository
	users    domain.UserDirectory
}

// NewMemberService compone el servicio con los dos repositorios y el directorio de usuarios.
func NewMemberService(projects domain.ProjectRepository, members domain.ProjectMemberRepository, users domain.UserDirectory) *MemberService {
	return &MemberService{projects: projects, members: members, users: users}
}

// AddMember agrega como integrante del proyecto a la cuenta registrada con el correo indicado, a
// pedido de actorID, y devuelve la vista del nuevo integrante. Firma mínima de T043: la
// implementación corresponde a T050.
func (s *MemberService) AddMember(ctx context.Context, projectID, actorID uuid.UUID, email string) (*domain.MemberView, error) {
	return nil, errors.New("not implemented")
}

// RemoveMember quita del proyecto la membresía de userID, a pedido de actorID. Firma mínima de
// T043: la implementación corresponde a T050.
func (s *MemberService) RemoveMember(ctx context.Context, projectID, actorID, userID uuid.UUID) error {
	return errors.New("not implemented")
}

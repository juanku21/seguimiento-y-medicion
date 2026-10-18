package service

import (
	"context"
	"fmt"
	"strings"

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
// pedido de actorID, y devuelve la vista del nuevo integrante. El correo se normaliza con TrimSpace
// y ToLower antes de buscar la cuenta (RN-05). Un usuario que ya es integrante se detecta por el
// índice único al insertar, sin consulta previa, y llega como ErrAlreadyMember (RN-13).
func (s *MemberService) AddMember(ctx context.Context, projectID, actorID uuid.UUID, email string) (*domain.MemberView, error) {
	if _, err := s.findEditableProject(ctx, projectID, actorID); err != nil {
		return nil, err
	}

	user, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, fmt.Errorf("buscar cuenta por correo: %w", err)
	}

	// La membresía nueva nunca es la del propietario, que ya es integrante desde la creación
	// (RN-01); por eso IsOwner es false y JoinedAt es el created_at que completa la persistencia.
	member := &domain.ProjectMember{
		ID:        uuid.New(),
		ProjectID: projectID,
		UserID:    user.ID,
	}
	if err := s.members.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("crear membresía: %w", err)
	}

	return &domain.MemberView{
		UserID:   user.ID,
		FullName: user.FullName,
		Email:    user.Email,
		IsOwner:  false,
		JoinedAt: member.CreatedAt,
	}, nil
}

// RemoveMember quita del proyecto la membresía de userID, a pedido de actorID. El propietario no
// puede quitarse a sí mismo (RN-12) y una persona que no es integrante llega del repositorio como
// ErrNotMember (FR-026).
func (s *MemberService) RemoveMember(ctx context.Context, projectID, actorID, userID uuid.UUID) error {
	project, err := s.findEditableProject(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if userID == project.OwnerID {
		return domain.ErrOwnerCannotLeave
	}

	if err := s.members.Delete(ctx, projectID, userID); err != nil {
		return fmt.Errorf("eliminar membresía: %w", err)
	}
	return nil
}

// findEditableProject obtiene el proyecto y verifica, en este orden, que actorID sea el propietario
// (RN-02) y que el proyecto no esté finalizado (RN-11). La propiedad se resuelve comparando con
// projects.owner_id; cualquier rechazo ocurre antes de escribir.
func (s *MemberService) findEditableProject(ctx context.Context, projectID, actorID uuid.UUID) (*domain.Project, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("buscar proyecto: %w", err)
	}
	if project.OwnerID != actorID {
		return nil, domain.ErrNotProjectOwner
	}
	if project.Status == domain.ProjectStatusFinished {
		return nil, domain.ErrProjectReadOnly
	}
	return project, nil
}

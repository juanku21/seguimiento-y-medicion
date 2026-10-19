package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProjectListItem es el tipo de lectura de un elemento del listado de proyectos propios (FR-032).
// No es un modelo persistido: lo arma ListForMember en una sola consulta que une project_members
// con projects y cuenta los integrantes (research.md sección 11). MemberCount es siempre 1 o más,
// porque el propietario es integrante (RN-01), e IsOwner indica si quien consulta es el propietario.
type ProjectListItem struct {
	ID             uuid.UUID
	Name           string
	Status         ProjectStatus
	StartDate      time.Time
	PlannedEndDate time.Time
	MemberCount    int
	IsOwner        bool
}

// MemberView es el tipo de lectura de un integrante en la vista de estado (FR-030). No es un modelo
// persistido: lo arma ListMembers con un solo JOIN entre project_members y users que nunca
// selecciona password_hash (research.md sección 8). Email se carga siempre y es delivery quien lo
// omite cuando quien consulta no es el propietario. IsOwner se deriva de comparar al integrante con
// projects.owner_id y JoinedAt es el created_at de la membresía.
type MemberView struct {
	UserID   uuid.UUID
	FullName string
	Email    string
	IsOwner  bool
	JoinedAt time.Time
}

// ProjectRepository declara solo las operaciones sobre projects que usa esta feature (data-model.md
// sección 7; Principio III). Create recibe la membresía del propietario para persistir el proyecto
// y esa membresía en una sola transacción, sin exponer la transacción al servicio (FR-007, FR-010).
// FindByID resuelve la vista de estado y las acciones de administración, Update modifica los datos
// editables, UpdateStatus guarda solo el estado y la fecha de finalización real (FR-016, FR-039,
// FR-040), y ListForMember devuelve los proyectos de los que el usuario es integrante. No se
// declaran borrado de proyecto (RC-05), listado global ni búsquedas por otros campos. Cada
// operación devuelve el error de negocio y nunca gorm.ErrRecordNotFound ni (nil, nil): si el
// proyecto no existe, FindByID devuelve ErrProjectNotFound (FR-049); Create y Update traducen
// gorm.ErrDuplicatedKey del índice único (owner_id, name_normalized) a ErrProjectNameTaken, sin
// consultar antes de escribir (FR-004, research.md sección 2).
type ProjectRepository interface {
	Create(ctx context.Context, project *Project, ownerMembership *ProjectMember) error
	FindByID(ctx context.Context, id uuid.UUID) (*Project, error)
	Update(ctx context.Context, project *Project) error
	UpdateStatus(ctx context.Context, project *Project) error
	ListForMember(ctx context.Context, userID uuid.UUID) ([]ProjectListItem, error)
}

// ProjectMemberRepository declara solo las operaciones sobre project_members que usa esta feature.
// FindByProjectAndUser sostiene el middleware de membresía, Create y Delete resuelven el alta y la
// baja de integrantes, y ListMembers arma la lista de integrantes de la vista de estado. Delete
// recibe (projectID, userID) porque es lo que trae la ruta, sin una búsqueda previa del
// identificador de la membresía. Cada operación devuelve el error de negocio y nunca
// gorm.ErrRecordNotFound ni (nil, nil). FindByProjectAndUser la usa solo el middleware de
// membresía (research.md sección 5): en una sola consulta resuelve la existencia del proyecto y la
// membresía, y si no hay fila devuelve ErrProjectNotFound, para que el proyecto inexistente y el
// ajeno sean indistinguibles (FR-049). Create traduce gorm.ErrDuplicatedKey del índice único
// (project_id, user_id) a ErrAlreadyMember (FR-022, research.md sección 6) y Delete devuelve
// ErrNotMember si no hay fila que borrar (FR-026).
type ProjectMemberRepository interface {
	FindByProjectAndUser(ctx context.Context, projectID, userID uuid.UUID) (*ProjectMember, error)
	Create(ctx context.Context, member *ProjectMember) error
	Delete(ctx context.Context, projectID, userID uuid.UUID) error
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]MemberView, error)
}

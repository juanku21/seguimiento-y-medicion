package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// DefectAccess es la vista de lectura que resuelve el acceso a un defecto: su identificador, el
// proyecto al que pertenece (derivado de la historia) y si ese proyecto está finalizado.
type DefectAccess struct {
	DefectID        uuid.UUID
	ProjectID       uuid.UUID
	ProjectFinished bool
}

// UserRef es la referencia mínima a un usuario dentro de un defecto.
type UserRef struct {
	ID       uuid.UUID
	FullName string
}

// StoryRef es la referencia mínima a la historia de un defecto.
type StoryRef struct {
	ID    uuid.UUID
	Title string
}

// SprintRef es la referencia a un sprint dentro de un defecto.
type SprintRef struct {
	ID        uuid.UUID
	Name      string
	Status    sprintsdomain.SprintStatus
	StartDate time.Time
}

// DefectView es el defecto leído con sus relaciones ya resueltas; los campos de relación de Defect
// nunca se precargan. ResolvedSprint es nil mientras el defecto no esté resuelto.
type DefectView struct {
	Defect          Defect
	Story           StoryRef
	DetectedSprint  SprintRef
	ResolvedSprint  *SprintRef
	CreatedBy       UserRef
	UpdatedBy       UserRef
	StatusChangedBy UserRef
}

// DefectInput son los datos de alta de un defecto.
type DefectInput struct {
	StoryID          uuid.UUID
	DetectedSprintID uuid.UUID
	Description      string
	Severity         DefectSeverity
}

// DefectPatch es la modificación parcial de un defecto: un puntero nil indica que el campo no cambia.
type DefectPatch struct {
	StoryID          *uuid.UUID
	DetectedSprintID *uuid.UUID
	Description      *string
	Severity         *DefectSeverity
}

// StatusChange describe un cambio de estado: el estado destino y, al resolver, el sprint de resolución.
type StatusChange struct {
	To               DefectStatus
	ResolvedSprintID *uuid.UUID
}

// DefectFilter acota el listado de defectos de un proyecto; las listas vacías y los punteros nil no
// filtran.
type DefectFilter struct {
	Statuses   []DefectStatus
	Severities []DefectSeverity
	StoryID    *uuid.UUID
	SprintID   *uuid.UUID
}

// DefectRepository declara solo las operaciones de persistencia que usa el módulo. Ninguna devuelve
// gorm.ErrRecordNotFound ni (nil, nil): si el defecto no existe o es ajeno, devuelven ErrDefectNotFound.
type DefectRepository interface {
	// FindAccessible resuelve el acceso del usuario a un defecto.
	FindAccessible(ctx context.Context, defectID, userID uuid.UUID) (*DefectAccess, error)
	// Create da de alta un defecto en el proyecto, con el actor como autor y now como instante de estado.
	Create(ctx context.Context, projectID, actorID uuid.UUID, input DefectInput, now time.Time) (*DefectView, error)
	// ChangeStatus aplica un cambio de estado fijando status_changed_at en now.
	ChangeStatus(ctx context.Context, defectID, actorID uuid.UUID, change StatusChange, now time.Time) (*DefectView, error)
	// Update aplica una modificación parcial de campos.
	Update(ctx context.Context, defectID, actorID uuid.UUID, patch DefectPatch) (*DefectView, error)
	// FindView devuelve el detalle de un defecto con sus relaciones resueltas.
	FindView(ctx context.Context, defectID uuid.UUID) (*DefectView, error)
	// ListForProject lista los defectos de un proyecto según el filtro.
	ListForProject(ctx context.Context, projectID uuid.UUID, filter DefectFilter) ([]DefectView, error)
	// ValidateFilter verifica que los identificadores del filtro pertenezcan al proyecto.
	ValidateFilter(ctx context.Context, projectID uuid.UUID, filter DefectFilter) error
	// HasProjectDefects indica si el proyecto tiene al menos un defecto.
	HasProjectDefects(ctx context.Context, projectID uuid.UUID) (bool, error)
}

package delivery

import (
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectInput es el cuerpo del alta de un defecto. No declara ningún campo de autoría: un createdBy
// o createdById que llegue en el cuerpo se ignora (FR-056).
type DefectInput struct {
	// Texto libre de 1 a 2000 caracteres tras recortar los espacios extremos (FR-002).
	Description string `json:"description" validate:"required" minLength:"1" maxLength:"2000"`
	// Severidad: critical, high, medium o low (FR-003).
	Severity string `json:"severity" validate:"required" enums:"critical,high,medium,low"`
	// Historia del mismo proyecto, en cualquier estado (FR-004).
	StoryID string `json:"storyId" validate:"required" format:"uuid"`
	// Sprint de detección del mismo proyecto, activo o cerrado (FR-006).
	DetectedSprintID string `json:"detectedSprintId" validate:"required" format:"uuid"`
}

// UserRef es la referencia a una persona dentro de un defecto.
type UserRef struct {
	// Identificador de la persona.
	ID string `json:"id" validate:"required" format:"uuid"`
	// Nombre completo; se muestra aunque la persona ya no sea integrante (FR-055).
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// StoryRef es la referencia mínima a la historia de un defecto.
type StoryRef struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Reporte de ventas mensual"`
}

// SprintRef es la referencia a un sprint dentro de un defecto.
type SprintRef struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 2"`
	// Estado del sprint de la spec 004.
	Status string `json:"status" validate:"required" enums:"planned,active,closed" example:"active"`
	// Fecha de inicio prevista; ordena detección y resolución (RC-07).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-12"`
}

// Defect es el defecto con sus relaciones resueltas (contracts/openapi.yaml).
type Defect struct {
	// Identificador del defecto.
	ID string `json:"id" validate:"required" format:"uuid"`
	// Proyecto del defecto, derivado del proyecto de su historia.
	ProjectID string `json:"projectId" validate:"required" format:"uuid"`
	// Descripción del defecto.
	Description string `json:"description" validate:"required"`
	// Severidad: critical, high, medium o low.
	Severity string `json:"severity" validate:"required" enums:"critical,high,medium,low" example:"high"`
	// Estado: open, in_progress, resolved o discarded.
	Status string `json:"status" validate:"required" enums:"open,in_progress,resolved,discarded" example:"open"`
	// Historia a la que pertenece el defecto.
	Story StoryRef `json:"story" validate:"required"`
	// Sprint de detección.
	DetectedSprint SprintRef `json:"detectedSprint" validate:"required"`
	// Sprint de la última resolución; null si el defecto no está resolved (FR-031, RN-23).
	ResolvedSprint *SprintRef `json:"resolvedSprint" validate:"required"`
	// Quien registró el defecto.
	CreatedBy UserRef `json:"createdBy" validate:"required"`
	// Quien lo modificó por última vez.
	UpdatedBy UserRef `json:"updatedBy" validate:"required"`
	// Momento del último cambio de estado; en un defecto recién creado, el de su creación (FR-022).
	StatusChangedAt time.Time `json:"statusChangedAt" validate:"required"`
	// Quien hizo el último cambio de estado.
	StatusChangedBy UserRef `json:"statusChangedBy" validate:"required"`
	// Momento de creación.
	CreatedAt time.Time `json:"createdAt" validate:"required"`
	// Momento de la última actualización.
	UpdatedAt time.Time `json:"updatedAt" validate:"required"`
}

// DefectConflictResponse es el rechazo 409 por una regla de estado del defecto. AllowedStatuses se
// serializa siempre como arreglo y CurrentStatus como null cuando no aplica.
type DefectConflictResponse struct {
	// Código del conflicto.
	Code string `json:"code" validate:"required" enums:"read_only,no_detection_sprint,invalid_transition,defect_discarded,links_frozen"`
	// Mensaje en español dirigido a la persona.
	Error string `json:"error" validate:"required"`
	// Estado vigente del defecto; null con read_only y no_detection_sprint.
	CurrentStatus *string `json:"currentStatus" enums:"open,in_progress,resolved,discarded" extensions:"x-nullable"`
	// Destinos admitidos desde currentStatus; vacío si no aplica o si está discarded.
	AllowedStatuses []string `json:"allowedStatuses" validate:"required"`
}

// newUserRef convierte la referencia de dominio a su forma HTTP.
func newUserRef(ref domain.UserRef) UserRef {
	return UserRef{ID: ref.ID.String(), FullName: ref.FullName}
}

// newSprintRef convierte la referencia de sprint de dominio; la fecha va como AAAA-MM-DD.
func newSprintRef(ref domain.SprintRef) SprintRef {
	return SprintRef{
		ID:        ref.ID.String(),
		Name:      ref.Name,
		Status:    string(ref.Status),
		StartDate: ref.StartDate.Format(time.DateOnly),
	}
}

// newDefect convierte un DefectView en Defect. El proyecto llega aparte porque la vista no lo trae:
// es el de la historia, ya verificado por el servicio.
func newDefect(view *domain.DefectView, projectID uuid.UUID) Defect {
	// El sprint de resolución queda nil, y se serializa null, mientras el defecto no esté resuelto.
	var resolved *SprintRef
	if view.ResolvedSprint != nil {
		ref := newSprintRef(*view.ResolvedSprint)
		resolved = &ref
	}

	return Defect{
		ID:          view.Defect.ID.String(),
		ProjectID:   projectID.String(),
		Description: view.Defect.Description,
		Severity:    string(view.Defect.Severity),
		Status:      string(view.Defect.Status),
		Story:       StoryRef{ID: view.Story.ID.String(), Title: view.Story.Title},

		DetectedSprint:  newSprintRef(view.DetectedSprint),
		ResolvedSprint:  resolved,
		CreatedBy:       newUserRef(view.CreatedBy),
		UpdatedBy:       newUserRef(view.UpdatedBy),
		StatusChangedAt: view.Defect.StatusChangedAt,
		StatusChangedBy: newUserRef(view.StatusChangedBy),
		CreatedAt:       view.Defect.CreatedAt,
		UpdatedAt:       view.Defect.UpdatedAt,
	}
}

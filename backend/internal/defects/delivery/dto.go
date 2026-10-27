package delivery

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/service"
)

// newDefectList convierte los defectos del servicio en el arreglo HTTP, siempre no nil, y el motivo
// del vacío en su texto o nil.
func newDefectList(list *service.DefectListView, projectID uuid.UUID) ([]Defect, *string) {
	defects := make([]Defect, 0, len(list.Defects))
	for i := range list.Defects {
		defects = append(defects, newDefect(&list.Defects[i], projectID))
	}
	var reason *string
	if list.EmptyReason != nil {
		text := string(*list.EmptyReason)
		reason = &text
	}
	return defects, reason
}

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

// DefectStatusChange es el cuerpo del cambio de estado de un defecto (contracts/openapi.yaml).
type DefectStatusChange struct {
	// Estado destino: open, in_progress, resolved o discarded (FR-018).
	Status string `json:"status" validate:"required" enums:"open,in_progress,resolved,discarded"`
	// Sprint de resolución: obligatorio si status es resolved y prohibido en cualquier otro destino
	// (FR-023).
	ResolvedSprintID string `json:"resolvedSprintId,omitempty" format:"uuid"`
}

// DefectPatch es el cuerpo de la modificación parcial de un defecto (contracts/openapi.yaml,
// FR-033). Cada campo llega como json.RawMessage para distinguir el campo ausente, que queda nil, del
// null, que llega como su literal. Status, ResolvedSprintID, ProjectID, CreatedBy y CreatedByID no
// forman parte del contrato: se decodifican solo para rechazarlos (FR-035).
type DefectPatch struct {
	// Texto libre de 1 a 2000 caracteres tras recortar los espacios extremos (FR-034).
	Description json.RawMessage `json:"description" swaggertype:"string" minLength:"1" maxLength:"2000"`
	// Severidad: critical, high, medium o low (FR-034).
	Severity json.RawMessage `json:"severity" swaggertype:"string" enums:"critical,high,medium,low"`
	// Historia del mismo proyecto; solo se corrige con el defecto open (FR-038).
	StoryID json.RawMessage `json:"storyId" swaggertype:"string" format:"uuid"`
	// Sprint de detección del mismo proyecto; solo se corrige con el defecto open (FR-038).
	DetectedSprintID json.RawMessage `json:"detectedSprintId" swaggertype:"string" format:"uuid"`
	// El estado se cambia con las acciones de estado: si llega, se rechaza (FR-035).
	Status json.RawMessage `json:"status" swaggerignore:"true"`
	// El sprint de resolución se cambia con las acciones de estado: si llega, se rechaza (FR-035).
	ResolvedSprintID json.RawMessage `json:"resolvedSprintId" swaggerignore:"true"`
	// El proyecto no se modifica: si llega, se rechaza (FR-035).
	ProjectID json.RawMessage `json:"projectId" swaggerignore:"true"`
	// El autor no se modifica: si llega, se rechaza (FR-035).
	CreatedBy json.RawMessage `json:"createdBy" swaggerignore:"true"`
	// El autor no se modifica: si llega, se rechaza (FR-035).
	CreatedByID json.RawMessage `json:"createdById" swaggerignore:"true"`
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

// ProjectDefects es el listado de defectos de un proyecto. Defects se serializa siempre como arreglo,
// nunca como null, y EmptyReason es null cuando la lista tiene elementos (FR-045, FR-048).
type ProjectDefects struct {
	// Ordenados por severidad descendente y, a igual severidad, por creación descendente.
	Defects []Defect `json:"defects" validate:"required"`
	// Motivo del vacío: no_defects si el proyecto no tiene defectos, no_matches si los filtros no
	// encuentran ninguno; null si la lista tiene elementos.
	EmptyReason *string `json:"emptyReason" enums:"no_defects,no_matches" extensions:"x-nullable"`
}

// StoryDefects es el listado de defectos de una historia, con el mismo orden y datos que el del
// proyecto. Defects se serializa siempre como arreglo, nunca como null.
type StoryDefects struct {
	// Defectos de la historia, de cualquier estado.
	Defects []Defect `json:"defects" validate:"required"`
	// no_defects si la historia no tiene defectos; null si la lista tiene elementos.
	EmptyReason *string `json:"emptyReason" enums:"no_defects" extensions:"x-nullable"`
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

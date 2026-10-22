package delivery

import (
	"time"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Los errores reutilizan sin cambios ErrorResponse, FieldError y ValidationErrorResponse de
// auth/delivery (spec 001); acá se declaran solo los esquemas propios de los sprints.

// SprintInput es el cuerpo de la creación y de la modificación de un sprint
// (contracts/openapi.yaml, FR-001 a FR-004, FR-012). Las fechas llegan como texto para que la
// validación distinga una fecha ausente de una mal formada y responda por campo (FR-011). Las
// etiquetas de longitud solo documentan el contrato en Swagger: la validación real la hace
// ValidateSprintInput.
type SprintInput struct {
	// Entre 1 y 100 caracteres tras recortar los espacios extremos (FR-002).
	Name string `json:"name" validate:"required" minLength:"1" maxLength:"100" example:"Sprint 1"`
	// Sprint Goal; entre 1 y 500 caracteres tras recortar los espacios extremos (FR-003).
	Goal string `json:"goal" validate:"required" minLength:"1" maxLength:"500" example:"Cerrar el alta de clientes"`
	// Fecha de inicio prevista, sin hora (RC-07).
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-01"`
	// Fecha de fin prevista; estrictamente posterior a startDate (FR-004).
	EndDate string `json:"endDate" validate:"required" format:"date" example:"2026-10-14"`
}

// UserRef identifica a una persona en el detalle del sprint: su autor o quien lo cerró.
type UserRef struct {
	// Identificador de la cuenta.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo de la persona.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// SprintRef es la identificación mínima de un sprint, usada en los mensajes de conflicto.
type SprintRef struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"7c1d2e9a-5b8f-4b7e-9c0a-1f2e3d4c5b6a"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 1"`
}

// SprintConflictResponse es el cuerpo del 409 por conflicto con otro sprint del proyecto
// (superposición o sprint activo), que nombra el sprint en conflicto (FR-005).
type SprintConflictResponse struct {
	// Mensaje del rechazo, que nombra el sprint en conflicto.
	Error string `json:"error" validate:"required" example:"El período se superpone con el del sprint «Sprint 1»."`
	// Sprint que provoca el conflicto.
	Sprint SprintRef `json:"sprint" validate:"required"`
}

// newSprintConflictResponse arma el cuerpo del 409 a partir del mensaje y del sprint en conflicto
// que informa el error de dominio.
func newSprintConflictResponse(message string, sprint domain.ConflictingSprint) SprintConflictResponse {
	return SprintConflictResponse{
		Error:  message,
		Sprint: SprintRef{ID: sprint.ID.String(), Name: sprint.Name},
	}
}

// SprintResults son los totales congelados en la instantánea de un sprint cerrado (FR-043, FR-051).
// Se declara para tipar el campo results del detalle; lo completa la historia de cierre.
type SprintResults struct {
	// Story Points comprometidos al iniciar el sprint (FR-028).
	CommittedStoryPoints int `json:"committedStoryPoints" validate:"required" example:"20"`
	// Story Points de las historias del sprint al cerrarse.
	PlannedStoryPoints int `json:"plannedStoryPoints" validate:"required" example:"36"`
	// Story Points de las historias completadas.
	CompletedStoryPoints int `json:"completedStoryPoints" validate:"required" example:"13"`
	// Cantidad de historias del sprint al cerrarse.
	PlannedStoryCount int `json:"plannedStoryCount" validate:"required" example:"4"`
	// Cantidad de historias completadas.
	CompletedStoryCount int `json:"completedStoryCount" validate:"required" example:"2"`
}

// SprintSchedule son los días restantes o vencidos de un sprint Activo, contados en la zona horaria
// del sistema con el día de fin incluido; exactamente uno de los dos campos es distinto de null
// (FR-053, RN-23). Se declara para tipar el campo schedule del detalle; lo completa la historia de
// consulta del sprint.
type SprintSchedule struct {
	// Días que faltan hasta la fecha de fin; 0 el último día (US6-5).
	RemainingDays *int `json:"remainingDays" validate:"required" minimum:"0" extensions:"x-nullable" example:"10"`
	// Días transcurridos desde la fecha de fin, cuando el sprint está vencido (US6-6).
	OverdueDays *int `json:"overdueDays" validate:"required" minimum:"1" extensions:"x-nullable"`
}

// SprintStoryItem es una historia comprometida en el sprint. En un sprint abierto refleja los datos
// vigentes y en uno cerrado los congelados al cierre (US6-8).
type SprintStoryItem struct {
	// Identificador de la historia.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Exportar el reporte de velocidad a PDF"`
	// Prioridad de la historia: high (Alta), medium (Media) o low (Baja).
	Priority backlogdomain.StoryPriority `json:"priority" validate:"required" swaggertype:"string" enums:"high,medium,low" example:"high"`
	// Story Points vigentes, o los del cierre en un sprint cerrado.
	StoryPoints int `json:"storyPoints" validate:"required" example:"5"`
	// Estado vigente de la historia en el backlog: pending, in_progress o completed.
	Status backlogdomain.StoryStatus `json:"status" validate:"required" swaggertype:"string" enums:"pending,in_progress,completed" example:"in_progress"`
	// Fecha y hora del último completado dentro de este sprint (RFC 3339), o null.
	CompletedAt *string `json:"completedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-20T14:30:00Z"`
	// Resultado en un sprint cerrado: completed o not_completed; null mientras está abierto.
	Outcome *domain.StoryOutcome `json:"outcome" validate:"required" swaggertype:"string" enums:"completed,not_completed" extensions:"x-nullable" example:"completed"`
}

// SprintDetail es el detalle de un sprint y la respuesta de las escrituras (FR-053). Las fechas
// previstas van como YYYY-MM-DD y los momentos en UTC con formato RFC 3339. Los campos anulables se
// serializan como null cuando no tienen valor y Stories como lista, vacía si no hay historias.
type SprintDetail struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"7c1d2e9a-5b8f-4b7e-9c0a-1f2e3d4c5b6a"`
	// Identificador del proyecto al que pertenece el sprint.
	ProjectID string `json:"projectId" validate:"required" format:"uuid" example:"9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 1"`
	// Sprint Goal.
	Goal string `json:"goal" validate:"required" example:"Cerrar el alta de clientes"`
	// Estado del sprint: planned (Planificado), active (Activo) o closed (Cerrado) (RN-02).
	Status domain.SprintStatus `json:"status" validate:"required" enums:"planned,active,closed" example:"planned"`
	// Fecha de inicio prevista.
	StartDate string `json:"startDate" validate:"required" format:"date" example:"2026-10-01"`
	// Fecha de fin prevista.
	EndDate string `json:"endDate" validate:"required" format:"date" example:"2026-10-14"`
	// Momento real de inicio (RFC 3339); null si nunca se inició.
	StartedAt *string `json:"startedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-01T12:00:00Z"`
	// Momento real de cierre (RFC 3339); null si no está cerrado.
	ClosedAt *string `json:"closedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-14T18:00:00Z"`
	// Autor del sprint (FR-009).
	CreatedBy UserRef `json:"createdBy" validate:"required"`
	// Quien cerró el sprint (FR-040); null si no está cerrado.
	ClosedBy *UserRef `json:"closedBy" validate:"required" extensions:"x-nullable"`
	// Fecha y hora de creación (RFC 3339).
	CreatedAt string `json:"createdAt" validate:"required" format:"date-time" example:"2026-10-01T12:00:00Z"`
	// Fecha y hora de la última modificación (RFC 3339).
	UpdatedAt string `json:"updatedAt" validate:"required" format:"date-time" example:"2026-10-01T12:00:00Z"`
	// Historias comprometidas en el sprint; vacía si no tiene.
	Stories []SprintStoryItem `json:"stories" validate:"required"`
	// Totales de la instantánea; null si el sprint no está cerrado.
	Results *SprintResults `json:"results" validate:"required" extensions:"x-nullable"`
	// Factor de horas por Story Point congelado al cierre, con la representación del factor del
	// proyecto de la spec 002; null si el sprint no está cerrado.
	HoursPerStoryPoint *float64 `json:"hoursPerStoryPoint" validate:"required" extensions:"x-nullable" example:"6.5"`
	// Días restantes o vencidos; solo en un sprint active y null en los demás.
	Schedule *SprintSchedule `json:"schedule" validate:"required" extensions:"x-nullable"`
}

// newSprintDetail arma SprintDetail del contrato a partir de la vista que entrega el servicio. Es el
// único lugar que garantiza la forma de la respuesta: la lista de historias nunca queda nula, de
// modo que se serializa como [], y los momentos ausentes quedan en null. Results, ClosedBy,
// HoursPerStoryPoint y Schedule quedan en null: los completan las historias de cierre y de
// consulta del sprint.
func newSprintDetail(view *service.SprintView) SprintDetail {
	sprint := view.Detail.Sprint

	stories := make([]SprintStoryItem, 0, len(view.Detail.Stories))
	for _, story := range view.Detail.Stories {
		stories = append(stories, newSprintStoryItem(story))
	}

	return SprintDetail{
		ID:        sprint.ID.String(),
		ProjectID: sprint.ProjectID.String(),
		Name:      sprint.Name,
		Goal:      sprint.Goal,
		Status:    sprint.Status,
		StartDate: sprint.StartDate.Format(time.DateOnly),
		EndDate:   sprint.EndDate.Format(time.DateOnly),
		StartedAt: formatOptionalTime(sprint.StartedAt),
		ClosedAt:  formatOptionalTime(sprint.ClosedAt),
		CreatedBy: UserRef{
			ID:       view.Detail.CreatedBy.ID.String(),
			FullName: view.Detail.CreatedBy.FullName,
		},
		CreatedAt: sprint.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: sprint.UpdatedAt.UTC().Format(time.RFC3339),
		Stories:   stories,
	}
}

// newSprintStoryItem arma una historia del detalle. Los Story Points nunca faltan en una historia
// comprometida, porque solo se admiten historias estimadas y no se pueden volver a "sin estimar"
// mientras están en un sprint abierto (RN-18); el puntero se lee con valor por defecto para no
// desreferenciar nil.
func newSprintStoryItem(story domain.SprintStoryView) SprintStoryItem {
	var points int
	if story.StoryPoints != nil {
		points = *story.StoryPoints
	}

	return SprintStoryItem{
		StoryID:     story.StoryID.String(),
		Title:       story.Title,
		Priority:    story.Priority,
		StoryPoints: points,
		Status:      story.Status,
		CompletedAt: formatOptionalTime(story.CompletedAt),
		Outcome:     story.Outcome,
	}
}

// formatOptionalTime convierte un momento opcional a UTC con formato RFC 3339, o devuelve nil para
// que se serialice como null.
func formatOptionalTime(moment *time.Time) *string {
	if moment == nil {
		return nil
	}
	formatted := moment.UTC().Format(time.RFC3339)
	return &formatted
}

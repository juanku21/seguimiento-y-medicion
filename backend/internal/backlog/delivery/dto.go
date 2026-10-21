package delivery

import (
	"encoding/json"
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
)

// StoryInput es el cuerpo de la creación de una historia (contracts/openapi.yaml, FR-001 a FR-007).
// Los Story Points llegan como json.Number para validarlos sobre su texto literal; nil representa
// "sin estimar" (research.md sección 4). Las etiquetas de longitud y de valores solo documentan el
// contrato en Swagger: la validación real la hace ValidateStoryInput, que devuelve los mensajes por
// campo (FR-013).
type StoryInput struct {
	// Entre 1 y 150 caracteres tras recortar los espacios extremos (FR-002).
	Title string `json:"title" validate:"required" minLength:"1" maxLength:"150" example:"Exportar el reporte de velocidad a PDF"`
	// Opcional, hasta 2000 caracteres; vacía o solo espacios se guarda como ausente (FR-003).
	Description *string `json:"description" maxLength:"2000" extensions:"x-nullable" example:"El equipo necesita compartir la velocidad con la gerencia."`
	// Prioridad de la historia: high (Alta), medium (Media) o low (Baja).
	Priority domain.StoryPriority `json:"priority" validate:"required" enums:"high,medium,low" example:"high"`
	// Story Points de la escala, o null para "sin estimar"; el 0 es una estimación válida y distinta
	// de null (FR-014, FR-015, RN-06, RN-07).
	StoryPoints *json.Number `json:"storyPoints" swaggertype:"integer" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"5"`
	// Opcional. Lista ordenada; cada criterio tiene entre 1 y 500 caracteres tras recortar los
	// espacios extremos (FR-006, FR-007).
	AcceptanceCriteria []string `json:"acceptanceCriteria" example:"El PDF incluye la velocidad de los últimos cinco sprints."`
}

// StoryPointsInput es el cuerpo de la estimación de una historia (contracts/openapi.yaml, FR-016,
// FR-018). StoryPoints llega como json.RawMessage para distinguir el campo ausente, que se rechaza,
// del null, que significa "sin estimar" (research.md sección 8).
type StoryPointsInput struct {
	// Story Points de la escala, o null para "sin estimar"; el campo es obligatorio.
	StoryPoints json.RawMessage `json:"storyPoints" validate:"required" swaggertype:"integer" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"13"`
}

// StoryPatch es el cuerpo de la modificación parcial de una historia (contracts/openapi.yaml,
// FR-020). Cada campo llega como json.RawMessage para distinguir el campo ausente del null
// (research.md sección 6). Status y StoryPoints se decodifican solo para rechazarlos.
// Firma mínima de T056; la declaración completa con su documentación de Swagger es de T061.
type StoryPatch struct {
	Title              json.RawMessage `json:"title"`
	Description        json.RawMessage `json:"description"`
	Priority           json.RawMessage `json:"priority"`
	AcceptanceCriteria json.RawMessage `json:"acceptanceCriteria"`
	Status             json.RawMessage `json:"status"`
	StoryPoints        json.RawMessage `json:"storyPoints"`
}

// Readiness indica si la historia está lista para planificar y qué condiciones le faltan
// (FR-028, FR-029). Missing se serializa siempre como lista, vacía cuando Ready es verdadero.
type Readiness struct {
	// Verdadero cuando la historia tiene Story Points (incluido 0) y al menos un criterio.
	Ready bool `json:"ready" validate:"required" example:"false"`
	// Condiciones que faltan; vacía cuando ready es verdadero.
	Missing []string `json:"missing" validate:"required" enums:"story_points,acceptance_criteria"`
}

// AuthorRef es el autor de la historia; se conserva aunque haya dejado de ser integrante (FR-050).
type AuthorRef struct {
	// Identificador de la cuenta del autor.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo del autor.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// StoryDetail es el detalle completo de una historia (FR-039) y la respuesta de las escrituras.
// Description y StoryPoints se serializan como null cuando no tienen valor, y AcceptanceCriteria
// como lista, vacía si la historia no tiene criterios (FR-040). Las fechas van en UTC con formato
// RFC 3339.
type StoryDetail struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid" example:"7c4e2a19-3b8d-4f60-a1c5-9e2d0b6f8a73"`
	// Identificador del proyecto al que pertenece la historia.
	ProjectID string `json:"projectId" validate:"required" format:"uuid" example:"9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Exportar el reporte de velocidad a PDF"`
	// Descripción de la historia, o null si no tiene.
	Description *string `json:"description" validate:"required" extensions:"x-nullable" example:"El equipo necesita compartir la velocidad con la gerencia."`
	// Prioridad de la historia: high (Alta), medium (Media) o low (Baja).
	Priority domain.StoryPriority `json:"priority" validate:"required" enums:"high,medium,low" example:"high"`
	// Estado de la historia: pending (Pendiente), in_progress (En progreso) o completed (Completada)
	// (FR-026).
	Status domain.StoryStatus `json:"status" validate:"required" enums:"pending,in_progress,completed" example:"pending"`
	// Story Points de la escala, o null para "sin estimar"; el 0 es una estimación válida.
	StoryPoints *int `json:"storyPoints" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"5"`
	// Criterios en el orden en que se cargaron; vacía si no tiene (FR-040).
	AcceptanceCriteria []string `json:"acceptanceCriteria" validate:"required" example:"El PDF incluye la velocidad de los últimos cinco sprints."`
	// Condición de lista para planificar y qué le falta.
	Readiness Readiness `json:"readiness" validate:"required"`
	// Autor de la historia.
	CreatedBy AuthorRef `json:"createdBy" validate:"required"`
	// Fecha y hora de creación (RFC 3339).
	CreatedAt string `json:"createdAt" validate:"required" format:"date-time" example:"2026-10-20T14:30:00Z"`
	// Fecha y hora de la última modificación (RFC 3339).
	UpdatedAt string `json:"updatedAt" validate:"required" format:"date-time" example:"2026-10-20T14:30:00Z"`
	// Verdadero si la historia está o estuvo asignada a un sprint (FR-039).
	EverAssignedToSprint bool `json:"everAssignedToSprint" validate:"required" example:"false"`
}

// newStoryDetail arma StoryDetail del contrato a partir de la vista que entrega el servicio. Es el
// único lugar que garantiza la forma de la respuesta: los punteros nulos de descripción y Story
// Points se serializan como null, y las listas de criterios y de condiciones faltantes nunca quedan
// nulas, de modo que se serializan como [] (FR-040). Las fechas se convierten a UTC con formato
// RFC 3339.
func newStoryDetail(view *service.StoryView) StoryDetail {
	story := view.Detail.Story

	criteria := view.Detail.AcceptanceCriteria
	if criteria == nil {
		criteria = []string{}
	}
	missing := view.Missing
	if missing == nil {
		missing = []string{}
	}

	return StoryDetail{
		ID:                 story.ID.String(),
		ProjectID:          story.ProjectID.String(),
		Title:              story.Title,
		Description:        story.Description,
		Priority:           story.Priority,
		Status:             story.Status,
		StoryPoints:        story.StoryPoints,
		AcceptanceCriteria: criteria,
		Readiness: Readiness{
			Ready:   view.Ready,
			Missing: missing,
		},
		CreatedBy: AuthorRef{
			ID:       view.Detail.CreatedBy.ID.String(),
			FullName: view.Detail.CreatedBy.FullName,
		},
		CreatedAt:            story.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            story.UpdatedAt.UTC().Format(time.RFC3339),
		EverAssignedToSprint: view.EverAssignedToSprint,
	}
}

// StoryListItem es un elemento del listado del backlog (FR-033). StoryPoints se serializa como null
// cuando la historia está sin estimar; el 0 es una estimación válida.
type StoryListItem struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid" example:"1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Exportar el reporte de velocidad a PDF"`
	// Prioridad de la historia: high (Alta), medium (Media) o low (Baja).
	Priority domain.StoryPriority `json:"priority" validate:"required" enums:"high,medium,low" example:"high"`
	// Estado de la historia: pending (Pendiente), in_progress (En progreso) o completed (Completada).
	Status domain.StoryStatus `json:"status" validate:"required" enums:"pending,in_progress,completed" example:"pending"`
	// Story Points de la escala, o null para "sin estimar".
	StoryPoints *int `json:"storyPoints" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"5"`
	// Cantidad de criterios de aceptación de la historia.
	AcceptanceCriteriaCount int `json:"acceptanceCriteriaCount" validate:"required" minimum:"0" example:"2"`
	// Verdadero cuando tiene Story Points (incluido 0) y al menos un criterio (FR-028).
	ReadyToPlan bool `json:"readyToPlan" validate:"required" example:"true"`
}

// AppliedFilters son los filtros aplicados al listado; cada lista va vacía, nunca null, si no se
// filtró por ese criterio (FR-036).
type AppliedFilters struct {
	// Estados por los que se filtró.
	Status []domain.StoryStatus `json:"status" validate:"required" enums:"pending,in_progress,completed"`
	// Prioridades por las que se filtró.
	Priority []domain.StoryPriority `json:"priority" validate:"required" enums:"high,medium,low"`
}

// StoryListResponse es la respuesta del listado del backlog. Stories nunca es null y EmptyReason es
// null si hay historias, backlog_empty si el proyecto no tiene ninguna (FR-035) y no_matches si las
// tiene pero ninguna cumple los filtros (FR-036).
type StoryListResponse struct {
	// Historias en el orden de FR-032; nunca es null.
	Stories []StoryListItem `json:"stories" validate:"required"`
	// Filtros aplicados al listado.
	AppliedFilters AppliedFilters `json:"appliedFilters" validate:"required"`
	// Motivo del listado vacío, o null si hay historias.
	EmptyReason *service.EmptyReason `json:"emptyReason" validate:"required" swaggertype:"string" enums:"backlog_empty,no_matches" extensions:"x-nullable" example:"backlog_empty"`
}

// newStoryListResponse arma StoryListResponse a partir de la vista del servicio y del filtro aplicado.
// Las listas de historias y de filtros se inicializan vacías para que se serialicen como [] y nunca
// como null.
func newStoryListResponse(view *service.StoryListView, filter domain.StoryFilter) StoryListResponse {
	stories := make([]StoryListItem, 0, len(view.Stories))
	for _, entry := range view.Stories {
		stories = append(stories, StoryListItem{
			ID:                      entry.Item.ID.String(),
			Title:                   entry.Item.Title,
			Priority:                entry.Item.Priority,
			Status:                  entry.Item.Status,
			StoryPoints:             entry.Item.StoryPoints,
			AcceptanceCriteriaCount: entry.Item.CriteriaCount,
			ReadyToPlan:             entry.ReadyToPlan,
		})
	}

	statuses := append([]domain.StoryStatus{}, filter.Statuses...)
	priorities := append([]domain.StoryPriority{}, filter.Priorities...)

	return StoryListResponse{
		Stories:        stories,
		AppliedFilters: AppliedFilters{Status: statuses, Priority: priorities},
		EmptyReason:    view.EmptyReason,
	}
}

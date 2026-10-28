// Package delivery expone por HTTP el registro de esfuerzo: DTOs, validación de entrada, manejadores
// y rutas (Principio III).
package delivery

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Los errores reutilizan sin cambios ErrorResponse, FieldError y ValidationErrorResponse de
// auth/delivery (spec 001); acá se declaran solo los esquemas propios del registro de esfuerzo.

// EffortEntryInput es el cuerpo de POST /api/v1/stories/{storyId}/effort-entries
// (contracts/openapi.yaml). Las horas se reciben como json.RawMessage para leer su texto literal y
// rechazar un número entre comillas; no hay campo de autor (FR-002).
type EffortEntryInput struct {
	// Fecha de calendario; no futura ni anterior al inicio del proyecto (FR-006).
	EntryDate string `json:"entryDate" validate:"required" format:"date" example:"2026-10-24"`
	// Texto libre de 1 a 300 caracteres tras recortar los espacios extremos (FR-003).
	Activity string `json:"activity" validate:"required" minLength:"1" maxLength:"300" example:"Implementación del formulario de alta"`
	// Horas trabajadas, mayores que 0 y hasta 24, en múltiplos de 0,25 (FR-004).
	Hours json.RawMessage `json:"hours" validate:"required" swaggertype:"number" maximum:"24" multipleOf:"0.25" example:"6.5"`
}

// EffortEntryPatch es el cuerpo de PATCH /api/v1/effort-entries/{entryId} (contracts/openapi.yaml):
// modificación parcial que debe incluir al menos uno de los tres campos editables. Cada campo llega
// como json.RawMessage para distinguir el campo ausente, que queda nil, del null. StoryID, UserID y
// SprintID no forman parte del contrato: se decodifican solo para rechazarlos (RN-12, FR-019).
type EffortEntryPatch struct {
	// Fecha de calendario nueva; no futura ni anterior al inicio del proyecto (FR-006).
	EntryDate json.RawMessage `json:"entryDate" swaggertype:"string" format:"date" example:"2026-10-23"`
	// Actividad nueva, de 1 a 300 caracteres tras recortar los espacios extremos (FR-003).
	Activity json.RawMessage `json:"activity" swaggertype:"string" minLength:"1" maxLength:"300" example:"Implementación y pruebas del formulario de alta"`
	// Horas nuevas, mayores que 0 y hasta 24, en múltiplos de 0,25 (FR-004).
	Hours json.RawMessage `json:"hours" swaggertype:"number" maximum:"24" multipleOf:"0.25" example:"6"`
	// La historia de un registro no se modifica: si llega, se rechaza (RN-12).
	StoryID json.RawMessage `json:"storyId" swaggerignore:"true"`
	// El integrante de un registro no se modifica: si llega, se rechaza (RN-12).
	UserID json.RawMessage `json:"userId" swaggerignore:"true"`
	// El sprint de un registro no se modifica: si llega, se rechaza (RN-12).
	SprintID json.RawMessage `json:"sprintId" swaggerignore:"true"`
}

// Hundredths es el tipo de centésimas de la API, definido en httpserver y compartido con otros
// módulos; el alias evita cambiar los usos existentes.
type Hundredths = httpserver.Hundredths

// UserRef identifica al autor de un registro con su identificador y su nombre completo.
type UserRef struct {
	// Identificador de la cuenta.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo de la persona.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// StoryRef identifica la historia de un registro con su identificador y su título.
type StoryRef struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid" example:"1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
}

// SprintRef es el sprint asociado al registro al crearlo; nunca cambia (FR-015).
type SprintRef struct {
	// Identificador del sprint.
	ID string `json:"id" validate:"required" format:"uuid" example:"7c1d2e9a-5b8f-4b7e-9c0a-1f2e3d4c5b6a"`
	// Nombre del sprint.
	Name string `json:"name" validate:"required" example:"Sprint 3"`
	// Estado del sprint de la spec 004.
	Status sprintsdomain.SprintStatus `json:"status" validate:"required" swaggertype:"string" enums:"planned,active,closed" example:"active"`
}

// EffortEntry es un registro de esfuerzo tal como lo devuelve la API (contracts/openapi.yaml). Sprint
// es nil, y se serializa null, cuando el registro no tiene sprint asociado (FR-033); Definitive es un
// valor derivado en cada respuesta (FR-020).
type EffortEntry struct {
	// Identificador del registro.
	ID string `json:"id" validate:"required" format:"uuid" example:"5d6e7f80-1a2b-4c3d-8e9f-0a1b2c3d4e5f"`
	// Fecha de calendario del trabajo.
	EntryDate string `json:"entryDate" validate:"required" format:"date" example:"2026-10-24"`
	// Qué se hizo.
	Activity string `json:"activity" validate:"required" example:"Implementación del formulario de alta"`
	// Horas con exactamente 2 decimales (SC-015).
	Hours Hundredths `json:"hours" validate:"required" swaggertype:"number" example:"6.50"`
	// Persona que registró el esfuerzo; siempre el titular de la sesión que lo creó (FR-002).
	Author UserRef `json:"author" validate:"required"`
	// Historia sobre la que se registró el esfuerzo.
	Story StoryRef `json:"story" validate:"required"`
	// Sprint asociado al crear el registro; null si no tiene (FR-033).
	Sprint *SprintRef `json:"sprint" validate:"required" extensions:"x-nullable"`
	// true si el sprint asociado está closed y su plazo de gracia venció: el registro ya no se
	// modifica ni se elimina (FR-020).
	Definitive bool `json:"definitive" validate:"required" example:"false"`
	// Instante de creación.
	CreatedAt string `json:"createdAt" validate:"required" format:"date-time" example:"2026-10-24T15:04:05Z"`
	// Instante de la última modificación.
	UpdatedAt string `json:"updatedAt" validate:"required" format:"date-time" example:"2026-10-24T15:04:05Z"`
}

// newEffortEntry arma el registro de la respuesta a partir del tipo de lectura del dominio. La
// condición de definitivo depende del reloj y del plazo de gracia, que viven en el servicio, por eso
// se recibe ya calculada. La fecha se escribe sin hora y los instantes en RFC 3339 en UTC; sin sprint
// asociado, Sprint queda nil.
func newEffortEntry(view domain.EntryView, definitive bool) EffortEntry {
	entry := EffortEntry{
		ID:         view.Entry.ID.String(),
		EntryDate:  view.Entry.EntryDate.Format(time.DateOnly),
		Activity:   view.Entry.Activity,
		Hours:      Hundredths(view.Entry.HoursHundredths),
		Author:     UserRef{ID: view.Author.ID.String(), FullName: view.Author.FullName},
		Story:      StoryRef{ID: view.Story.ID.String(), Title: view.Story.Title},
		Definitive: definitive,
		CreatedAt:  view.Entry.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  view.Entry.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if view.Sprint != nil {
		entry.Sprint = &SprintRef{
			ID:     view.Sprint.ID.String(),
			Name:   view.Sprint.Name,
			Status: view.Sprint.Status,
		}
	}
	return entry
}

// EffortConflictResponse es el cuerpo del 409 del registro de esfuerzo: proyecto de solo lectura,
// sprint definitivo o tope diario superado. AvailableHours es nil, y se serializa null, salvo con
// daily_cap_exceeded.
type EffortConflictResponse struct {
	// Motivo del conflicto.
	Code string `json:"code" validate:"required" enums:"read_only,effort_finalized,daily_cap_exceeded" example:"daily_cap_exceeded"`
	// Mensaje en español dirigido a la persona.
	Error string `json:"error" validate:"required" example:"Superarías las 24 horas registradas para ese día. Te quedan 4 horas disponibles."`
	// Horas que quedan para la fecha, con 2 decimales; solo con daily_cap_exceeded, si no null.
	AvailableHours *Hundredths `json:"availableHours" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"4.00"`
}

// MemberEffort es la línea del desglose por integrante: la persona y la suma de sus horas sobre la
// historia, incluidas las de quienes ya no integran el proyecto (FR-032).
type MemberEffort struct {
	// Persona que registró esfuerzo sobre la historia.
	User UserRef `json:"user" validate:"required"`
	// Suma de sus horas con exactamente 2 decimales (SC-015).
	Hours Hundredths `json:"hours" validate:"required" swaggertype:"number" example:"10.00"`
}

// StoryEffort es el esfuerzo de una historia tal como lo devuelve la API (contracts/openapi.yaml):
// el total, el desglose por integrante y los registros. Sin registros, el total es 0.00 y las dos
// listas se serializan [] y nunca null (FR-034).
type StoryEffort struct {
	// Identificador de la historia consultada.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f"`
	// Total de horas de todos los registros, de cualquier autor, fecha y sprint, con exactamente 2
	// decimales (FR-031, SC-015).
	TotalHours Hundredths `json:"totalHours" validate:"required" swaggertype:"number" example:"16.00"`
	// Una línea por persona con registros, ordenadas por nombre, incluidas las ex integrantes (FR-032).
	ByMember []MemberEffort `json:"byMember" validate:"required"`
	// Registros por fecha descendente y, a igual fecha, por momento de creación (FR-035).
	Entries []EffortEntry `json:"entries" validate:"required"`
}

// newStoryEffort arma la respuesta del esfuerzo de una historia a partir del tipo de lectura del
// dominio. Las listas se crean con su capacidad y no nil, para que una historia sin registros
// responda [] en lugar de null; la condición de definitivo de cada registro ya viene calculada por el
// servicio.
func newStoryEffort(storyID uuid.UUID, effort domain.StoryEffort) StoryEffort {
	byMember := make([]MemberEffort, 0, len(effort.ByMember))
	for _, member := range effort.ByMember {
		byMember = append(byMember, MemberEffort{
			User:  UserRef{ID: member.UserID.String(), FullName: member.FullName},
			Hours: Hundredths(member.HoursHundredths),
		})
	}
	entries := make([]EffortEntry, 0, len(effort.Entries))
	for _, view := range effort.Entries {
		entries = append(entries, newEffortEntry(view, view.Definitive))
	}
	return StoryEffort{
		StoryID:    storyID.String(),
		TotalHours: Hundredths(effort.TotalHundredths),
		ByMember:   byMember,
		Entries:    entries,
	}
}

// ProjectEffort es la consulta del esfuerzo del proyecto tal como la devuelve la API
// (contracts/openapi.yaml): los registros que cumplen todos los filtros y su total. Entries se
// serializa [] y nunca null; EmptyReason es nil, y se serializa null, cuando la lista no está vacía.
type ProjectEffort struct {
	// Total de horas de los registros filtrados, con exactamente 2 decimales (FR-041).
	TotalHours Hundredths `json:"totalHours" validate:"required" swaggertype:"number" example:"14.25"`
	// Registros por fecha descendente y, a igual fecha, por momento de creación (FR-045).
	Entries []EffortEntry `json:"entries" validate:"required"`
	// no_effort si el proyecto no tiene ningún registro; no_matches si los tiene pero ninguno cumple
	// los filtros; null si la lista no está vacía (FR-042).
	EmptyReason *string `json:"emptyReason" validate:"required" enums:"no_effort,no_matches" extensions:"x-nullable" example:"no_matches"`
}

// newProjectEffort arma la respuesta de la consulta del esfuerzo del proyecto a partir del resultado
// del servicio, reutilizando el mapeo de cada registro. La lista se crea no nil para que una consulta
// vacía responda []; la condición de definitivo ya viene calculada por el servicio.
func newProjectEffort(view service.ProjectEffortView) ProjectEffort {
	entries := make([]EffortEntry, 0, len(view.Entries))
	for _, entry := range view.Entries {
		entries = append(entries, newEffortEntry(entry, entry.Definitive))
	}
	response := ProjectEffort{
		TotalHours: Hundredths(view.TotalHundredths),
		Entries:    entries,
	}
	if view.EmptyReason != nil {
		reason := string(*view.EmptyReason)
		response.EmptyReason = &reason
	}
	return response
}

// EffortComparisonRow es la comparación entre el esfuerzo estimado y el real de una historia tal como
// la devuelve la API (contracts/openapi.yaml). Los siete campos están siempre presentes: un valor "no
// calculable" se serializa null y nunca 0.00 (FR-046 a FR-051, FR-055).
type EffortComparisonRow struct {
	// Identificador de la historia.
	StoryID string `json:"storyId" validate:"required" format:"uuid" example:"1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Alta de clientes"`
	// Story Points de la historia; null significa "sin estimar" y 0 es una estimación válida.
	StoryPoints *int `json:"storyPoints" validate:"required" extensions:"x-nullable" example:"5"`
	// Story Points × factor vigente del proyecto, con 2 decimales; null si la historia no está estimada.
	EstimatedHours *Hundredths `json:"estimatedHours" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"30.00"`
	// Suma de las horas registradas sobre la historia, con 2 decimales; 0.00 sin registros.
	ActualHours Hundredths `json:"actualHours" validate:"required" swaggertype:"number" example:"36.00"`
	// Real − estimada, con 2 decimales y signo; null si la historia no está estimada.
	DeviationHours *Hundredths `json:"deviationHours" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"6.00"`
	// (Real − estimada) / estimada × 100, con 2 decimales y signo, redondeado mitad lejos de cero
	// solo al final; null si la estimada es nula o 0 (FR-049, RN-26).
	DeviationPercent *Hundredths `json:"deviationPercent" validate:"required" swaggertype:"number" extensions:"x-nullable" example:"20.00"`
}

// ProjectEffortComparison es la comparación de todas las historias del proyecto tal como la devuelve
// la API (contracts/openapi.yaml). Stories se serializa [] y nunca null.
type ProjectEffortComparison struct {
	// Factor de horas por Story Point vigente del proyecto, con 2 decimales (FR-054).
	HoursPerStoryPoint Hundredths `json:"hoursPerStoryPoint" validate:"required" swaggertype:"number" example:"6.00"`
	// Todas las historias del proyecto, de cualquier estado, en el orden del backlog de la 003 (FR-052).
	Stories []EffortComparisonRow `json:"stories" validate:"required"`
}

// newEffortComparisonRow arma la fila de la comparación a partir del resultado del servicio. Cada
// valor en centésimas que el dominio deja nil queda nil, y se serializa null.
func newEffortComparisonRow(row service.StoryComparisonRow) EffortComparisonRow {
	comparison := row.Comparison
	return EffortComparisonRow{
		StoryID:          row.StoryID.String(),
		Title:            row.Title,
		StoryPoints:      comparison.StoryPoints,
		EstimatedHours:   optionalHundredths(comparison.EstimatedHundredths),
		ActualHours:      Hundredths(comparison.ActualHundredths),
		DeviationHours:   optionalHundredths(comparison.DeviationHundredths),
		DeviationPercent: optionalHundredths(comparison.DeviationPercentHundredths),
	}
}

// newProjectEffortComparison arma la comparación del proyecto a partir del resultado del servicio. La
// lista se crea no nil para que un proyecto sin historias responda [].
func newProjectEffortComparison(view service.ProjectComparisonView) ProjectEffortComparison {
	stories := make([]EffortComparisonRow, 0, len(view.Stories))
	for _, row := range view.Stories {
		stories = append(stories, newEffortComparisonRow(row))
	}
	return ProjectEffortComparison{
		HoursPerStoryPoint: Hundredths(view.FactorHundredths),
		Stories:            stories,
	}
}

// optionalHundredths convierte un valor opcional en centésimas del dominio al tipo de la API,
// conservando nil como "no calculable".
func optionalHundredths(value *int) *Hundredths {
	if value == nil {
		return nil
	}
	hundredths := Hundredths(*value)
	return &hundredths
}

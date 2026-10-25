// Package delivery expone por HTTP el registro de esfuerzo: DTOs, validación de entrada, manejadores
// y rutas (Principio III).
package delivery

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
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

// Hundredths es una magnitud en centésimas (de hora o de punto porcentual) que la API serializa como
// número JSON con exactamente 2 decimales (research.md sección 5; SC-015 y SC-022). Un *Hundredths
// nulo representa "no calculable" y se serializa null.
type Hundredths int

// MarshalJSON escribe el entero de centésimas como número con 2 decimales y signo. El signo se separa
// de la magnitud para no perderlo cuando la parte entera es cero (-5 se escribe -0.05); la magnitud se
// toma como uint para que el valor mínimo de int no desborde al cambiarle el signo.
func (h Hundredths) MarshalJSON() ([]byte, error) {
	sign := ""
	magnitude := uint(h)
	if h < 0 {
		sign = "-"
		magnitude = uint(-int(h))
	}
	return fmt.Appendf(nil, "%s%d.%02d", sign, magnitude/100, magnitude%100), nil
}

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

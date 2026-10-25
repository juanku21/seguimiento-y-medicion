// Package delivery expone por HTTP el registro de esfuerzo: DTOs, validación de entrada, manejadores
// y rutas (Principio III).
package delivery

import "encoding/json"

// EffortEntryInput es el cuerpo de POST /api/v1/stories/{storyId}/effort-entries
// (contracts/openapi.yaml). Las horas se reciben como json.RawMessage para leer su texto literal y
// rechazar un número entre comillas; no hay campo de autor (FR-002).
type EffortEntryInput struct {
	EntryDate string          `json:"entryDate"`
	Activity  string          `json:"activity"`
	Hours     json.RawMessage `json:"hours"`
}

// Package delivery expone por HTTP el registro de esfuerzo: DTOs, validación de entrada, manejadores
// y rutas (Principio III).
package delivery

import (
	"encoding/json"
	"errors"
)

// EffortEntryInput es el cuerpo de POST /api/v1/stories/{storyId}/effort-entries
// (contracts/openapi.yaml). Las horas se reciben como json.RawMessage para leer su texto literal y
// rechazar un número entre comillas; no hay campo de autor (FR-002).
type EffortEntryInput struct {
	EntryDate string          `json:"entryDate"`
	Activity  string          `json:"activity"`
	Hours     json.RawMessage `json:"hours"`
}

// Hundredths es una magnitud en centésimas (de hora o de punto porcentual) que la API serializa como
// número JSON con exactamente 2 decimales (research.md sección 5; SC-015 y SC-022). Un *Hundredths
// nulo representa "no calculable" y se serializa null.
type Hundredths int

// MarshalJSON escribe el entero de centésimas como número con 2 decimales y signo.
func (h Hundredths) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not implemented")
}

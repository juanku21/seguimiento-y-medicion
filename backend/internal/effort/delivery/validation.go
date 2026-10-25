package delivery

import (
	"encoding/json"
	"errors"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// ParseHours valida las horas sobre su texto literal y las devuelve en centésimas de hora (RN-03).
// Firma mínima de T009: la implementación corresponde a T025.
func ParseHours(raw json.RawMessage) (int, error) {
	return 0, errors.New("not implemented")
}

// ValidateEntryInput valida el cuerpo del alta y devuelve los datos normalizados junto con un
// FieldError por cada campo inválido. Firma mínima de T009: la implementación corresponde a T025.
func ValidateEntryInput(req EffortEntryInput) (domain.EntryInput, []authdelivery.FieldError) {
	return domain.EntryInput{}, nil
}

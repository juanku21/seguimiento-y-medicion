package delivery

import (
	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// ValidateSprintInput aplica las reglas RN-03 y RN-04 al cuerpo de la creación y la modificación de
// un sprint y devuelve los datos normalizados junto con un FieldError por cada campo inválido,
// todos a la vez (FR-011). Firma mínima de T011: la implementación corresponde a T016.
func ValidateSprintInput(req SprintInput) (domain.SprintInput, []authdelivery.FieldError) {
	return domain.SprintInput{}, nil
}

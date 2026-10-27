package delivery

import (
	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// ValidateDefectInput valida el cuerpo del alta y devuelve los datos normalizados junto con un
// FieldError por cada campo inválido, todos a la vez (Principio VII).
func ValidateDefectInput(req DefectInput) (domain.DefectInput, []authdelivery.FieldError) {
	return domain.DefectInput{}, nil
}

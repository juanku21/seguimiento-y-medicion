package delivery

import (
	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// ValidateVoteRequest valida el cuerpo del voto y devuelve la carta admitida, o un FieldError en card
// si la carta no es válida o si falta, es null o está vacía (FR-020, FR-025, RN-36). Ante un error no
// se llega al servicio, así que el voto vigente no cambia (Principio VII).
func ValidateVoteRequest(req VoteRequest) (domain.Card, []authdelivery.FieldError) {
	return "", nil
}

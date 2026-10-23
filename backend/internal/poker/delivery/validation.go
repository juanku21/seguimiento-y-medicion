package delivery

import (
	"encoding/json"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Mensajes de validación del voto dirigidos a la persona, declarados en un único lugar
// (contracts/openapi.yaml, ejemplos cartaInvalida y retiro; FR-025, RN-36).
const (
	msgInvalidCard  = "La carta no es válida. Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?."
	msgWithdrawVote = `Un voto emitido no se retira: solo se cambia por otra carta, incluida "?".`
)

// ValidateVoteRequest valida el cuerpo del voto y devuelve la carta admitida, o un FieldError en card
// si la carta no es válida o si falta, es null o está vacía (FR-020, FR-025, RN-36). Ante un error no
// se llega al servicio, así que el voto vigente no cambia (Principio VII).
// La carta ausente llega como mensaje vacío y el null como su literal; cualquier otro valor debe ser
// un texto JSON, de modo que un número o un booleano es una carta no válida. El texto se valida con
// domain.ParseCard y su error se traduce al mensaje de delivery.
func ValidateVoteRequest(req VoteRequest) (domain.Card, []authdelivery.FieldError) {
	invalid := []authdelivery.FieldError{{Field: "card", Message: msgInvalidCard}}
	withdraw := []authdelivery.FieldError{{Field: "card", Message: msgWithdrawVote}}

	if raw := string(req.Card); raw == "" || raw == "null" {
		return "", withdraw
	}
	var text string
	if err := json.Unmarshal(req.Card, &text); err != nil {
		return "", invalid
	}
	if text == "" {
		return "", withdraw
	}
	card, err := domain.ParseCard(text)
	if err != nil {
		return "", invalid
	}
	return card, nil
}

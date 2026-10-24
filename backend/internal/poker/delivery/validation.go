package delivery

import (
	"encoding/json"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Mensajes de validación del voto y del registro dirigidos a la persona, declarados en un único lugar
// (contracts/openapi.yaml, ejemplos cartaInvalida y retiro y respuesta 400 de finalize; FR-025,
// FR-050, RN-36). Las enumeraciones se toman de domain.AdmittedCardsList y domain.AgreedScaleList
// para que la escala se declare una sola vez.
var (
	msgInvalidCard        = "La carta no es válida. Cartas admitidas: " + domain.AdmittedCardsList() + "."
	msgWithdrawVote       = `Un voto emitido no se retira: solo se cambia por otra carta, incluida "?".`
	msgInvalidAgreedValue = "El valor acordado debe ser uno de " + domain.AgreedScaleList() + "."
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

// ValidateFinalizeRequest valida el cuerpo del registro y devuelve el valor acordado, o un FieldError
// en storyPoints si no es un entero de la escala numérica (FR-049, FR-050, RN-18).
// El valor ausente o null se rechaza antes de decodificar, porque null decodificado en un int queda en
// 0, que es un valor de la escala. Cualquier otro valor debe ser un número JSON entero, de modo que
// "?", un texto o un decimal se rechazan; el entero se valida con domain.ValidateAgreedValue y su
// error se traduce al mensaje de delivery.
func ValidateFinalizeRequest(req FinalizeRequest) (int, []authdelivery.FieldError) {
	invalid := []authdelivery.FieldError{{Field: "storyPoints", Message: msgInvalidAgreedValue}}

	if raw := string(req.StoryPoints); raw == "" || raw == "null" {
		return 0, invalid
	}
	var value int
	if err := json.Unmarshal(req.StoryPoints, &value); err != nil {
		return 0, invalid
	}
	if err := domain.ValidateAgreedValue(value); err != nil {
		return 0, invalid
	}
	return value, nil
}

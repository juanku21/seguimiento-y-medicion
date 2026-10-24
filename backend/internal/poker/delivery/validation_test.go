package delivery

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Pruebas unitarias obligatorias de la validación del cuerpo del voto (T034, RN-06, RN-36). Los
// nombres llevan el prefijo "Validacion" para no chocar con las pruebas de integración de US2 en
// handler_test.go (T036), que cubren los mismos escenarios de punta a punta. Los ayudantes llevan el
// prefijo "validation" por la misma razón.

// validationInvalidCardMessage es el mensaje del rechazo de una carta no admitida
// (contracts/openapi.yaml, ejemplo cartaInvalida).
const validationInvalidCardMessage = "La carta no es válida. Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?."

// validationWithdrawVoteMessage es el mensaje del rechazo de un intento de retirar el voto
// (contracts/openapi.yaml, ejemplo retiro; RN-36).
const validationWithdrawVoteMessage = `Un voto emitido no se retira: solo se cambia por otra carta, incluida "?".`

// validationDecodeVote decodifica el cuerpo JSON recibido en VoteRequest, tal como lo hace el
// manejador antes de validar, para probar la validación con los cuerpos literales del contrato.
func validationDecodeVote(t *testing.T, body string) VoteRequest {
	t.Helper()
	var req VoteRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req), "el cuerpo %s debía decodificarse", body)
	return req
}

// validationRequireCardRejected exige que la validación rechace exactamente el campo card con el
// mensaje indicado y que no devuelva ninguna carta.
func validationRequireCardRejected(t *testing.T, body, message string) {
	t.Helper()
	card, fields := ValidateVoteRequest(validationDecodeVote(t, body))

	require.Equal(t, []authdelivery.FieldError{{Field: "card", Message: message}}, fields,
		"el cuerpo %s debía rechazarse solo en card", body)
	assert.Equal(t, domain.Card(""), card)
}

// Escenario: Votar en secreto - Caso normal: voto oculto (005/US2)
// Un cuerpo con la carta como texto admitido pasa la validación y devuelve esa carta sin errores; la
// carta "?" también, porque es un voto emitido igual que cualquier otro.
func TestUS2_ValidacionVotoValido(t *testing.T) {
	accepted := []struct {
		name string
		body string
		want domain.Card
	}{
		{"carta 5", `{"card":"5"}`, domain.Card("5")},
		{"carta ?", `{"card":"?"}`, domain.CardUnknown},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			card, fields := ValidateVoteRequest(validationDecodeVote(t, tc.body))

			assert.Empty(t, fields)
			assert.Equal(t, tc.want, card)
		})
	}
}

// Escenario: Votar en secreto - Caso de error: valor fuera de la escala (005/US2)
// Una carta fuera de la escala, con formato distinto del admitido o enviada como número JSON en
// lugar de texto se rechaza en card con la lista de cartas admitidas (FR-020, FR-025, RN-06). Cada
// cuerpo rechazado es un subtest.
func TestUS2_ValidacionValorFueraDeEscala(t *testing.T) {
	rejected := []struct {
		name string
		body string
	}{
		{"numero JSON 5", `{"card":5}`},
		{"numero JSON decimal 1.5", `{"card":1.5}`},
		{"intermedio 4", `{"card":"4"}`},
		{"intermedio 50", `{"card":"50"}`},
		{"decimal 1.5", `{"card":"1.5"}`},
		{"negativo -1", `{"card":"-1"}`},
		{"fuera de rango 101", `{"card":"101"}`},
		{"espacio inicial", `{"card":" 5"}`},
		{"signo repetido", `{"card":"??"}`},
		{"booleano", `{"card":true}`},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			validationRequireCardRejected(t, tc.body, validationInvalidCardMessage)
		})
	}
}

// Escenario: Votar en secreto - Caso de error: retiro de voto (005/US2)
// Una carta ausente, null o vacía es un intento de volver a quedar sin voto y se rechaza en card
// indicando que solo se cambia por otra carta, incluida "?" (RN-36). Cada cuerpo es un subtest.
func TestUS2_ValidacionRetiroDeVoto(t *testing.T) {
	withdrawals := []struct {
		name string
		body string
	}{
		{"carta ausente", `{}`},
		{"carta null", `{"card":null}`},
		{"carta vacia", `{"card":""}`},
	}
	for _, tc := range withdrawals {
		t.Run(tc.name, func(t *testing.T) {
			validationRequireCardRejected(t, tc.body, validationWithdrawVoteMessage)
		})
	}
}

// Pruebas unitarias obligatorias de la validación del cuerpo de registro de la estimación acordada
// (T062, FR-049, FR-050, RN-18). Siguen el mismo prefijo "Validacion" para no chocar con las pruebas
// de integración de US5 en handler_test.go (T063).

// validationInvalidAgreedValueMessage es el mensaje del rechazo de un valor acordado fuera de la
// escala numérica (contracts/openapi.yaml, respuesta 400 de finalize).
const validationInvalidAgreedValueMessage = "El valor acordado debe ser uno de 0, 1, 2, 3, 5, 8, 13, 20, 40 o 100."

// validationDecodeFinalize decodifica el cuerpo JSON recibido en FinalizeRequest, tal como lo hace el
// manejador antes de validar, para probar la validación con los cuerpos literales del contrato.
func validationDecodeFinalize(t *testing.T, body string) FinalizeRequest {
	t.Helper()
	var req FinalizeRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req), "el cuerpo %s debía decodificarse", body)
	return req
}

// Escenario: Registrar la estimación acordada - Caso normal: registro con consenso (005/US5)
// Un cuerpo con un entero de la escala pasa la validación y devuelve ese valor sin errores.
func TestUS5_ValidacionRegistroValido(t *testing.T) {
	value, fields := ValidateFinalizeRequest(validationDecodeFinalize(t, `{"storyPoints":8}`))

	assert.Empty(t, fields)
	assert.Equal(t, 8, value)
}

// Escenario: Registrar la estimación acordada - Caso de error: valor inválido (005/US5)
// La carta "?", el valor nulo, el valor ausente (cuerpo vacío), un decimal y un entero fuera de la
// escala se rechazan en storyPoints con el mensaje que enumera la escala numérica, sin devolver un
// valor (FR-050). Cada cuerpo rechazado es un subtest.
func TestUS5_ValidacionValorInvalido(t *testing.T) {
	rejected := []struct {
		name string
		body string
	}{
		{"carta ?", `{"storyPoints":"?"}`},
		{"valor null", `{"storyPoints":null}`},
		{"cuerpo vacio", `{}`},
		{"decimal 7.5", `{"storyPoints":7.5}`},
		{"fuera de escala 7", `{"storyPoints":7}`},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			value, fields := ValidateFinalizeRequest(validationDecodeFinalize(t, tc.body))

			require.Equal(t,
				[]authdelivery.FieldError{{Field: "storyPoints", Message: validationInvalidAgreedValueMessage}},
				fields, "el cuerpo %s debía rechazarse solo en storyPoints", tc.body)
			assert.Equal(t, 0, value)
		})
	}
}

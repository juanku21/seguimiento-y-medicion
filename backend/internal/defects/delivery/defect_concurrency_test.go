package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Prueba de integración de dos transiciones simultáneas sobre el mismo defecto (T029; FR-018, RN-24,
// research.md sección 9): dos integrantes envían a la vez PUT /api/v1/defects/{defectId}/status con
// in_progress sobre un defecto Abierto, contra la base smye_test. Exactamente una transición se
// aplica y la otra se valida contra el estado ya cambiado y se rechaza con 409 invalid_transition.
// Cada repetición usa un defecto nuevo para que el intercalado entre las transacciones ocurra de
// verdad. No lleva la etiqueta de un escenario de la spec porque verifica una regla de negocio
// (RN-24) y no un escenario Given/When/Then.

// defectConcurrentRounds es la cantidad de repeticiones que fija tasks.md (T029).
const defectConcurrentRounds = 10

// defectRequestsAtOnce ejecuta una solicitud por token desde goroutines que esperan la misma señal de
// salida, para maximizar la simultaneidad, y devuelve las respuestas en el orden de los tokens. Cada
// goroutine escribe solo en su posición del slice, así que no comparten estado mutable.
func defectRequestsAtOnce(tokens []string, send func(token string) *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	recs := make([]*httptest.ResponseRecorder, len(tokens))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() {
			<-start
			recs[i] = send(token)
		})
	}
	close(start)
	wg.Wait()
	return recs
}

// Ana y Beto pasan a la vez a En progreso el mismo defecto Abierto: en cada repetición uno recibe 200
// y el otro 409 invalid_transition con currentStatus in_progress y las transiciones permitidas desde
// ahí; ninguno recibe 500, y la fila guarda como autor del cambio a quien ganó (FR-018, RN-24).
func TestDefectTransition_ConcurrentStart(t *testing.T) {
	f := newDefectFixture(t)
	memberID, memberToken := f.env.signUp(t, "Beto Ruiz", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")
	tokens := []string{f.ownerToken, memberToken}
	actors := []uuid.UUID{f.ownerID, memberID}

	// Cada repetición registra un defecto Abierto nuevo y lanza las dos transiciones a la vez.
	for round := range defectConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			defectID := f.registerDefect(t, f.storyID, f.sprintID)

			recs := defectRequestsAtOnce(tokens, func(token string) *httptest.ResponseRecorder {
				return f.env.putDefectStatus(t, token, defectID, "in_progress", uuid.Nil)
			})

			// Una respuesta es la transición aplicada y la otra el rechazo contra el estado vigente.
			winner := -1
			rejected := 0
			for i, rec := range recs {
				switch rec.Code {
				case http.StatusOK:
					winner = i
					var defect defectResponse
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &defect), "cuerpo del 200: %s", rec.Body.String())
					assert.Equal(t, "in_progress", defect.Status, "la transición aceptada deja el defecto En progreso")
				case http.StatusConflict:
					rejected++
					body := requireDefectConflict(t, rec, "invalid_transition")
					assert.Equal(t, `"in_progress"`, string(body.CurrentStatus), "currentStatus es el estado ya cambiado")
					assert.Contains(t, body.AllowedStatuses, "resolved", "desde En progreso se permite Resuelto")
					assert.NotContains(t, body.AllowedStatuses, "open")
					assert.NotContains(t, body.AllowedStatuses, "in_progress")
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
			require.NotEqual(t, -1, winner, "exactamente una de las dos transiciones debe aceptarse")
			assert.Equal(t, 1, rejected, "exactamente una de las dos transiciones debe rechazarse")

			// La fila quedó En progreso y registra como autor del cambio a quien recibió el 200.
			stored := f.env.storedDefect(t, defectID)
			assert.Equal(t, "in_progress", string(stored.Status))
			assert.Equal(t, actors[winner], stored.StatusChangedByID, "status_changed_by es quien ganó")
		})
	}
}

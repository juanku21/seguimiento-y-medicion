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

// Un integrante corrige la historia de un defecto Abierto mientras otro lo pasa a En progreso (T049;
// FR-018, FR-038): en cada repetición el resultado es uno de dos. O la corrección se aplicó y después
// el paso (200 y 200, con el defecto En progreso vinculado a la historia nueva), o el paso se aplicó
// primero y la corrección recibió 409 links_frozen (el defecto queda En progreso con la historia
// original). Ninguna solicitud recibe 500. No lleva etiqueta de escenario porque verifica una regla
// de concurrencia y no un escenario Given/When/Then.
func TestDefectLinks_ConcurrentCorrectionAndStart(t *testing.T) {
	f := newDefectFixture(t)
	otherStory := f.env.createStory(t, f.ownerToken, f.projectID, "Exportación a PDF")
	_, memberToken := f.env.signUp(t, "Beto Ruiz", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")

	// Cada repetición registra un defecto Abierto nuevo y lanza la corrección y el paso a la vez.
	for round := range defectConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			defectID := f.registerDefect(t, f.storyID, f.sprintID)

			// El índice 0 corrige la historia y el 1 pasa el defecto a En progreso.
			recs := defectRequestsAtOnce([]string{f.ownerToken, memberToken}, func(token string) *httptest.ResponseRecorder {
				if token == f.ownerToken {
					return f.env.patchDefect(t, token, defectID, map[string]any{"storyId": otherStory.String()})
				}
				return f.env.putDefectStatus(t, token, defectID, "in_progress", uuid.Nil)
			})
			correction, start := recs[0], recs[1]

			// El paso a En progreso siempre se acepta: desde Abierto es una transición válida.
			require.Equal(t, http.StatusOK, start.Code, "el paso a En progreso; cuerpo: %s", start.Body.String())

			stored := f.env.storedDefect(t, defectID)
			assert.Equal(t, "in_progress", string(stored.Status))
			switch correction.Code {
			case http.StatusOK:
				// La corrección se aplicó antes del paso: la historia nueva quedó vinculada.
				assert.Equal(t, otherStory, stored.StoryID, "corrección aplicada: el defecto queda en la historia nueva")
			case http.StatusConflict:
				// El paso ganó: la corrección se rechazó y la historia original no cambió.
				body := requireDefectConflict(t, correction, "links_frozen")
				assert.Equal(t, `"in_progress"`, string(body.CurrentStatus))
				assert.Equal(t, f.storyID, stored.StoryID, "corrección rechazada: la historia original no cambia")
			default:
				t.Errorf("corrección: código %d inesperado; cuerpo: %s", correction.Code, correction.Body.String())
			}
		})
	}
}

// Dos integrantes actúan a la vez sobre el mismo defecto En progreso, uno lo resuelve y el otro lo
// descarta (T064; FR-018, RN-24, SC-019): en cada repetición exactamente uno recibe 200 y el otro 409
// invalid_transition con el estado vigente, y el estado final es el de quien ganó, de modo que nunca
// queda Resuelto después de Descartado. No lleva etiqueta de escenario porque verifica una regla de
// concurrencia y no un escenario Given/When/Then.
func TestDefectTransition_Concurrency(t *testing.T) {
	f := newDefectFixture(t)
	_, memberToken := f.env.signUp(t, "Beto Ruiz", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")

	// Cada repetición registra un defecto En progreso nuevo y lanza la resolución y el descarte a la vez.
	for round := range defectConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			defectID := f.registerDefect(t, f.storyID, f.sprintID)
			f.startDefect(t, f.ownerToken, defectID)

			// El índice 0 resuelve (Ana) y el 1 descarta (Beto).
			recs := defectRequestsAtOnce([]string{f.ownerToken, memberToken}, func(token string) *httptest.ResponseRecorder {
				if token == f.ownerToken {
					return f.env.putDefectStatus(t, token, defectID, "resolved", f.sprintID)
				}
				return f.env.putDefectStatus(t, token, defectID, "discarded", uuid.Nil)
			})

			// Una respuesta es la transición aplicada y la otra el rechazo contra el estado ya cambiado.
			targets := []string{"resolved", "discarded"}
			winner := -1
			rejected := 0
			for i, rec := range recs {
				switch rec.Code {
				case http.StatusOK:
					winner = i
					var defect defectResponse
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &defect), "cuerpo del 200: %s", rec.Body.String())
					assert.Equal(t, targets[i], defect.Status, "la transición aceptada deja el estado pedido")
				case http.StatusConflict:
					rejected++
					requireDefectConflict(t, rec, "invalid_transition")
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
			require.NotEqual(t, -1, winner, "exactamente una de las dos transiciones debe aceptarse")
			require.Equal(t, 1, rejected, "exactamente una de las dos transiciones debe rechazarse")

			// El rechazo informa el estado vigente, que es el que dejó quien ganó.
			loser := 1 - winner
			var conflict defectConflictResponse
			require.NoError(t, json.Unmarshal(recs[loser].Body.Bytes(), &conflict))
			assert.Equal(t, fmt.Sprintf("%q", targets[winner]), string(conflict.CurrentStatus), "currentStatus es el estado ya cambiado")

			// La fila quedó en el estado del ganador: nunca Resuelto después de Descartado.
			stored := f.env.storedDefect(t, defectID)
			assert.Equal(t, targets[winner], string(stored.Status))
			if targets[winner] == "discarded" {
				assert.Nil(t, stored.ResolvedSprintID, "un defecto Descartado no lleva sprint de resolución")
			}
		})
	}
}

// Un integrante modifica la descripción de un defecto En progreso mientras otro lo descarta (T064;
// FR-018, última oración): el descarte siempre se acepta; la modificación se aplica antes (200) o, si
// el descarte se aplicó primero, recibe 409 defect_discarded y la descripción no cambia. Ninguna
// solicitud recibe 500. No lleva etiqueta de escenario porque verifica una regla de concurrencia.
func TestDefectTransition_ConcurrentUpdateAndDiscard(t *testing.T) {
	f := newDefectFixture(t)
	_, memberToken := f.env.signUp(t, "Beto Ruiz", "beto@example.com")
	f.env.addMember(t, f.ownerToken, f.projectID, "beto@example.com")
	const newDescription = "Descripción corregida en simultáneo"

	// Cada repetición registra un defecto En progreso nuevo y lanza la modificación y el descarte a la vez.
	for round := range defectConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			defectID := f.registerDefect(t, f.storyID, f.sprintID)
			f.startDefect(t, f.ownerToken, defectID)
			before := f.env.storedDefect(t, defectID)

			// El índice 0 modifica la descripción (Ana) y el 1 descarta (Beto).
			recs := defectRequestsAtOnce([]string{f.ownerToken, memberToken}, func(token string) *httptest.ResponseRecorder {
				if token == f.ownerToken {
					return f.env.patchDefect(t, token, defectID, map[string]any{"description": newDescription})
				}
				return f.env.putDefectStatus(t, token, defectID, "discarded", uuid.Nil)
			})
			update, discard := recs[0], recs[1]

			// El descarte siempre se acepta: desde En progreso es una transición válida.
			require.Equal(t, http.StatusOK, discard.Code, "el descarte; cuerpo: %s", discard.Body.String())

			stored := f.env.storedDefect(t, defectID)
			assert.Equal(t, "discarded", string(stored.Status), "el defecto termina Descartado")
			switch update.Code {
			case http.StatusOK:
				// La modificación se aplicó antes del descarte.
				assert.Equal(t, newDescription, stored.Description)
			case http.StatusConflict:
				// El descarte ganó: la modificación se rechazó y la descripción original no cambió.
				body := requireDefectConflict(t, update, "defect_discarded")
				assert.Equal(t, `"discarded"`, string(body.CurrentStatus))
				assert.Equal(t, before.Description, stored.Description, "modificación rechazada: la descripción no cambia")
			default:
				t.Errorf("modificación: código %d inesperado; cuerpo: %s", update.Code, update.Body.String())
			}
		})
	}
}

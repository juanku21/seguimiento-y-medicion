package delivery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Prueba de integración de la apertura simultánea de sesiones (T016; SC-012, RN-04): diez cuentas
// integrantes del proyecto envían a la vez POST /api/v1/stories/{storyId}/estimation-sessions sobre
// la misma historia contra la base smye_test, y al terminar se verifica que la historia tiene una
// sola sesión open. Cada repetición usa una historia nueva para que el intercalado entre las
// transacciones ocurra de verdad. No lleva la etiqueta de un escenario de la spec porque verifica un
// criterio de éxito y no un escenario Given/When/Then.

// Cantidad de repeticiones y de cuentas integrantes que abren la sesión a la vez en cada una.
const (
	pokerConcurrentRounds   = 5
	pokerConcurrentAccounts = 10
)

// pokerConcurrentResult es lo que obtuvo una apertura simultánea: la respuesta leída o el error de
// transporte, que se informa desde la goroutine principal.
type pokerConcurrentResult struct {
	resp pokerResponse
	err  error
}

// pokerConcurrentOpen envía la apertura sin usar require, porque corre en una goroutine distinta de
// la de la prueba y ahí no se puede llamar a FailNow; el error vuelve como valor.
func (e *pokerTestEnv) pokerConcurrentOpen(token string, storyID uuid.UUID) pokerConcurrentResult {
	req, err := http.NewRequest(http.MethodPost, e.server.URL+openSessionPath(storyID), http.NoBody)
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: pokerHTTPTimeout}).Do(req)
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	return pokerConcurrentResult{resp: pokerResponse{
		Code:        resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        data,
	}}
}

// pokerOpenAtOnce abre la sesión sobre la historia con todos los tokens desde goroutines que
// esperan la misma señal de salida, para maximizar la simultaneidad, y devuelve los resultados en el
// orden de los tokens. Cada goroutine escribe solo en su posición del slice.
func (e *pokerTestEnv) pokerOpenAtOnce(tokens []string, storyID uuid.UUID) []pokerConcurrentResult {
	results := make([]pokerConcurrentResult, len(tokens))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() {
			<-start
			results[i] = e.pokerConcurrentOpen(token, storyID)
		})
	}
	close(start)
	wg.Wait()
	return results
}

// Diez aperturas simultáneas sobre la misma historia desde cuentas integrantes distintas: en cada
// repetición exactamente una responde 201, las nueve restantes 409 con el sessionId de la ganadora,
// ninguna 500, y en la base la historia tiene una única sesión open, la de la ganadora (SC-012).
func TestPokerOpen_Concurrency(t *testing.T) {
	env := newPokerTestEnv(t)

	// Ana es propietaria de un proyecto En curso con diez cuentas integrantes.
	_, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProject(t, anaToken, "Portal de Clientes")
	tokens := make([]string, 0, pokerConcurrentAccounts)
	for i := range pokerConcurrentAccounts {
		email := fmt.Sprintf("integrante%d@example.com", i+1)
		_, token := env.signUp(t, fmt.Sprintf("Integrante %d", i+1), email)
		env.addMember(t, anaToken, projectID, email)
		tokens = append(tokens, token)
	}
	env.setProjectStatus(t, anaToken, projectID, "in_progress")

	// Cada repetición usa una historia Pendiente recién creada y sin sesiones.
	for round := range pokerConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			storyID := env.createStory(t, anaToken, projectID, pokerStoryInput{
				Title: fmt.Sprintf("Historia concurrente %d", round+1),
			})

			results := env.pokerOpenAtOnce(tokens, storyID)

			// Cada respuesta es una apertura o un rechazo por sesión en curso, y se acepta una sola.
			var winners []pokerSessionBody
			var conflicts []pokerResponse
			for i, res := range results {
				if !assert.NoError(t, res.err, "apertura %d sin respuesta", i+1) {
					continue
				}
				switch res.resp.Code {
				case http.StatusCreated:
					var session pokerSessionBody
					if assert.NoError(t, json.Unmarshal(res.resp.Body, &session), "el 201 no es JSON válido: %s", res.resp.Body) {
						winners = append(winners, session)
					}
				case http.StatusConflict:
					conflicts = append(conflicts, res.resp)
				default:
					t.Errorf("apertura %d: código %d inesperado; cuerpo: %s", i+1, res.resp.Code, res.resp.Body)
				}
			}
			require.Len(t, winners, 1, "debería aceptarse exactamente una apertura")
			assert.Len(t, conflicts, pokerConcurrentAccounts-1, "las demás aperturas deberían responder 409")

			// Cada 409 trae el mensaje de sesión en curso y el sessionId de la ganadora.
			for _, resp := range conflicts {
				assert.JSONEq(t, `{"error":"`+pokerOpenSessionExistsMessage+`","sessionId":"`+winners[0].ID+`"}`,
					string(resp.Body))
			}

			// En la base la historia tiene una sola sesión, open, y es la de la ganadora.
			var open []domain.EstimationSession
			require.NoError(t, env.db.Where("story_id = ? AND status = ?", storyID, domain.SessionStatusOpen).
				Find(&open).Error, "no se pudieron leer las sesiones abiertas de la historia")
			require.Len(t, open, 1, "la historia debería tener una sola sesión open")
			assert.Equal(t, winners[0].ID, open[0].ID.String(), "la sesión open debería ser la de la ganadora")
			assert.Equal(t, int64(1), pokerSessionRows(t, env.db, storyID), "no debería haber otras sesiones de la historia")
		})
	}
}

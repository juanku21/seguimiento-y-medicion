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

// Prueba de integración del cambio de voto simultáneo con la revelación manual (T048; SC-007,
// FR-031, RN-11): repite veinte veces, cada vez sobre una sesión nueva, el caso de
// TestUS2_CambioSimultaneoConRevelacion y compara las vistas de los tres canales entre sí y con la
// base. No lleva la etiqueta de un escenario de la spec porque verifica un criterio de éxito.

// Cantidad de revelaciones simultáneas con un cambio de voto que exige SC-007.
const pokerVoteRevealRounds = 20

// pokerVoteAndRevealAtOnce envía el cambio de carta del participante y la revelación del facilitador
// desde dos goroutines que esperan la misma señal de salida, y devuelve ambos resultados.
func (e *pokerTestEnv) pokerVoteAndRevealAtOnce(voterToken, facilitatorToken, sessionID string, payload []byte) (change, reveal pokerConcurrentResult) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		change = e.pokerSend(http.MethodPut, votePath(sessionID), voterToken, payload)
	})
	wg.Go(func() {
		<-start
		reveal = e.pokerSend(http.MethodPost, revealPath(sessionID), facilitatorToken, nil)
	})
	close(start)
	wg.Wait()
	return change, reveal
}

// En cada repetición, Bruno ya votó 5, Carla 8 y Ana, facilitadora, no votó; Bruno cambia a 13 en el
// mismo instante en que Ana revela. Ninguna respuesta es 500: la revelación responde 200 y el cambio
// 200 o 409. Los canales de Ana, Bruno y Carla reciben la ronda revelada con la misma versión, que es
// la guardada en la base, y el mismo conjunto de votos, que coincide con la carta guardada de Bruno
// (SC-007).
func TestPokerVoteReveal_Concurrency(t *testing.T) {
	f := newPokerUS1Fixture(t)
	payload, err := json.Marshal(map[string]any{"card": "13"})
	require.NoError(t, err)

	for round := range pokerVoteRevealRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			// Sesión nueva sobre una historia nueva, con los tres canales conectados y dos votos.
			storyID := f.env.createStory(t, f.anaToken, f.projectID, pokerStoryInput{
				Title: fmt.Sprintf("Historia con revelación concurrente %d", round+1),
			})
			session := f.env.requireOpenedSession(t, f.anaToken, storyID)
			streams := map[string]*pokerStream{
				"Ana":   f.env.openStream(t, f.anaToken, session.ID),
				"Bruno": f.env.openStream(t, f.brunoToken, session.ID),
				"Carla": f.env.openStream(t, f.carlaToken, session.ID),
			}
			three := allConnected(f.anaID, f.brunoID, f.carlaID)
			for name, stream := range streams {
				stream.NextUntil(t, "los tres conectados en el canal de "+name, three)
			}
			f.env.requireVoted(t, f.brunoToken, session.ID, "5")
			f.env.requireVoted(t, f.carlaToken, session.ID, "8")

			change, reveal := f.env.pokerVoteAndRevealAtOnce(f.brunoToken, f.anaToken, session.ID, payload)

			// Ninguna respuesta es 500: la revelación se acepta y el cambio se acepta o se rechaza
			// porque la ronda ya estaba revelada.
			require.NoError(t, reveal.err, "la revelación no obtuvo respuesta")
			require.NoError(t, change.err, "el cambio de voto no obtuvo respuesta")
			assert.NotEqual(t, http.StatusInternalServerError, reveal.resp.Code, "revelación; cuerpo: %s", reveal.resp.Body)
			assert.NotEqual(t, http.StatusInternalServerError, change.resp.Code, "cambio de voto; cuerpo: %s", change.resp.Body)
			require.Equal(t, http.StatusOK, reveal.resp.Code, "revelación; cuerpo: %s", reveal.resp.Body)
			brunoCard := "5"
			switch change.resp.Code {
			case http.StatusOK:
				brunoCard = "13"
			case http.StatusConflict:
				assert.JSONEq(t, pokerRoundRevealedBodyJSON, string(change.resp.Body))
			default:
				t.Fatalf("cambio de voto: código %d inesperado; cuerpo: %s", change.resp.Code, change.resp.Body)
			}

			// Los tres canales muestran la misma revelación: misma versión y mismos votos.
			votes := map[uuid.UUID]string{f.brunoID: brunoCard, f.carlaID: "8"}
			versions := make(map[string]int64, len(streams))
			cards := make(map[string]map[string]string, len(streams))
			for name, stream := range streams {
				snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
				revealed := requireRevealedRound(t, snapshot, votes, []uuid.UUID{f.anaID}, "canal de "+name)
				versions[name] = snapshot.Version
				cards[name] = pokerRevealedCards(revealed)
			}
			stored := pokerStoredVersion(t, f.env.db, session.ID)
			for name := range streams {
				assert.Equal(t, stored, versions[name], "el canal de %s debería ver la versión guardada de la revelación", name)
				assert.Equal(t, cards["Ana"], cards[name], "el canal de %s debería ver los mismos votos que el de Ana", name)
			}

			// La base guarda la ronda revelada y la carta de Bruno que corresponde al resultado.
			requireSingleStoredVote(t, f.env.db, session.ID, f.brunoID, brunoCard)
			requireStoredRevealed(t, f.env.db, session.ID, f.env.clock.Now())
		})
	}
}

// Prueba de integración de la toma simultánea del rol de facilitador (T072; SC-025, FR-072): en cada
// repetición, sobre una sesión nueva, diez participantes conectados piden a la vez POST
// /api/v1/estimation-sessions/{sessionId}/facilitator con la facilitadora desconectada hace más de 5
// minutos, y al terminar se verifica que prosperó un único pedido y que la base guarda un único cambio
// de facilitador. No lleva la etiqueta de un escenario de la spec porque verifica un criterio de
// éxito; el escenario de dos pedidos simultáneos es TestUS6_TomasSimultaneas.

// Cantidad de repeticiones de la tanda de pedidos simultáneos de tomar el rol.
const pokerTakeoverRounds = 5

// pokerTakeAtOnce pide el rol de facilitador de la sesión con todos los tokens desde goroutines que
// esperan la misma señal de salida, y devuelve los resultados en el orden de los tokens. Cada
// goroutine escribe solo en su posición del slice.
func (e *pokerTestEnv) pokerTakeAtOnce(tokens []string, sessionID string) []pokerConcurrentResult {
	results := make([]pokerConcurrentResult, len(tokens))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() {
			<-start
			results[i] = e.pokerSend(http.MethodPost, facilitatorPath(sessionID), token, nil)
		})
	}
	close(start)
	wg.Wait()
	return results
}

// pokerTakeoverMember es una de las diez cuentas integrantes que piden el rol a la vez.
type pokerTakeoverMember struct {
	id       uuid.UUID
	fullName string
	token    string
}

// Ana, propietaria y facilitadora, abre en cada repetición una sesión nueva a la que se conectan
// ella y diez integrantes; Ana cierra su canal, vence la gracia y pasan 5 minutos. Los diez piden el
// rol a la vez: exactamente uno responde 200 con él como facilitador, los nueve restantes 409 con el
// mensaje de facilitador conectado y el ganador como vigente, ninguno 500, y en la base
// facilitator_id queda en el ganador y la versión sube en uno (SC-025, FR-072).
func TestPokerFacilitator_Concurrency(t *testing.T) {
	env := newPokerTestEnv(t)

	// Ana es propietaria de un proyecto En curso con diez cuentas integrantes.
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProject(t, anaToken, "Portal de Clientes")
	members := make([]pokerTakeoverMember, 0, pokerConcurrentAccounts)
	tokens := make([]string, 0, pokerConcurrentAccounts)
	everyone := []uuid.UUID{anaID}
	for i := range pokerConcurrentAccounts {
		name := fmt.Sprintf("Integrante %d", i+1)
		email := fmt.Sprintf("integrante%d@example.com", i+1)
		id, token := env.signUp(t, name, email)
		env.addMember(t, anaToken, projectID, email)
		members = append(members, pokerTakeoverMember{id: id, fullName: name, token: token})
		tokens = append(tokens, token)
		everyone = append(everyone, id)
	}
	env.setProjectStatus(t, anaToken, projectID, "in_progress")

	for round := range pokerTakeoverRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			// Sesión nueva con los once conectados; Ana se desconecta y el rol queda disponible.
			storyID := env.createStory(t, anaToken, projectID, pokerStoryInput{
				Title: fmt.Sprintf("Historia con relevo concurrente %d", round+1),
			})
			session := env.requireOpenedSession(t, anaToken, storyID)
			ana := env.openStream(t, anaToken, session.ID)
			streams := make([]*pokerStream, 0, len(members))
			for _, m := range members {
				streams = append(streams, env.openStream(t, m.token, session.ID))
			}
			all := allConnected(everyone...)
			ana.NextUntil(t, "los once conectados en el canal de Ana", all)
			for i, stream := range streams {
				stream.NextUntil(t, fmt.Sprintf("los once conectados en el canal de %s", members[i].fullName), all)
			}
			env.dropAndExpire(t, ana)
			streams[0].NextUntil(t, "Ana desconectada en el canal de "+members[0].fullName, participantGone(anaID))
			env.clock.Advance(pokerTakeoverWait)
			before := pokerStoredVersion(t, env.db, session.ID)

			results := env.pokerTakeAtOnce(tokens, session.ID)

			// Cada respuesta es una toma o un rechazo, y prospera una sola.
			var winners []int
			var conflicts []pokerResponse
			for i, res := range results {
				if !assert.NoError(t, res.err, "pedido %d sin respuesta", i+1) {
					continue
				}
				switch res.resp.Code {
				case http.StatusOK:
					winners = append(winners, i)
				case http.StatusConflict:
					conflicts = append(conflicts, res.resp)
				default:
					t.Errorf("pedido de %s: código %d inesperado; cuerpo: %s", members[i].fullName, res.resp.Code, res.resp.Body)
				}
			}
			require.Len(t, winners, 1, "debería prosperar exactamente un pedido")
			assert.Len(t, conflicts, pokerConcurrentAccounts-1, "los demás pedidos deberían responder 409")
			winner := members[winners[0]]

			// El 200 muestra al ganador como facilitador y cada 409 lo trae como vigente.
			var taken pokerSessionBody
			require.NoError(t, json.Unmarshal(results[winners[0]].resp.Body, &taken), "el 200 no es JSON válido: %s", results[winners[0]].resp.Body)
			taken.Raw = results[winners[0]].resp.Body
			requireFacilitatorRole(t, taken, winner.id, "respuesta de "+winner.fullName)
			for _, resp := range conflicts {
				requireFacilitatorUnavailable(t, resp, pokerFacilitatorPresentMessage, winner.id, winner.fullName, nil)
			}

			// En la base hay un único cambio de facilitador, a favor del ganador.
			requireStoredFacilitator(t, env.db, session.ID, winner.id, "prospera solo el primer pedido")
			assert.Equal(t, before+1, pokerStoredVersion(t, env.db, session.ID), "un único cambio de facilitador sube la versión en uno")
		})
	}
}

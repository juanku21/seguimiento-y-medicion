package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Prueba de escala del tiempo real (T092; SC-004, RC-14, FR-073): diez sesiones en paralelo con diez
// participantes conectados cada una, es decir cien canales de eventos abiertos a la vez contra el
// router compuesto como cmd/app y la base smye_test. Se emite un voto por sesión y se mide, para cada
// canal, el tiempo entre la respuesta del voto y la llegada de la instantánea con la nueva versión.
// No lleva la etiqueta de un escenario de la spec porque verifica un criterio de éxito, y se omite con
// go test -short porque abre cien conexiones.

// Cantidad de sesiones simultáneas y de participantes conectados en cada una que fija RC-14.
const (
	pokerScaleSessions     = 10
	pokerScaleParticipants = 10
)

// pokerScaleWaitCap acota la espera de cada canal después del voto. Es mayor que el límite de 2
// segundos para que un canal lento quede medido e informado en lugar de cortar la espera antes.
const pokerScaleWaitCap = 5 * time.Second

// pokerScaleArrival es lo que observó un canal después del voto: el instante en que llegó la
// instantánea con la versión esperada, o el motivo por el que no llegó.
type pokerScaleArrival struct {
	at  time.Time
	err error
}

// pokerScaleSession reúne una de las sesiones de la prueba con sus diez canales abiertos.
type pokerScaleSession struct {
	id      string
	streams []*pokerStream
}

// awaitVersion lee los eventos del canal hasta la primera instantánea cuya versión sea al menos la
// indicada y devuelve el instante en que llegó. No usa require porque corre en una goroutine distinta
// de la de la prueba; el fallo vuelve como valor.
func (s *pokerStream) awaitVersion(version func() int64, ready <-chan struct{}, deadline time.Time) pokerScaleArrival {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				return pokerScaleArrival{err: fmt.Errorf("el canal se cerró sin la instantánea del voto")}
			}
			at := time.Now()
			if ev.Name != "state" {
				return pokerScaleArrival{err: fmt.Errorf("llegó el evento %q en lugar de state: %s", ev.Name, ev.Data)}
			}
			var snapshot pokerSessionBody
			if err := json.Unmarshal(ev.Data, &snapshot); err != nil {
				return pokerScaleArrival{err: fmt.Errorf("instantánea inválida (%v): %s", err, ev.Data)}
			}
			// La versión del voto se conoce recién con su respuesta: si la instantánea llega antes,
			// se espera la respuesta para compararla.
			select {
			case <-ready:
			case <-timer.C:
				return pokerScaleArrival{err: fmt.Errorf("el voto no respondió a tiempo")}
			}
			if snapshot.Version >= version() {
				return pokerScaleArrival{at: at}
			}
		case <-timer.C:
			return pokerScaleArrival{err: fmt.Errorf("no llegó la instantánea del voto en %s", pokerScaleWaitCap)}
		}
	}
}

// Diez sesiones abiertas por Ana sobre diez historias del mismo proyecto, con Ana y nueve integrantes
// conectados a cada una; en cada sesión vota un integrante distinto, los diez a la vez. Cada voto
// responde 200 y cada uno de los cien canales recibe la instantánea con la versión del voto de su
// sesión en menos de 2 segundos medidos desde la respuesta del voto (SC-004, RC-14).
func TestPokerStream_Scale(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de escala con cien canales de eventos: se omite con -short")
	}
	env := newPokerTestEnv(t)

	// Ana es propietaria de un proyecto En curso con nueve integrantes: diez cuentas en total, que
	// son los diez participantes de cada sesión.
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := env.createProject(t, anaToken, "Portal de Clientes")
	ids := []uuid.UUID{anaID}
	tokens := []string{anaToken}
	for i := range pokerScaleParticipants - 1 {
		email := fmt.Sprintf("integrante%d@example.com", i+1)
		id, token := env.signUp(t, fmt.Sprintf("Integrante %d", i+1), email)
		env.addMember(t, anaToken, projectID, email)
		ids = append(ids, id)
		tokens = append(tokens, token)
	}
	env.setProjectStatus(t, anaToken, projectID, "in_progress")

	// Cada sesión se abre sobre una historia propia y las diez cuentas abren su canal en ella; se
	// espera a que cada canal vea a los diez conectados para que ninguna instantánea de la
	// incorporación se confunda con la del voto.
	everyone := allConnected(ids...)
	sessions := make([]pokerScaleSession, 0, pokerScaleSessions)
	for i := range pokerScaleSessions {
		storyID := env.createStory(t, anaToken, projectID, pokerStoryInput{
			Title: fmt.Sprintf("Historia de escala %d", i+1),
		})
		opened := env.requireOpenedSession(t, anaToken, storyID)
		session := pokerScaleSession{id: opened.ID}
		for _, token := range tokens {
			session.streams = append(session.streams, env.openStream(t, token, opened.ID))
		}
		for j, stream := range session.streams {
			stream.NextUntil(t, fmt.Sprintf("los diez conectados en el canal %d de la sesión %d", j+1, i+1), everyone)
		}
		sessions = append(sessions, session)
	}

	// Los cien lectores esperan desde antes del voto, para registrar la llegada aunque la
	// instantánea se adelante a la respuesta. En la sesión i vota el participante i+1 (los
	// integrantes), salvo la última, en la que vota Ana; los diez votos salen a la vez.
	payload, err := json.Marshal(map[string]any{"card": "5"})
	require.NoError(t, err)
	deadline := time.Now().Add(pokerScaleWaitCap)
	start := make(chan struct{})
	votes := make([]pokerConcurrentResult, pokerScaleSessions)
	votedAt := make([]time.Time, pokerScaleSessions)
	versions := make([]int64, pokerScaleSessions)
	arrivals := make([][]pokerScaleArrival, pokerScaleSessions)
	var wg sync.WaitGroup
	for i, session := range sessions {
		ready := make(chan struct{})
		arrivals[i] = make([]pokerScaleArrival, len(session.streams))
		for j, stream := range session.streams {
			wg.Go(func() {
				arrivals[i][j] = stream.awaitVersion(func() int64 { return versions[i] }, ready, deadline)
			})
		}
		voter := tokens[(i+1)%pokerScaleParticipants]
		wg.Go(func() {
			defer close(ready)
			<-start
			votes[i] = env.pokerSend(http.MethodPut, votePath(session.id), voter, payload)
			votedAt[i] = time.Now()
			var body pokerSessionBody
			if votes[i].err == nil && votes[i].resp.Code == http.StatusOK && json.Unmarshal(votes[i].resp.Body, &body) == nil {
				versions[i] = body.Version
			} else {
				versions[i] = 1<<63 - 1
			}
		})
	}
	close(start)
	wg.Wait()

	// Cada voto responde 200 y cada canal recibe la nueva versión dentro de los 2 segundos desde la
	// respuesta; si la instantánea llegó antes que la respuesta, la demora cuenta como cero.
	var worst time.Duration
	latest := time.Duration(-1 << 63)
	for i := range sessions {
		require.NoError(t, votes[i].err, "el voto de la sesión %d no obtuvo respuesta", i+1)
		require.Equal(t, http.StatusOK, votes[i].resp.Code, "voto de la sesión %d; cuerpo: %s", i+1, votes[i].resp.Body)
		for j, arrival := range arrivals[i] {
			if !assert.NoError(t, arrival.err, "canal %d de la sesión %d", j+1, i+1) {
				continue
			}
			latest = max(latest, arrival.at.Sub(votedAt[i]))
			delay := max(arrival.at.Sub(votedAt[i]), 0)
			worst = max(worst, delay)
			assert.Less(t, delay, pokerStreamDeadline, "canal %d de la sesión %d recibió la versión %d en %s",
				j+1, i+1, versions[i], delay)
		}
	}
	t.Logf("demora máxima desde la respuesta del voto: %s (llegada más tardía respecto de la respuesta, con signo: %s)",
		worst, latest)

	// Se cierran los cien canales antes de terminar, cada uno esperando a que el servidor libere su
	// conexión, para no dejar goroutines vivas entre pruebas.
	for _, session := range sessions {
		for _, stream := range session.streams {
			stream.Close()
		}
	}
}

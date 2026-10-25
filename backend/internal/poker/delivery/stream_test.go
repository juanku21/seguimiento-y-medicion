package delivery

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
)

// Prueba de integración del cierre del canal de eventos al vencer el token (T093; research.md,
// punto abierto 5): el middleware de sesión de la 001 solo corre al abrir el canal, así que el canal
// debe cerrarse por su cuenta cuando llega el exp de los claims. Usa reloj real, porque el
// vencimiento lo decide time.Now en la 001, y una sesión de autenticación de unos 2 segundos creada
// directamente en smye_test y firmada con el mismo emisor que usa el inicio de sesión. No lleva la
// etiqueta de un escenario porque spec.md no tiene un escenario sobre el vencimiento del token: la
// regla viene del punto abierto 5 de research.md y del plan.

// pokerTokenLifetime es la vida mínima del token corto de la prueba.
const pokerTokenLifetime = 2 * time.Second

// pokerExpiryMargin es cuánto se espera, después del vencimiento, a que el servidor cierre el canal.
const pokerExpiryMargin = 1500 * time.Millisecond

// pokerQuietWindow es cuánto se observa el canal vencido, después de un hecho de la sesión, para
// comprobar que no llega ninguna instantánea nueva.
const pokerQuietWindow = time.Second

// issueShortToken crea en la base de test una sesión de autenticación vigente de la cuenta que vence
// en pokerTokenLifetime o un poco más, y devuelve su JWT firmado como lo firma el inicio de sesión,
// junto con su vencimiento. El vencimiento se redondea hacia arriba al segundo porque el claim exp
// tiene precisión de segundos; así exp y expires_at coinciden exactamente.
func (e *pokerTestEnv) issueShortToken(t *testing.T, userID uuid.UUID) (string, time.Time) {
	t.Helper()
	now := time.Now()
	expiresAt := now.Add(pokerTokenLifetime).Truncate(time.Second).Add(time.Second)
	session := authdomain.Session{ID: uuid.New(), UserID: userID, CreatedAt: now, ExpiresAt: expiresAt}
	event := authdomain.AuthEvent{ID: uuid.New(), EventType: authdomain.EventLoginSucceeded, UserID: &userID}
	require.NoError(t, authrepository.NewSessionRepository(e.db).Create(context.Background(), &session, &event),
		"no se pudo crear la sesión de autenticación corta")
	token, err := authservice.IssueToken(e.cfg.JWTSecret, session)
	require.NoError(t, err, "no se pudo firmar el token corto")
	return token, expiresAt
}

// waitStreamClosed lee el canal hasta que el servidor lo cierra o hasta el instante límite, y
// devuelve si se cerró y cuándo. Las instantáneas que lleguen mientras tanto se descartan.
func waitStreamClosed(s *pokerStream, deadline time.Time) (bool, time.Time) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case _, ok := <-s.events:
			if !ok {
				return true, time.Now()
			}
		case <-timer.C:
			return false, time.Time{}
		}
	}
}

// TestPokerStream_CierreAlVencerElToken verifica que el canal de Bruno, abierto con un token que
// vence en unos 2 segundos, recibe instantáneas mientras el token es válido, que el servidor lo
// cierra al vencer y que un hecho posterior de la sesión ya no le llega, mientras el canal de Ana,
// con su token de 8 horas, sí lo recibe.
func TestPokerStream_CierreAlVencerElToken(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	ana := f.env.openStream(t, f.anaToken, session.ID)

	// Bruno se une con el token corto y, mientras es válido, su canal recibe la instantánea con los
	// dos conectados y la del voto de Ana.
	shortToken, expiresAt := f.env.issueShortToken(t, f.brunoID)
	bruno := f.env.openStream(t, shortToken, session.ID)
	both := allConnected(f.anaID, f.brunoID)
	bruno.NextUntil(t, "los dos conectados en el canal de Bruno", both)
	ana.NextUntil(t, "los dos conectados en el canal de Ana", both)
	f.env.requireVoted(t, f.anaToken, session.ID, "5")
	bruno.NextUntil(t, "Ana ya votó en el canal de Bruno", hasVoted(f.anaID))
	require.True(t, time.Now().Before(expiresAt),
		"la preparación tardó más que la vida del token; la prueba no puede verificar el canal vigente")

	// Al vencer el token, el servidor cierra el canal de Bruno: el cuerpo termina dentro del margen
	// y no antes del vencimiento. Que la 001 ya rechace el token confirma que de verdad venció.
	closed, closedAt := waitStreamClosed(bruno, expiresAt.Add(pokerExpiryMargin))
	requirePokerStatus(t, f.env.do(t, http.MethodGet, "/api/v1/estimation-sessions/"+session.ID, shortToken, nil),
		http.StatusUnauthorized, nil)
	if assert.True(t, closed, "el canal de Bruno sigue abierto %s después de vencer su token", pokerExpiryMargin) {
		assert.False(t, closedAt.Before(expiresAt.Add(-100*time.Millisecond)),
			"el canal se cerró en %s, antes del vencimiento del token en %s", closedAt, expiresAt)
	}

	// Un hecho posterior de la sesión llega al canal de Ana, que sigue vigente, y no al de Bruno.
	changed := f.env.requireVoted(t, f.anaToken, session.ID, "8")
	ana.NextUntil(t, "el cambio de voto de Ana en su canal", func(s pokerSessionBody) bool { return s.Version >= changed.Version })
	quiet := time.NewTimer(pokerQuietWindow)
	defer quiet.Stop()
	for {
		select {
		case ev, ok := <-bruno.events:
			if !ok {
				return
			}
			assert.Fail(t, "el canal de Bruno recibió un evento después de vencer su token",
				"evento %q con datos %s", ev.Name, ev.Data)
		case <-quiet.C:
			return
		}
	}
}

// Prueba de integración del cierre del canal de eventos al revocarse la sesión de autenticación
// (T107; hallazgo de seguridad de la fase Polish y research.md, punto abierto 5): el middleware de
// sesión de la 001 solo corre al abrir el canal, así que un cierre de sesión posterior no lo corta
// por sí solo; debe cortarlo el barrido periódico, como corta los de membresía revocada. Igual que la
// prueba del vencimiento, no lleva la etiqueta de un escenario porque spec.md no tiene un escenario
// sobre la sesión de autenticación con el canal abierto.

// waitStreamClosedWithoutState lee el canal hasta que el servidor lo cierra o hasta el instante
// límite, y devuelve si se cerró junto con los eventos state que llegaron mientras tanto. Los demás
// eventos, como removed, se descartan.
func waitStreamClosedWithoutState(s *pokerStream, deadline time.Time) (bool, []pokerStreamEvent) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	var states []pokerStreamEvent
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				return true, states
			}
			if ev.Name == "state" {
				states = append(states, ev)
			}
		case <-timer.C:
			return false, states
		}
	}
}

// TestPokerStream_CierreAlRevocarLaSesionDeAutenticacion verifica que el canal de Bruno, abierto con
// un token vigente, se cierra en la primera pasada del barrido después de que Bruno cierra su sesión
// de autenticación con POST /api/v1/auth/logout, que desde la revocación no recibe ningún state, ni
// siquiera el de un hecho posterior de la sesión, y que el canal de Ana sí lo recibe.
func TestPokerStream_CierreAlRevocarLaSesionDeAutenticacion(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	both := allConnected(f.anaID, f.brunoID)
	bruno.NextUntil(t, "los dos conectados en el canal de Bruno", both)
	ana.NextUntil(t, "los dos conectados en el canal de Ana", both)

	// Una pasada previa del barrido registra la historia como ya vista, así la pasada posterior a la
	// revocación no publica la sesión por un cambio de la historia; se consume esa instantánea en los
	// dos canales y la del voto de Ana, para que nada anterior a la revocación quede en camino.
	f.env.sweep(t, 0)
	f.env.requireVoted(t, f.anaToken, session.ID, "5")
	bruno.NextUntil(t, "Ana ya votó en el canal de Bruno", hasVoted(f.anaID))
	ana.NextUntil(t, "Ana ya votó en su canal", hasVoted(f.anaID))

	// Bruno cierra su sesión de autenticación; la 001 ya rechaza su token en las acciones REST.
	requirePokerStatus(t, f.env.do(t, http.MethodPost, "/api/v1/auth/logout", f.brunoToken, nil),
		http.StatusNoContent, nil)
	requirePokerStatus(t, f.env.do(t, http.MethodGet, "/api/v1/estimation-sessions/"+session.ID, f.brunoToken, nil),
		http.StatusUnauthorized, nil)

	// Una pasada del barrido cierra el canal de Bruno dentro del plazo de 2 segundos, sin enviarle
	// ningún state después de la revocación.
	f.env.sweep(t, 0)
	closed, states := waitStreamClosedWithoutState(bruno, time.Now().Add(pokerStreamDeadline))
	assert.True(t, closed, "el canal de Bruno sigue abierto %s después del barrido posterior a su cierre de sesión",
		pokerStreamDeadline)
	for _, ev := range states {
		assert.Fail(t, "el canal de Bruno recibió un state después de revocarse su sesión de autenticación",
			"datos %s", ev.Data)
	}

	// Un hecho posterior de la sesión llega al canal de Ana, que sigue vigente, y no al de Bruno.
	changed := f.env.requireVoted(t, f.anaToken, session.ID, "8")
	ana.NextUntil(t, "el cambio de voto de Ana en su canal", func(s pokerSessionBody) bool { return s.Version >= changed.Version })
	quiet := time.NewTimer(pokerQuietWindow)
	defer quiet.Stop()
	for {
		select {
		case ev, ok := <-bruno.events:
			if !ok {
				return
			}
			if ev.Name == "state" {
				assert.Fail(t, "el canal de Bruno recibió un state después de revocarse su sesión de autenticación",
					"datos %s", ev.Data)
			}
		case <-quiet.C:
			return
		}
	}
}

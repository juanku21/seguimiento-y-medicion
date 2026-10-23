package service

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Pruebas unitarias del hub de difusión (T012, research.md sección 7). Ninguna deriva de un
// escenario de spec.md: verifican el mecanismo que sostiene la difusión de US1. Los nombres llevan el
// prefijo "Hub" para no chocar con las demás pruebas del paquete.

// hubWait es el plazo máximo para esperar una entrega; hubQuiet, cuánto se espera para afirmar que
// no llegó nada.
const (
	hubWait  = time.Second
	hubQuiet = 50 * time.Millisecond
)

// versionView devuelve un viewFor que arma, para cada usuario, una vista con la versión dada y el
// usuario como facilitador, de modo que la prueba pueda saber a quién se armó cada vista.
func versionView(sessionID uuid.UUID, version int64) func(uuid.UUID) domain.SessionView {
	return func(userID uuid.UUID) domain.SessionView {
		return domain.SessionView{
			ID:          sessionID,
			Version:     version,
			Facilitator: domain.UserRef{ID: userID},
		}
	}
}

// subscribe suscribe y exige un canal no nulo, para que una prueba nunca quede bloqueada leyendo un
// canal nil.
func subscribe(t *testing.T, h *Hub, sessionID, userID uuid.UUID) (<-chan domain.SessionView, func()) {
	t.Helper()
	ch, unsubscribe := h.Subscribe(sessionID, userID)
	require.NotNil(t, ch, "Subscribe debe devolver un canal")
	require.NotNil(t, unsubscribe, "Subscribe debe devolver la función para darse de baja")
	return ch, unsubscribe
}

// receiveView espera una vista del canal dentro del plazo y falla si no llega o si el canal se cerró.
func receiveView(t *testing.T, ch <-chan domain.SessionView) domain.SessionView {
	t.Helper()
	select {
	case v, ok := <-ch:
		require.True(t, ok, "el canal no debía estar cerrado")
		return v
	case <-time.After(hubWait):
		require.FailNow(t, "no llegó ninguna vista dentro del plazo")
		return domain.SessionView{}
	}
}

// assertNoView afirma que no hay una vista para leer: el canal sigue vacío o ya fue cerrado.
func assertNoView(t *testing.T, ch <-chan domain.SessionView) {
	t.Helper()
	select {
	case v, ok := <-ch:
		if ok {
			assert.Fail(t, "no debía llegar ninguna vista", "llegó la versión %d", v.Version)
		}
	case <-time.After(hubQuiet):
	}
}

// assertClosed afirma que el canal está cerrado, descartando a lo sumo una vista pendiente.
func assertClosed(t *testing.T, ch <-chan domain.SessionView) {
	t.Helper()
	for i := 0; i < 2; i++ {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-time.After(hubWait):
			assert.Fail(t, "el canal debía estar cerrado")
			return
		}
	}
	assert.Fail(t, "el canal debía estar cerrado y seguía entregando vistas")
}

// Research sección 7, no un escenario de la spec: Publish entrega a cada suscripción de la sesión la
// vista armada para su propio usuario, incluidas las dos conexiones de una misma persona.
func TestUS1_HubPublicarEntregaATodasLasSuscripcionesDeLaSesion(t *testing.T) {
	h := NewHub()
	sessionID, alice, bob := uuid.New(), uuid.New(), uuid.New()
	aliceTab1, _ := subscribe(t, h, sessionID, alice)
	aliceTab2, _ := subscribe(t, h, sessionID, alice)
	bobCh, _ := subscribe(t, h, sessionID, bob)

	h.Publish(sessionID, versionView(sessionID, 1))

	for _, ch := range []<-chan domain.SessionView{aliceTab1, aliceTab2} {
		v := receiveView(t, ch)
		assert.Equal(t, int64(1), v.Version)
		assert.Equal(t, alice, v.Facilitator.ID, "la vista se arma para el usuario de la suscripción")
	}
	v := receiveView(t, bobCh)
	assert.Equal(t, int64(1), v.Version)
	assert.Equal(t, bob, v.Facilitator.ID, "la vista se arma para el usuario de la suscripción")
}

// Research sección 7, no un escenario de la spec: publicar en una sesión no entrega nada a las
// suscripciones de otra, aunque sean de la misma persona.
func TestUS1_HubPublicarNoEntregaAOtraSesion(t *testing.T) {
	h := NewHub()
	sessionA, sessionB, userID := uuid.New(), uuid.New(), uuid.New()
	chA, _ := subscribe(t, h, sessionA, userID)
	chB, _ := subscribe(t, h, sessionB, userID)

	h.Publish(sessionA, versionView(sessionA, 1))

	assert.Equal(t, sessionA, receiveView(t, chA).ID)
	assertNoView(t, chB)
}

// Research sección 7, no un escenario de la spec: con el canal lleno, la vista pendiente se
// reemplaza por la nueva ("la última gana") y Publish no se bloquea esperando al lector.
func TestUS1_HubLaUltimaVistaGana(t *testing.T) {
	h := NewHub()
	sessionID, userID := uuid.New(), uuid.New()
	ch, _ := subscribe(t, h, sessionID, userID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for version := int64(1); version <= 3; version++ {
			h.Publish(sessionID, versionView(sessionID, version))
		}
	}()
	select {
	case <-done:
	case <-time.After(hubWait):
		require.FailNow(t, "Publish no debe bloquearse con el canal lleno")
	}

	assert.Equal(t, int64(3), receiveView(t, ch).Version, "solo queda pendiente la última vista")
	assertNoView(t, ch)
}

// Research sección 7, no un escenario de la spec: quien se da de baja no recibe más vistas, y darse
// de baja dos veces no falla ni afecta a las demás suscripciones.
func TestUS1_HubDarseDeBaja(t *testing.T) {
	h := NewHub()
	sessionID, userID := uuid.New(), uuid.New()
	ch, unsubscribe := subscribe(t, h, sessionID, userID)
	other, _ := subscribe(t, h, sessionID, userID)

	unsubscribe()
	assert.NotPanics(t, unsubscribe, "darse de baja dos veces no falla")

	h.Publish(sessionID, versionView(sessionID, 1))
	assertNoView(t, ch)
	assert.Equal(t, int64(1), receiveView(t, other).Version, "la otra suscripción sigue recibiendo")
}

// Research sección 7, no un escenario de la spec: CloseSession cierra todos los canales de la sesión
// y ninguno de otra; después, publicar o darse de baja no falla.
func TestUS1_HubCerrarSesion(t *testing.T) {
	h := NewHub()
	sessionID, otherSession := uuid.New(), uuid.New()
	alice, bob := uuid.New(), uuid.New()
	aliceCh, aliceUnsubscribe := subscribe(t, h, sessionID, alice)
	bobCh, _ := subscribe(t, h, sessionID, bob)
	otherCh, _ := subscribe(t, h, otherSession, alice)

	h.CloseSession(sessionID)

	assertClosed(t, aliceCh)
	assertClosed(t, bobCh)
	assert.NotPanics(t, func() { h.Publish(sessionID, versionView(sessionID, 2)) },
		"publicar en una sesión cerrada no falla")
	assert.NotPanics(t, aliceUnsubscribe, "darse de baja después del cierre no falla")

	h.Publish(otherSession, versionView(otherSession, 1))
	assert.Equal(t, otherSession, receiveView(t, otherCh).ID, "la otra sesión sigue abierta")
}

// Research sección 7, no un escenario de la spec: CloseUser cierra todos los canales de una persona
// en una sesión, sin tocar los de otras personas ni los de esa persona en otra sesión.
func TestUS1_HubCerrarUsuario(t *testing.T) {
	h := NewHub()
	sessionID, otherSession := uuid.New(), uuid.New()
	alice, bob := uuid.New(), uuid.New()
	aliceTab1, aliceUnsubscribe := subscribe(t, h, sessionID, alice)
	aliceTab2, _ := subscribe(t, h, sessionID, alice)
	bobCh, _ := subscribe(t, h, sessionID, bob)
	aliceElsewhere, _ := subscribe(t, h, otherSession, alice)

	h.CloseUser(sessionID, alice)

	assertClosed(t, aliceTab1)
	assertClosed(t, aliceTab2)
	assert.NotPanics(t, aliceUnsubscribe, "darse de baja después del cierre no falla")

	h.Publish(sessionID, versionView(sessionID, 1))
	assert.Equal(t, bob, receiveView(t, bobCh).Facilitator.ID, "las demás personas siguen recibiendo")
	h.Publish(otherSession, versionView(otherSession, 1))
	assert.Equal(t, otherSession, receiveView(t, aliceElsewhere).ID,
		"la misma persona en otra sesión sigue recibiendo")
}

// Prueba de concurrencia, no un escenario de la spec: suscripciones, publicaciones, bajas y cierres
// simultáneos sobre la misma sesión no fallan ni se bloquean. Se ejecuta con go test -race para
// detectar accesos sin sincronizar.
func TestUS1_HubUsoConcurrente(t *testing.T) {
	const n = 50
	h := NewHub()
	sessionID := uuid.New()
	first, _ := subscribe(t, h, sessionID, uuid.New())

	// Cada vuelta suscribe a una persona y lanza en paralelo una publicación, su doble baja, el
	// cierre de sus canales y una suscripción efímera de otra persona.
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		userID := uuid.New()
		_, unsubscribe := subscribe(t, h, sessionID, userID)
		wg.Add(4)
		go func() {
			defer wg.Done()
			h.Publish(sessionID, versionView(sessionID, int64(i)))
		}()
		go func() {
			defer wg.Done()
			unsubscribe()
			unsubscribe()
		}()
		go func() {
			defer wg.Done()
			h.CloseUser(sessionID, userID)
		}()
		go func() {
			defer wg.Done()
			extra, extraUnsubscribe := h.Subscribe(sessionID, uuid.New())
			if extra != nil {
				extraUnsubscribe()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "las operaciones concurrentes no deben bloquearse")
	}

	// Al cerrar la sesión se cierra el canal que nunca se dio de baja.
	h.CloseSession(sessionID)
	assertClosed(t, first)
}

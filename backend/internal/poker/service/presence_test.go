package service

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de Presence (T011, RN-23 y RN-34, research.md sección 9). Todas
// usan instantes explícitos a partir de t0, sin esperas reales. Los nombres llevan el prefijo
// "Presencia" para no chocar con las pruebas de integración de US1; las que no derivan de un
// escenario de spec.md lo indican en su comentario.

// t0 es el instante de referencia de todas las pruebas de presencia.
var t0 = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// newPair devuelve un par (sesión, usuario) nuevo y la clave esperada en Expired.
func newPair() (uuid.UUID, uuid.UUID, PresenceKey) {
	sessionID, userID := uuid.New(), uuid.New()
	return sessionID, userID, PresenceKey{SessionID: sessionID, UserID: userID}
}

// Regla RN-34, no un escenario de la spec: una persona es un único participante aunque abra varias
// conexiones, así que solo la primera conexión del par es una llegada.
func TestUS1_PresenciaPrimeraConexionInformaLlegada(t *testing.T) {
	p := NewPresence()
	sessionID, userID, _ := newPair()

	assert.True(t, p.Connect(sessionID, userID), "la primera conexión del par debe informar la llegada")
	assert.False(t, p.Connect(sessionID, userID), "la segunda conexión del mismo par no es una llegada")
}

// Regla RN-34, no un escenario de la spec: cerrar una de dos conexiones deja a la persona conectada,
// sin caída anotada, de modo que ningún vencimiento posterior la informa.
func TestUS1_PresenciaCierreDeUnaDeDosConexiones(t *testing.T) {
	p := NewPresence()
	sessionID, userID, key := newPair()

	// Control: la misma secuencia con una sola conexión sí vence, para que la prueba no pase
	// porque Expired nunca devuelve nada.
	otherSession, otherUser, otherKey := newPair()
	p.Connect(otherSession, otherUser)
	p.Release(otherSession, otherUser, t0)

	p.Connect(sessionID, userID)
	p.Connect(sessionID, userID)
	p.Release(sessionID, userID, t0)

	expired := p.Expired(t0.Add(time.Hour))
	assert.NotContains(t, expired, key, "con una conexión viva la persona no vence")
	assert.Contains(t, expired, otherKey, "el par de control sin conexiones vivas sí vence")
}

// Escenario: Abandono de un participante (005/US1)
// Al cerrar la última conexión la persona sigue contando como conectada: se anota el instante y
// recién vence cuando pasaron 30 segundos o más (FR-014). A los 29 segundos no vence; a los 30 sí,
// y Expired la quita, así que una segunda consulta ya no la devuelve.
func TestUS1_PresenciaAbandonoDeUnParticipante(t *testing.T) {
	t.Run("a los 29 segundos no vence", func(t *testing.T) {
		p := NewPresence()
		sessionID, userID, key := newPair()
		p.Connect(sessionID, userID)
		p.Release(sessionID, userID, t0)

		assert.NotContains(t, p.Expired(t0.Add(29*time.Second)), key)
		assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(30*time.Second)),
			"una consulta a los 29 s no debe quitar el par antes de que venza")
	})

	t.Run("a los 30 segundos vence y se quita", func(t *testing.T) {
		p := NewPresence()
		sessionID, userID, key := newPair()
		p.Connect(sessionID, userID)
		p.Release(sessionID, userID, t0)

		assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(30*time.Second)))
		assert.Empty(t, p.Expired(t0.Add(time.Minute)), "un par vencido no se devuelve dos veces")
	})

	t.Run("después de vencer, volver a conectarse es una llegada", func(t *testing.T) {
		p := NewPresence()
		sessionID, userID, key := newPair()
		p.Connect(sessionID, userID)
		p.Release(sessionID, userID, t0)
		assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(45*time.Second)))

		assert.True(t, p.Connect(sessionID, userID), "quien ya figuraba desconectado vuelve a llegar")
	})
}

// Regla RN-23, no un escenario de la spec: quien vuelve dentro de los 30 segundos de gracia no
// generó ningún hecho; la reconexión borra la caída anotada sin informar una llegada.
func TestUS1_PresenciaReconexionDentroDeLaGracia(t *testing.T) {
	p := NewPresence()
	sessionID, userID, key := newPair()
	p.Connect(sessionID, userID)
	p.Release(sessionID, userID, t0)

	assert.False(t, p.Connect(sessionID, userID), "volver dentro de la gracia no es una llegada")
	assert.NotContains(t, p.Expired(t0.Add(time.Hour)), key, "la reconexión borra la caída anotada")

	// Una nueva caída se mide desde su propio instante, no desde la anterior.
	p.Release(sessionID, userID, t0.Add(20*time.Second))
	assert.NotContains(t, p.Expired(t0.Add(49*time.Second)), key)
	assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(50*time.Second)))
}

// Regla de FR-002 (research.md sección 9), no un escenario de la spec: quien abre la sesión figura
// conectado sin canal, con una caída anotada en el instante de apertura. Si no abre el canal, vence
// a los 30 segundos; si lo abre dentro de la gracia, no hay llegada ni vencimiento.
func TestUS1_PresenciaQuienAbreSinCanal(t *testing.T) {
	t.Run("sin canal vence a los 30 segundos", func(t *testing.T) {
		p := NewPresence()
		sessionID, userID, key := newPair()
		p.MarkPending(sessionID, userID, t0)

		assert.NotContains(t, p.Expired(t0.Add(29*time.Second)), key)
		assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(30*time.Second)))
	})

	t.Run("abrir el canal dentro de la gracia no es una llegada", func(t *testing.T) {
		p := NewPresence()
		sessionID, userID, key := newPair()
		p.MarkPending(sessionID, userID, t0)

		assert.False(t, p.Connect(sessionID, userID), "quien abrió ya figura conectado")
		assert.NotContains(t, p.Expired(t0.Add(time.Hour)), key)

		// Control: la marca existía, porque al soltar el canal vuelve a vencer.
		p.Release(sessionID, userID, t0.Add(10*time.Second))
		assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(40*time.Second)))
	})
}

// Regla RN-34, no un escenario de la spec: la presencia se cuenta por par (sesión, usuario), así que
// la misma persona en dos sesiones tiene conexiones y caídas independientes.
func TestUS1_PresenciaSesionesDistintasPorSeparado(t *testing.T) {
	p := NewPresence()
	userID := uuid.New()
	sessionA, sessionB := uuid.New(), uuid.New()
	keyA := PresenceKey{SessionID: sessionA, UserID: userID}
	keyB := PresenceKey{SessionID: sessionB, UserID: userID}

	assert.True(t, p.Connect(sessionA, userID), "llegada a la sesión A")
	assert.True(t, p.Connect(sessionB, userID), "llegada a la sesión B, independiente de A")

	// Soltar la sesión A no afecta la conexión viva en B.
	p.Release(sessionA, userID, t0)
	assert.Equal(t, []PresenceKey{keyA}, p.Expired(t0.Add(30*time.Second)))
	assert.NotContains(t, p.Expired(t0.Add(time.Hour)), keyB)
}

// Prueba de concurrencia, no un escenario de la spec: muchas conexiones y cierres simultáneos del
// mismo par, junto con consultas a Expired, dejan el contador consistente. Se ejecuta con
// go test -race para detectar accesos sin sincronizar.
func TestUS1_PresenciaUsoConcurrente(t *testing.T) {
	const n = 50
	p := NewPresence()
	sessionID, userID, key := newPair()

	// Fase 1: n conexiones en paralelo, de las cuales exactamente una es la llegada.
	var wg sync.WaitGroup
	var mu sync.Mutex
	arrivals := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if p.Connect(sessionID, userID) {
				mu.Lock()
				arrivals++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, arrivals, "solo una de las conexiones simultáneas es la llegada")

	// Fase 2: n cierres en paralelo mientras se consulta Expired antes de la gracia.
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			p.Release(sessionID, userID, t0)
		}()
		go func() {
			defer wg.Done()
			p.Expired(t0.Add(time.Second))
		}()
	}
	wg.Wait()

	assert.Equal(t, []PresenceKey{key}, p.Expired(t0.Add(30*time.Second)),
		"tras cerrar todas las conexiones el par vence una sola vez")
}

package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de FacilitatorTakeover (T070, RN-24, RN-25, RN-26, RN-35, SC-015):
// el rol de facilitador solo se puede tomar cuando el vigente lleva 5 minutos o más desconectado; si
// está conectado, o lleva menos, el pedido se rechaza con FacilitatorUnavailableError, que nombra al
// facilitador vigente y dice cuánto falta (cero si está conectado). Quien ya es el facilitador
// tampoco puede tomarlo. Los nombres llevan el prefijo "Dominio", convención de las pruebas de
// dominio del proyecto, para no chocar con las pruebas de integración de US6.

// takeoverNow es el instante fijo de los pedidos, para medir las desconexiones sin reloj real.
var takeoverNow = time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)

// connectedFacilitator arma al facilitador vigente conectado, sin momento de desconexión.
func connectedFacilitator() SnapshotParticipant {
	return SnapshotParticipant{User: UserRef{ID: uuid.New(), FullName: "Ana Facilitadora"}, Connected: true}
}

// facilitatorDisconnectedFor arma al facilitador vigente desconectado desde hace elapsed respecto de
// takeoverNow.
func facilitatorDisconnectedFor(elapsed time.Duration) SnapshotParticipant {
	since := takeoverNow.Add(-elapsed)
	return SnapshotParticipant{
		User:           UserRef{ID: uuid.New(), FullName: "Ana Facilitadora"},
		Connected:      false,
		DisconnectedAt: &since,
	}
}

// requireFacilitatorUnavailable verifica que el error sea un FacilitatorUnavailableError y lo
// devuelve para inspeccionar sus campos.
func requireFacilitatorUnavailable(t *testing.T, err error) *FacilitatorUnavailableError {
	t.Helper()
	var unavailable *FacilitatorUnavailableError
	require.True(t, errors.As(err, &unavailable), "se esperaba FacilitatorUnavailableError, se obtuvo %v", err)
	return unavailable
}

// Escenario: Sobrevivir a las desconexiones y relevar al facilitador - Caso de error: facilitador presente (005/US6)
// Con el facilitador conectado, otro participante no puede tomar el rol: el rechazo nombra al
// facilitador vigente y no indica espera, porque el rol no se libera mientras siga presente (FR-069).
func TestUS6_DominioFacilitadorPresente(t *testing.T) {
	facilitator := connectedFacilitator()

	unavailable := requireFacilitatorUnavailable(t, FacilitatorTakeover(takeoverNow, facilitator, uuid.New()))
	assert.Equal(t, facilitator.User, unavailable.Facilitator)
	assert.Zero(t, unavailable.RetryAfter)
}

// Escenario: Sobrevivir a las desconexiones y relevar al facilitador - Caso de error: rol todavía no disponible (005/US6)
// Con el facilitador desconectado hace menos de 5 minutos, el pedido se rechaza indicando cuánto
// falta para que el rol quede disponible (FR-069, SC-015). Cada desconexión es un subtest: a los 2
// minutos faltan 3, y a los 4:59 falta 1 segundo.
func TestUS6_DominioRolTodaviaNoDisponible(t *testing.T) {
	cases := []struct {
		name       string
		elapsed    time.Duration
		retryAfter time.Duration
	}{
		{name: "desconectado hace 2 minutos", elapsed: 2 * time.Minute, retryAfter: 3 * time.Minute},
		{name: "desconectado hace 4:59", elapsed: 4*time.Minute + 59*time.Second, retryAfter: time.Second},
	}

	// Cada desconexión se prueba por separado para identificar cuál falla.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facilitator := facilitatorDisconnectedFor(tc.elapsed)

			unavailable := requireFacilitatorUnavailable(t, FacilitatorTakeover(takeoverNow, facilitator, uuid.New()))
			assert.Equal(t, facilitator.User, unavailable.Facilitator)
			assert.Equal(t, tc.retryAfter, unavailable.RetryAfter)
		})
	}
}

// Escenario: Sobrevivir a las desconexiones y relevar al facilitador - Caso límite: facilitador original relevado (005/US6)
// Con el facilitador desconectado hace 5 minutos o más, cualquier otro participante puede tomar el
// rol (FR-068, RN-25). Se verifica el límite exacto de 5:00 y una desconexión más larga, de 7:00, cada
// una en su subtest.
func TestUS6_DominioRolDisponible(t *testing.T) {
	// Cada desconexión se prueba por separado para identificar cuál falla.
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
	}{
		{name: "desconectado hace 5:00", elapsed: 5 * time.Minute},
		{name: "desconectado hace 7:00", elapsed: 7 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, FacilitatorTakeover(takeoverNow, facilitatorDisconnectedFor(tc.elapsed), uuid.New()))
		})
	}
}

// Contraparte de las reglas RN-26 y RN-35, no un escenario de la spec: quien ya es el facilitador
// recibe el mismo rechazo que cualquiera ante un facilitador conectado, con su propio nombre y sin
// espera. Es lo que ve el segundo de dos pedidos simultáneos cuando el primero ya tomó el rol.
func TestUS6_DominioQuienPideYaEsFacilitador(t *testing.T) {
	facilitator := connectedFacilitator()

	unavailable := requireFacilitatorUnavailable(t, FacilitatorTakeover(takeoverNow, facilitator, facilitator.User.ID))
	assert.Equal(t, facilitator.User, unavailable.Facilitator)
	assert.Zero(t, unavailable.RetryAfter)
}

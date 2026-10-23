package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de SessionStatus.EnsureOpen (T010, RN-22): una sesión open admite
// la acción aunque reciba motivo o valor, y una sesión finished o canceled se rechaza con
// SessionClosedError, que copia el estado, el motivo de cancelación y el valor acordado recibidos.
// Los nombres llevan el prefijo "Dominio", convención de las pruebas de dominio del proyecto, para
// no chocar con las pruebas de integración de US1.

// intPtr y reasonPtr devuelven punteros a valores literales para armar las entradas opcionales.
func intPtr(v int) *int { return &v }

func reasonPtr(r CancelReason) *CancelReason { return &r }

// requireSessionClosed verifica que el error sea un SessionClosedError y lo devuelve para
// inspeccionar sus campos.
func requireSessionClosed(t *testing.T, err error) *SessionClosedError {
	t.Helper()
	var closed *SessionClosedError
	require.True(t, errors.As(err, &closed), "se esperaba SessionClosedError, se obtuvo %v", err)
	return closed
}

// Contraparte de la regla RN-22, no un escenario de la spec: una sesión open admite la acción sin
// error, con o sin motivo de cancelación y valor acordado en las entradas, porque el estado es lo
// único que decide.
func TestUS1_DominioSesionAbierta(t *testing.T) {
	cases := []struct {
		name   string
		reason *CancelReason
		points *int
	}{
		{name: "sin motivo ni valor", reason: nil, points: nil},
		{name: "con motivo inactivity", reason: reasonPtr(CancelReasonInactivity), points: nil},
		{name: "con valor 8", reason: nil, points: intPtr(8)},
	}

	// Cada combinación de entradas se prueba por separado para identificar cuál falla.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, SessionStatusOpen.EnsureOpen(tc.reason, tc.points))
		})
	}
}

// Escenario: Unirse a una sesión - Sesión Finalizada o Cancelada (005/US1)
// Una sesión cerrada se rechaza con SessionClosedError que informa el estado y el resultado
// registrado (FR-018, RN-22). Las variantes son finished con valor 8, canceled por inactividad y
// canceled sin motivo, cada una en su subtest.
func TestUS1_DominioSesionFinalizadaOCancelada(t *testing.T) {
	// Sesión finished con valor acordado 8: el valor se copia al error y no hay motivo.
	t.Run("finished con valor 8", func(t *testing.T) {
		err := SessionStatusFinished.EnsureOpen(nil, intPtr(8))

		closed := requireSessionClosed(t, err)
		assert.Equal(t, SessionStatusFinished, closed.Status)
		require.NotNil(t, closed.AgreedStoryPoints)
		assert.Equal(t, 8, *closed.AgreedStoryPoints)
		assert.Nil(t, closed.CancelReason)
	})

	// Cancelación del sistema por inactividad: el motivo se copia al error.
	t.Run("canceled por inactividad", func(t *testing.T) {
		err := SessionStatusCanceled.EnsureOpen(reasonPtr(CancelReasonInactivity), nil)

		closed := requireSessionClosed(t, err)
		assert.Equal(t, SessionStatusCanceled, closed.Status)
		require.NotNil(t, closed.CancelReason)
		assert.Equal(t, CancelReasonInactivity, *closed.CancelReason)
		assert.Nil(t, closed.AgreedStoryPoints)
	})

	// Cancelación del facilitador: no hay motivo y el error lo refleja con nil.
	t.Run("canceled sin motivo", func(t *testing.T) {
		err := SessionStatusCanceled.EnsureOpen(nil, nil)

		closed := requireSessionClosed(t, err)
		assert.Equal(t, SessionStatusCanceled, closed.Status)
		assert.Nil(t, closed.CancelReason)
		assert.Nil(t, closed.AgreedStoryPoints)
	})
}

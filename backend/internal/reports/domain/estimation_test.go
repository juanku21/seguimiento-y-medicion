package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// day devuelve un instante fijo del mes de la prueba, desplazado en días, para armar las sesiones.
func day(n int) time.Time {
	return time.Date(2026, 3, n, 12, 0, 0, 0, time.UTC)
}

// dayPtr devuelve un puntero al instante de day(n).
func dayPtr(n int) *time.Time {
	t := day(n)
	return &t
}

// newSession arma una sesión con estado, creación, cierre, valor acordado y rondas dados.
func newSession(id uuid.UUID, status pokerdomain.SessionStatus, created int, closed *time.Time, points *int, rounds int) SessionRecord {
	return SessionRecord{ID: id, Status: status, CreatedAt: day(created), ClosedAt: closed, AgreedStoryPoints: points, Rounds: rounds}
}

func intPtr(v int) *int { return &v }

// Escenario: Una historia con sesión de Planning Poker finalizada muestra su resumen (010/US1)
func TestUS1_SelectEstimationUltimaFinalizada(t *testing.T) {
	idLow := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	idHigh := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	t.Run("una finalizada devuelve acordado con rondas y valor", func(t *testing.T) {
		sessions := []SessionRecord{newSession(idLow, pokerdomain.SessionStatusFinished, 1, dayPtr(2), intPtr(8), 3)}
		state, summary := SelectEstimation(sessions, nil)
		assert.Equal(t, PokerAgreed, state)
		if assert.NotNil(t, summary) {
			assert.Equal(t, 3, summary.Rounds)
			assert.Equal(t, 8, summary.AgreedStoryPoints)
			assert.Equal(t, day(2), summary.FinishedAt)
		}
	})

	t.Run("dos finalizadas elige la de mayor closed_at", func(t *testing.T) {
		sessions := []SessionRecord{
			newSession(idHigh, pokerdomain.SessionStatusFinished, 1, dayPtr(3), intPtr(13), 4),
			newSession(idLow, pokerdomain.SessionStatusFinished, 1, dayPtr(5), intPtr(5), 2),
		}
		state, summary := SelectEstimation(sessions, nil)
		assert.Equal(t, PokerAgreed, state)
		if assert.NotNil(t, summary) {
			assert.Equal(t, 2, summary.Rounds)
			assert.Equal(t, 5, summary.AgreedStoryPoints)
		}
	})

	t.Run("empate de closed_at elige el mayor id sin depender del orden", func(t *testing.T) {
		a := newSession(idLow, pokerdomain.SessionStatusFinished, 1, dayPtr(4), intPtr(3), 1)
		b := newSession(idHigh, pokerdomain.SessionStatusFinished, 1, dayPtr(4), intPtr(8), 2)
		for _, sessions := range [][]SessionRecord{{a, b}, {b, a}} {
			state, summary := SelectEstimation(sessions, nil)
			assert.Equal(t, PokerAgreed, state)
			if assert.NotNil(t, summary) {
				assert.Equal(t, 8, summary.AgreedStoryPoints)
				assert.Equal(t, 2, summary.Rounds)
			}
		}
	})

	t.Run("sin corte todas cuentan", func(t *testing.T) {
		sessions := []SessionRecord{newSession(idLow, pokerdomain.SessionStatusFinished, 20, dayPtr(25), intPtr(2), 1)}
		state, summary := SelectEstimation(sessions, nil)
		assert.Equal(t, PokerAgreed, state)
		assert.NotNil(t, summary)
	})
}

// Escenario: Una historia sin sesión de Planning Poker indica que no hubo estimación (010/US1)
func TestUS1_SelectEstimationSinSesiones(t *testing.T) {
	t.Run("lista vacía", func(t *testing.T) {
		state, summary := SelectEstimation(nil, nil)
		assert.Equal(t, PokerNone, state)
		assert.Nil(t, summary)
	})

	t.Run("una sesión creada después del corte no cuenta", func(t *testing.T) {
		sessions := []SessionRecord{newSession(uuid.New(), pokerdomain.SessionStatusFinished, 20, dayPtr(21), intPtr(5), 2)}
		state, summary := SelectEstimation(sessions, dayPtr(10))
		assert.Equal(t, PokerNone, state)
		assert.Nil(t, summary)
	})
}

// Escenario: Una historia con sesiones canceladas y ninguna finalizada no tiene valor acordado (010/US1)
func TestUS1_SelectEstimationSinValorAcordado(t *testing.T) {
	t.Run("solo canceladas", func(t *testing.T) {
		sessions := []SessionRecord{newSession(uuid.New(), pokerdomain.SessionStatusCanceled, 1, dayPtr(2), nil, 1)}
		state, summary := SelectEstimation(sessions, nil)
		assert.Equal(t, PokerNoAgreement, state)
		assert.Nil(t, summary)
	})

	t.Run("una sesión abierta", func(t *testing.T) {
		sessions := []SessionRecord{newSession(uuid.New(), pokerdomain.SessionStatusOpen, 1, nil, nil, 1)}
		state, summary := SelectEstimation(sessions, nil)
		assert.Equal(t, PokerNoAgreement, state)
		assert.Nil(t, summary)
	})

	t.Run("finalizada después del corte pero creada antes", func(t *testing.T) {
		sessions := []SessionRecord{newSession(uuid.New(), pokerdomain.SessionStatusFinished, 5, dayPtr(15), intPtr(8), 3)}
		state, summary := SelectEstimation(sessions, dayPtr(10))
		assert.Equal(t, PokerNoAgreement, state)
		assert.Nil(t, summary)
	})
}

// Escenario: El reporte de un sprint cerrado usa la estimación vigente al cierre (010/US1)
func TestUS1_SelectEstimationConCorte(t *testing.T) {
	t.Run("la finalizada anterior al corte gana a otra posterior", func(t *testing.T) {
		sessions := []SessionRecord{
			newSession(uuid.New(), pokerdomain.SessionStatusFinished, 1, dayPtr(3), intPtr(5), 2),
			newSession(uuid.New(), pokerdomain.SessionStatusFinished, 6, dayPtr(15), intPtr(13), 4),
		}
		state, summary := SelectEstimation(sessions, dayPtr(10))
		assert.Equal(t, PokerAgreed, state)
		if assert.NotNil(t, summary) {
			assert.Equal(t, 5, summary.AgreedStoryPoints)
			assert.Equal(t, 2, summary.Rounds)
		}
	})

	t.Run("finalizada exactamente en el corte cuenta", func(t *testing.T) {
		sessions := []SessionRecord{newSession(uuid.New(), pokerdomain.SessionStatusFinished, 1, dayPtr(10), intPtr(5), 2)}
		state, _ := SelectEstimation(sessions, dayPtr(10))
		assert.Equal(t, PokerAgreed, state)
	})
}

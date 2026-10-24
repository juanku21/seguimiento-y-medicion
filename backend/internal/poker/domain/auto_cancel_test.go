package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Pruebas unitarias obligatorias de AutoCancelReason (T078, RN-30, RN-31, SC-021) con la tabla de
// data-model.md sección 6: una historia completada cancela con story_completed, un proyecto
// finalizado con project_finished, y 60 minutos o más sin hechos con inactivity; la historia
// completada tiene precedencia sobre el proyecto finalizado, y este sobre la inactividad. Los nombres
// llevan el prefijo "Dominio", convención de las pruebas de dominio del proyecto, para no chocar con
// las pruebas de integración de US7.

// autoCancelNow es el instante fijo de la evaluación, para medir la inactividad sin reloj real.
var autoCancelNow = time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)

// Escenario: Cancelar una sesión sin registrar estimación - Caso límite: la historia pasa a Completada con una sesión abierta (005/US7)
// Con la historia completada y actividad reciente, corresponde cancelar con story_completed, en
// cualquiera de los estados de la historia que la preceden (RN-30).
func TestUS7_DominioHistoriaCompletada(t *testing.T) {
	recent := autoCancelNow.Add(-time.Minute)

	reason, cancel := AutoCancelReason(backlogdomain.StoryStatusCompleted, false, recent, autoCancelNow)
	assert.True(t, cancel)
	assert.Equal(t, CancelReasonStoryCompleted, reason)

	// Las historias que no están completadas no cancelan la sesión por sí solas.
	for _, status := range []backlogdomain.StoryStatus{backlogdomain.StoryStatusPending, backlogdomain.StoryStatusInProgress} {
		t.Run(string(status), func(t *testing.T) {
			_, cancel := AutoCancelReason(status, false, recent, autoCancelNow)
			assert.False(t, cancel)
		})
	}
}

// Escenario: Cancelar una sesión sin registrar estimación - Caso límite: el proyecto pasa a Finalizado con una sesión abierta (005/US7)
// Con el proyecto finalizado, la historia sin completar y actividad reciente, corresponde cancelar
// con project_finished (RN-30).
func TestUS7_DominioProyectoFinalizado(t *testing.T) {
	recent := autoCancelNow.Add(-time.Minute)

	reason, cancel := AutoCancelReason(backlogdomain.StoryStatusInProgress, true, recent, autoCancelNow)
	assert.True(t, cancel)
	assert.Equal(t, CancelReasonProjectFinished, reason)
}

// Escenario: Cancelar una sesión sin registrar estimación - Caso límite: precedencia de los motivos (005/US7)
// Cuando se cumplen varias condiciones, el motivo sigue el orden de la tabla: la historia completada
// gana al proyecto finalizado y a la inactividad, y el proyecto finalizado gana a la inactividad.
func TestUS7_DominioPrecedenciaDeMotivos(t *testing.T) {
	recent := autoCancelNow.Add(-time.Minute)
	stale := autoCancelNow.Add(-2 * time.Hour)

	t.Run("historia completada y proyecto finalizado", func(t *testing.T) {
		reason, cancel := AutoCancelReason(backlogdomain.StoryStatusCompleted, true, recent, autoCancelNow)
		assert.True(t, cancel)
		assert.Equal(t, CancelReasonStoryCompleted, reason)
	})

	t.Run("las tres condiciones", func(t *testing.T) {
		reason, cancel := AutoCancelReason(backlogdomain.StoryStatusCompleted, true, stale, autoCancelNow)
		assert.True(t, cancel)
		assert.Equal(t, CancelReasonStoryCompleted, reason)
	})

	t.Run("proyecto finalizado e inactividad", func(t *testing.T) {
		reason, cancel := AutoCancelReason(backlogdomain.StoryStatusPending, true, stale, autoCancelNow)
		assert.True(t, cancel)
		assert.Equal(t, CancelReasonProjectFinished, reason)
	})
}

// Escenario: Cancelar una sesión sin registrar estimación - Caso límite: 59 minutos sin hechos (005/US7)
// Con el último hecho hace 59:59, la ventana de inactividad no se cumplió y no corresponde cancelar;
// tampoco con un hecho en el mismo instante (RN-31, SC-021).
func TestUS7_DominioInactividadSinCumplir(t *testing.T) {
	for name, elapsed := range map[string]time.Duration{
		"59:59":      59*time.Minute + 59*time.Second,
		"sin demora": 0,
		"59 minutos": 59 * time.Minute,
	} {
		t.Run(name, func(t *testing.T) {
			reason, cancel := AutoCancelReason(backlogdomain.StoryStatusPending, false, autoCancelNow.Add(-elapsed), autoCancelNow)
			assert.False(t, cancel)
			assert.Empty(t, reason)
		})
	}
}

// Escenario: Cancelar una sesión sin registrar estimación - Caso límite: 60 minutos sin hechos (005/US7)
// Con el último hecho hace exactamente 60:00, o más, corresponde cancelar por inactividad: el límite
// es inclusivo (RN-31, SC-021).
func TestUS7_DominioInactividadCumplida(t *testing.T) {
	for name, elapsed := range map[string]time.Duration{
		"60:00":   60 * time.Minute,
		"60:01":   60*time.Minute + time.Second,
		"3 horas": 3 * time.Hour,
	} {
		t.Run(name, func(t *testing.T) {
			reason, cancel := AutoCancelReason(backlogdomain.StoryStatusInProgress, false, autoCancelNow.Add(-elapsed), autoCancelNow)
			assert.True(t, cancel)
			assert.Equal(t, CancelReasonInactivity, reason)
		})
	}
}

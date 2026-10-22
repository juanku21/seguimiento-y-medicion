package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Pruebas unitarias obligatorias del avance de una historia dentro del sprint Activo (T052; RN-11,
// RN-12, FR-033, FR-038; data-model.md sección 5). Recorren la matriz completa de los nueve pares
// origen-destino: solo pending -> in_progress, in_progress -> completed y completed -> in_progress se
// aceptan; los otros seis, incluido el salto directo de pending a completed, la permanencia en el
// mismo estado y todo destino pending, se rechazan con ErrInvalidStoryTransition. Los escenarios de
// US4 se verifican de punta a punta en las pruebas de integración de T055.
func TestValidateStoryProgress(t *testing.T) {
	cases := []struct {
		name    string
		from    backlogdomain.StoryStatus
		to      backlogdomain.StoryStatus
		allowed bool
	}{
		{name: "pending a pending", from: backlogdomain.StoryStatusPending, to: backlogdomain.StoryStatusPending, allowed: false},
		{name: "pending a in_progress", from: backlogdomain.StoryStatusPending, to: backlogdomain.StoryStatusInProgress, allowed: true},
		{name: "pending a completed", from: backlogdomain.StoryStatusPending, to: backlogdomain.StoryStatusCompleted, allowed: false},
		{name: "in_progress a pending", from: backlogdomain.StoryStatusInProgress, to: backlogdomain.StoryStatusPending, allowed: false},
		{name: "in_progress a in_progress", from: backlogdomain.StoryStatusInProgress, to: backlogdomain.StoryStatusInProgress, allowed: false},
		{name: "in_progress a completed", from: backlogdomain.StoryStatusInProgress, to: backlogdomain.StoryStatusCompleted, allowed: true},
		{name: "completed a pending", from: backlogdomain.StoryStatusCompleted, to: backlogdomain.StoryStatusPending, allowed: false},
		{name: "completed a in_progress", from: backlogdomain.StoryStatusCompleted, to: backlogdomain.StoryStatusInProgress, allowed: true},
		{name: "completed a completed", from: backlogdomain.StoryStatusCompleted, to: backlogdomain.StoryStatusCompleted, allowed: false},
	}

	// Cada par se prueba en un subtest propio: los admitidos no devuelven error y los demás devuelven
	// el centinela de transición de historia inválida, que delivery traduce a la respuesta 409.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStoryProgress(tc.from, tc.to)
			if tc.allowed {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrInvalidStoryTransition)
		})
	}
}

// Pruebas unitarias obligatorias del resultado congelado de una historia al cerrar el sprint (T062;
// RN-15, FR-043; data-model.md sección 6). Solo una historia Completada queda completed; pending e
// in_progress quedan not_completed. Los totales de la instantánea se calculan en SQL y se verifican
// en las pruebas de integración de T064.
func TestOutcome(t *testing.T) {
	cases := []struct {
		name   string
		status backlogdomain.StoryStatus
		want   StoryOutcome
	}{
		{name: "completed queda completed", status: backlogdomain.StoryStatusCompleted, want: StoryOutcomeCompleted},
		{name: "pending queda not_completed", status: backlogdomain.StoryStatusPending, want: StoryOutcomeNotCompleted},
		{name: "in_progress queda not_completed", status: backlogdomain.StoryStatusInProgress, want: StoryOutcomeNotCompleted},
	}

	// Cada estado se prueba en un subtest propio y se compara con el resultado esperado al cierre.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Outcome(tc.status))
		})
	}
}

package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas unitarias obligatorias de ResolveAnchor (T013, RN-07, RN-24, RN-25) con la tabla de
// data-model.md sección 6.1: un compromiso abierto ancla a su sprint; sin él, el último compromiso
// congelado ancla a su sprint mientras dure el plazo de gracia, y vencido el plazo la historia
// completada rechaza el alta y la no completada queda sin sprint. El sprint cierra un viernes a las
// 18:00 con un plazo de 48 h, de modo que el plazo vence el domingo a las 18:00 exactas.

// anchorGrace, anchorClosedAt y los instantes de frontera son fijos para no depender del reloj real.
var (
	anchorGrace      = 48 * time.Hour
	anchorClosedAt   = time.Date(2026, time.October, 23, 18, 0, 0, 0, time.UTC)
	anchorSunday1759 = time.Date(2026, time.October, 25, 17, 59, 0, 0, time.UTC)
	anchorSunday1800 = time.Date(2026, time.October, 25, 18, 0, 0, 0, time.UTC)
	anchorSunday1801 = time.Date(2026, time.October, 25, 18, 1, 0, 0, time.UTC)
)

// closedCommitment construye el último compromiso congelado con el cierre fijo y el resultado dado.
func closedCommitment(sprintID uuid.UUID, outcome sprintsdomain.StoryOutcome) *ClosedCommitment {
	return &ClosedCommitment{SprintID: sprintID, ClosedAt: anchorClosedAt, Outcome: outcome}
}

// Con un compromiso abierto, el alta se ancla a su sprint aunque exista un compromiso cerrado en plazo:
// la historia fue recomprometida y manda el sprint Planificado o Activo (006/US1, RN-07).
func TestResolveAnchor_OpenCommitment(t *testing.T) {
	tests := []struct {
		name       string
		lastClosed func() *ClosedCommitment
		now        time.Time
	}{
		{name: "sin compromiso cerrado", lastClosed: func() *ClosedCommitment { return nil }, now: anchorSunday1759},
		{name: "recomprometida con cerrado en plazo", lastClosed: func() *ClosedCommitment {
			return closedCommitment(uuid.New(), sprintsdomain.StoryOutcomeNotCompleted)
		}, now: anchorSunday1759},
		{name: "con cerrado completed vencido", lastClosed: func() *ClosedCommitment {
			return closedCommitment(uuid.New(), sprintsdomain.StoryOutcomeCompleted)
		}, now: anchorSunday1801},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openSprint := uuid.New()

			got, err := ResolveAnchor(&OpenCommitment{SprintID: openSprint}, tt.lastClosed(), tt.now, anchorGrace)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, openSprint, *got)
		})
	}
}

// Sin compromiso abierto y dentro del plazo, el alta se ancla al sprint cerrado tanto si la historia
// quedó completada como si volvió al backlog sin completarse (006/US1, RN-24).
func TestResolveAnchor_ClosedWithinGrace(t *testing.T) {
	tests := []struct {
		name    string
		outcome sprintsdomain.StoryOutcome
	}{
		{name: "completed", outcome: sprintsdomain.StoryOutcomeCompleted},
		{name: "not_completed", outcome: sprintsdomain.StoryOutcomeNotCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closedSprint := uuid.New()

			got, err := ResolveAnchor(nil, closedCommitment(closedSprint, tt.outcome), anchorSunday1759, anchorGrace)

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, closedSprint, *got)
		})
	}
}

// Vencido el plazo, la historia completada rechaza el alta con ErrSprintFinalized; el vencimiento
// incluye el instante exacto de cierre más el plazo (006/US1, RN-25).
func TestResolveAnchor_CompletedGraceExpired(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
	}{
		{name: "domingo 18:00 exacto", now: anchorSunday1800},
		{name: "domingo 18:01", now: anchorSunday1801},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAnchor(nil, closedCommitment(uuid.New(), sprintsdomain.StoryOutcomeCompleted), tt.now, anchorGrace)

			assert.ErrorIs(t, err, ErrSprintFinalized)
			assert.Nil(t, got)
		})
	}
}

// Vencido el plazo, la historia devuelta al backlog acepta el alta sin sprint asociado
// (006/US1, RN-25).
func TestResolveAnchor_NotCompletedGraceExpired(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
	}{
		{name: "domingo 18:00 exacto", now: anchorSunday1800},
		{name: "domingo 18:01", now: anchorSunday1801},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAnchor(nil, closedCommitment(uuid.New(), sprintsdomain.StoryOutcomeNotCompleted), tt.now, anchorGrace)

			assert.NoError(t, err)
			assert.Nil(t, got)
		})
	}
}

// Sin ningún compromiso, la historia está solo en el backlog y el alta queda sin sprint
// (006/US1, RN-07).
func TestResolveAnchor_NoCommitments(t *testing.T) {
	got, err := ResolveAnchor(nil, nil, anchorSunday1759, anchorGrace)

	assert.NoError(t, err)
	assert.Nil(t, got)
}

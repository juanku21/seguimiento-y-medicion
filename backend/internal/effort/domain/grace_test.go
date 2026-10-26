package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas unitarias obligatorias de GraceOpen (T014, RN-23) con la regla de data-model.md sección
// 6.2: el plazo de gracia está vigente mientras now < closedAt + grace y se cuenta cumplido al
// alcanzarse. Se usa un cierre fijo el viernes a las 18:00 con el plazo por defecto de 48 h.
var (
	graceClosedAt = time.Date(2026, time.October, 23, 18, 0, 0, 0, time.UTC)
	defaultGrace  = 48 * time.Hour
)

// Justo después del cierre el plazo de gracia está vigente (006/US1, RN-23).
func TestGraceOpen_JustAfterClose(t *testing.T) {
	assert.True(t, GraceOpen(graceClosedAt, graceClosedAt.Add(time.Nanosecond), defaultGrace))
}

// Un nanosegundo antes de cumplirse el plazo sigue vigente (006/US1, RN-23).
func TestGraceOpen_OneNanosecondBeforeDeadline(t *testing.T) {
	now := graceClosedAt.Add(defaultGrace - time.Nanosecond)

	assert.True(t, GraceOpen(graceClosedAt, now, defaultGrace))
}

// Al alcanzarse closedAt + grace el plazo se cuenta cumplido y ya no está vigente (006/US1, RN-23).
func TestGraceOpen_AtDeadline(t *testing.T) {
	now := graceClosedAt.Add(defaultGrace)

	assert.False(t, GraceOpen(graceClosedAt, now, defaultGrace))
}

// Después de cumplido el plazo deja de estar vigente (006/US1, RN-23).
func TestGraceOpen_AfterDeadline(t *testing.T) {
	now := graceClosedAt.Add(defaultGrace + time.Minute)

	assert.False(t, GraceOpen(graceClosedAt, now, defaultGrace))
}

// Pruebas unitarias obligatorias de EnsureEntryWritable (T040, RN-09 y RN-10) con la tabla de
// data-model.md sección 6.2: sin sprint, Planificado o Activo el registro es escribible; Cerrado lo es
// solo mientras el plazo de gracia siga vigente y, cumplido, devuelve ErrEffortFinalized. Se reutiliza
// el cierre fijo y el plazo de 48 h de las pruebas de GraceOpen.
func closedSprint() *AnchoredSprint {
	closedAt := graceClosedAt
	return &AnchoredSprint{ID: uuid.New(), Name: "Sprint 1", Status: sprintsdomain.SprintStatusClosed, ClosedAt: &closedAt}
}

// Un registro sin sprint asociado es escribible (006/US3, RN-10).
func TestEnsureEntryWritable_WithoutSprint(t *testing.T) {
	assert.NoError(t, EnsureEntryWritable(nil, graceClosedAt.Add(30*24*time.Hour), defaultGrace))
}

// Un registro de un sprint Planificado o Activo es escribible (006/US3, RN-09).
func TestEnsureEntryWritable_OpenSprint(t *testing.T) {
	for _, status := range []sprintsdomain.SprintStatus{sprintsdomain.SprintStatusPlanned, sprintsdomain.SprintStatusActive} {
		t.Run(string(status), func(t *testing.T) {
			sprint := &AnchoredSprint{ID: uuid.New(), Name: "Sprint 2", Status: status}

			assert.NoError(t, EnsureEntryWritable(sprint, graceClosedAt, defaultGrace))
		})
	}
}

// Un registro de un sprint cerrado hace 10 h, con plazo de 48 h, sigue escribible (006/US3, RN-09).
func TestEnsureEntryWritable_ClosedWithinGrace(t *testing.T) {
	now := graceClosedAt.Add(10 * time.Hour)

	assert.NoError(t, EnsureEntryWritable(closedSprint(), now, defaultGrace))
}

// Al cumplirse exactamente las 48 h del cierre el registro es definitivo (006/US3, RN-09).
func TestEnsureEntryWritable_ClosedAtDeadline(t *testing.T) {
	now := graceClosedAt.Add(defaultGrace)

	assert.ErrorIs(t, EnsureEntryWritable(closedSprint(), now, defaultGrace), ErrEffortFinalized)
}

// Cinco días después del cierre el registro es definitivo (006/US3, RN-09).
func TestEnsureEntryWritable_ClosedLongAgo(t *testing.T) {
	now := graceClosedAt.Add(5 * 24 * time.Hour)

	assert.ErrorIs(t, EnsureEntryWritable(closedSprint(), now, defaultGrace), ErrEffortFinalized)
}

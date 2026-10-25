package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

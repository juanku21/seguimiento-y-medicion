package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	defectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// allSeverities son las cuatro severidades que siempre deben aparecer en cada desglose.
var allSeverities = []defectsdomain.DefectSeverity{
	defectsdomain.DefectSeverityCritical,
	defectsdomain.DefectSeverityHigh,
	defectsdomain.DefectSeverityMedium,
	defectsdomain.DefectSeverityLow,
}

// assertTally verifica el total y el desglose completo (severidades ausentes en 0) de un conteo.
func assertTally(t *testing.T, tally domain.DefectTally, total int, by map[defectsdomain.DefectSeverity]int) {
	t.Helper()
	assert.Equal(t, total, tally.Total)
	sum := 0
	for _, sev := range allSeverities {
		got, ok := tally.BySeverity[sev]
		assert.True(t, ok, "falta la severidad %s", sev)
		assert.Equal(t, by[sev], got, "severidad %s", sev)
		sum += got
	}
	assert.Len(t, tally.BySeverity, 4)
	assert.Equal(t, tally.Total, sum, "la suma del desglose debe igualar el total")
}

// Escenario: Defectos del proyecto por estado y severidad (008/US5)
func TestUS5_TallyDefects(t *testing.T) {
	c := defectsdomain.DefectSeverityCritical
	h := defectsdomain.DefectSeverityHigh
	m := defectsdomain.DefectSeverityMedium
	l := defectsdomain.DefectSeverityLow

	t.Run("detectados, resueltos y abiertos por estado", func(t *testing.T) {
		got := domain.TallyDefects([]domain.DefectCount{
			{Status: defectsdomain.DefectStatusOpen, Severity: c, Count: 2},
			{Status: defectsdomain.DefectStatusInProgress, Severity: h, Count: 3},
			{Status: defectsdomain.DefectStatusResolved, Severity: c, Count: 1},
			{Status: defectsdomain.DefectStatusResolved, Severity: l, Count: 4},
		})
		assertTally(t, got.Detected, 10, map[defectsdomain.DefectSeverity]int{c: 3, h: 3, l: 4})
		assertTally(t, got.Resolved, 5, map[defectsdomain.DefectSeverity]int{c: 1, l: 4})
		assertTally(t, got.Open, 5, map[defectsdomain.DefectSeverity]int{c: 2, h: 3})
		assert.Equal(t, got.Resolved.Total+got.Open.Total, got.Detected.Total)
	})

	t.Run("una fila discarded se ignora", func(t *testing.T) {
		got := domain.TallyDefects([]domain.DefectCount{
			{Status: defectsdomain.DefectStatusOpen, Severity: m, Count: 1},
			{Status: defectsdomain.DefectStatusDiscarded, Severity: m, Count: 3},
		})
		assertTally(t, got.Detected, 1, map[defectsdomain.DefectSeverity]int{m: 1})
		assertTally(t, got.Resolved, 0, nil)
		assertTally(t, got.Open, 1, map[defectsdomain.DefectSeverity]int{m: 1})
	})

	t.Run("sin filas los tres conteos quedan en 0 con las cuatro severidades", func(t *testing.T) {
		got := domain.TallyDefects(nil)
		assertTally(t, got.Detected, 0, nil)
		assertTally(t, got.Resolved, 0, nil)
		assertTally(t, got.Open, 0, nil)
	})

	t.Run("proyecto de la prueba independiente: 12 detectados, 5 resueltos, 7 abiertos", func(t *testing.T) {
		got := domain.TallyDefects([]domain.DefectCount{
			{Status: defectsdomain.DefectStatusOpen, Severity: c, Count: 2},
			{Status: defectsdomain.DefectStatusInProgress, Severity: h, Count: 3},
			{Status: defectsdomain.DefectStatusOpen, Severity: m, Count: 2},
			{Status: defectsdomain.DefectStatusResolved, Severity: c, Count: 1},
			{Status: defectsdomain.DefectStatusResolved, Severity: h, Count: 1},
			{Status: defectsdomain.DefectStatusResolved, Severity: m, Count: 1},
			{Status: defectsdomain.DefectStatusResolved, Severity: l, Count: 2},
			{Status: defectsdomain.DefectStatusDiscarded, Severity: l, Count: 3},
		})
		assertTally(t, got.Detected, 12, map[defectsdomain.DefectSeverity]int{c: 3, h: 4, m: 3, l: 2})
		assertTally(t, got.Resolved, 5, map[defectsdomain.DefectSeverity]int{c: 1, h: 1, m: 1, l: 2})
		assertTally(t, got.Open, 7, map[defectsdomain.DefectSeverity]int{c: 2, h: 3, m: 2})
	})
}

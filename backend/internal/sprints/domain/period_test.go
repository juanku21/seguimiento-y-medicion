package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de PeriodsOverlap (T010; RN-05, SC-004; data-model.md sección 6).
// No llevan la etiqueta de un escenario de la spec: los escenarios de superposición se cubren con
// las pruebas de integración TestUS1_FechasContiguas y TestUS1_Superposicion.

// day construye una fecha de octubre de 2026 a medianoche UTC, como se representan las fechas de
// los sprints.
func day(d int) time.Time {
	return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC)
}

// period construye un período de octubre de 2026 a partir del día de inicio y el de fin.
func period(start, end int) Period {
	return Period{Start: day(start), End: day(end)}
}

// Cada caso compara dos períodos en el orden dado; los casos simétricos se declaran en los dos
// órdenes para verificar que el resultado no depende de cuál se pasa primero.
func TestPeriodsOverlap(t *testing.T) {
	cases := []struct {
		name string
		a    Period
		b    Period
		want bool
	}{
		{"comparten solo el día de borde (anterior primero)", period(1, 14), period(14, 28), false},
		{"comparten solo el día de borde (posterior primero)", period(14, 28), period(1, 14), false},
		{"un día de solape (anterior primero)", period(1, 14), period(13, 20), true},
		{"un día de solape (posterior primero)", period(13, 20), period(1, 14), true},
		{"período idéntico", period(1, 14), period(1, 14), true},
		{"el primero contiene al segundo", period(1, 28), period(5, 10), true},
		{"el segundo contiene al primero", period(5, 10), period(1, 28), true},
		{"disjuntos con un día de separación (anterior primero)", period(1, 14), period(15, 28), false},
		{"disjuntos con un día de separación (posterior primero)", period(15, 28), period(1, 14), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PeriodsOverlap(tc.a, tc.b),
				"PeriodsOverlap(%s→%s, %s→%s)",
				tc.a.Start.Format(time.DateOnly), tc.a.End.Format(time.DateOnly),
				tc.b.Start.Format(time.DateOnly), tc.b.End.Format(time.DateOnly))
		})
	}
}

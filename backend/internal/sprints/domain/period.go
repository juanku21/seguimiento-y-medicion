package domain

import "time"

// Period es el intervalo de fechas de un sprint, con inicio y fin a medianoche UTC (research.md
// sección 4).
type Period struct {
	Start time.Time
	End   time.Time
}

// PeriodsOverlap indica si dos períodos se superponen (FR-005, RN-05). Firma mínima para la prueba
// de T010: la regla se implementa en T015.
func PeriodsOverlap(a, b Period) bool {
	return false
}

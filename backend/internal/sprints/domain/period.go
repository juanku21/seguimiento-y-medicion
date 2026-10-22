package domain

import "time"

// Period es el intervalo de fechas de un sprint, con inicio y fin a medianoche UTC (research.md
// sección 4).
type Period struct {
	Start time.Time
	End   time.Time
}

// PeriodsOverlap indica si dos períodos se superponen (FR-005, RN-05). Con desigualdades estrictas,
// compartir solo el día de borde no es superposición; la contención total y el período idéntico sí
// lo son.
func PeriodsOverlap(a, b Period) bool {
	return a.Start.Before(b.End) && b.Start.Before(a.End)
}

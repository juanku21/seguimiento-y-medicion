package domain

import "time"

// Schedule calcula los días restantes de un sprint Activo hasta su fecha de fin, con el día de fin
// incluido, o los días que lleva vencido; exactamente uno de los dos valores es distinto de nil y
// ninguno es negativo (FR-053, RN-23; research.md sección 15). Firma mínima de T071: la
// implementación corresponde a T076.
func Schedule(endDate, now time.Time, location *time.Location) (remainingDays, overdueDays *int) {
	return nil, nil
}

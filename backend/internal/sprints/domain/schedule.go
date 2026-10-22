package domain

import "time"

// Schedule calcula los días restantes de un sprint Activo hasta su fecha de fin, con el día de fin
// incluido, o los días que lleva vencido; exactamente uno de los dos valores es distinto de nil y
// ninguno es negativo (FR-053, RN-23; research.md sección 15).
func Schedule(endDate, now time.Time, location *time.Location) (remainingDays, overdueDays *int) {
	// Se comparan fechas de calendario: la de fin tal como se persiste y la de hoy en la zona del
	// sistema, ambas llevadas a medianoche UTC para que la resta dé días enteros.
	endYear, endMonth, endDay := endDate.Date()
	end := time.Date(endYear, endMonth, endDay, 0, 0, 0, 0, time.UTC)
	todayYear, todayMonth, todayDay := now.In(location).Date()
	today := time.Date(todayYear, todayMonth, todayDay, 0, 0, 0, 0, time.UTC)

	// Una diferencia negativa se informa como días de vencimiento, nunca como número negativo.
	days := int(end.Sub(today).Hours() / 24)
	if days < 0 {
		overdue := -days
		return nil, &overdue
	}
	return &days, nil
}

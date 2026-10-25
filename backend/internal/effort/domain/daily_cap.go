package domain

// Límites de las horas de esfuerzo, expresados en centésimas de hora: el tope diario por persona
// (24 h entre todos sus registros del mismo día), el mínimo y el máximo de un registro (0,25 h y 24 h)
// y el paso válido de un registro (múltiplos de 0,25 h) (data-model.md secciones 1 y 6.4).
const (
	MaxDailyHundredths  = 2400
	MinEntryHundredths  = 25
	MaxEntryHundredths  = 2400
	HoursStepHundredths = 25
)

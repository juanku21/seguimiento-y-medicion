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

// CheckDailyCap verifica que las centésimas ya registradas por la persona en la fecha más las
// solicitadas no superen el tope diario (RN-04, data-model.md sección 6.4). Si lo superan, devuelve
// DailyCapExceededError con las centésimas todavía disponibles, que nunca son negativas aunque lo ya
// registrado exceda el tope.
func CheckDailyCap(alreadyLogged, requested int) error {
	if alreadyLogged+requested <= MaxDailyHundredths {
		return nil
	}
	return &DailyCapExceededError{AvailableHundredths: max(MaxDailyHundredths-alreadyLogged, 0)}
}

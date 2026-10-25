package domain

import "errors"

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
// solicitadas no superen el tope diario (RN-04, data-model.md sección 6.4). Firma mínima de T012:
// la implementación corresponde a T022.
func CheckDailyCap(alreadyLogged, requested int) error {
	return errors.New("not implemented")
}

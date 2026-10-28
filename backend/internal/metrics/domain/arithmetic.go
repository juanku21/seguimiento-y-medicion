package domain

import effortdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"

// Percent calcula un porcentaje en centésimas de punto porcentual (FR-033). Con denominador 0 no es
// calculable; si no, delega en el redondeo único de la spec 006 para que ambas features coincidan
// (RC-13), sin ningún otro redondeo.
func Percent(numerator, denominator int) Metric {
	if denominator == 0 {
		return NotCalculable()
	}
	return Available(effortdomain.RoundPercentHundredths(numerator, denominator))
}

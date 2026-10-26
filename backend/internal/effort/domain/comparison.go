package domain

// Comparison es la comparación entre el esfuerzo estimado y el real de una historia, en centésimas de
// hora y de punto porcentual. Un puntero nil representa "no calculable" (research.md sección 12).
type Comparison struct {
	StoryPoints                *int
	EstimatedHundredths        *int
	ActualHundredths           int
	DeviationHundredths        *int
	DeviationPercentHundredths *int
}

// Compare calcula la comparación entre lo estimado y lo real de una historia (FR-046 a FR-051).
func Compare(storyPoints *int, factorHundredths, actualHundredths int) Comparison {
	// Sin Story Points la estimada y ambas desviaciones quedan como "no calculable" (RN-17).
	result := Comparison{StoryPoints: storyPoints, ActualHundredths: actualHundredths}
	if storyPoints == nil {
		return result
	}

	// Estimada = puntos × factor y diferencia = real − estimada; el porcentaje solo se calcula con
	// estimada mayor que 0 (RN-16, RN-18 y RN-19).
	estimated := *storyPoints * factorHundredths
	deviation := actualHundredths - estimated
	result.EstimatedHundredths = &estimated
	result.DeviationHundredths = &deviation
	if estimated != 0 {
		percent := RoundPercentHundredths(deviation, estimated)
		result.DeviationPercentHundredths = &percent
	}
	return result
}

// RoundPercentHundredths devuelve numerator × 10000 / denominator redondeado mitad lejos de cero
// (RN-26). La spec 008 debe reutilizar esta función para su desviación porcentual, de modo que ambas
// features redondeen igual (RC-13). El denominador no puede ser 0.
func RoundPercentHundredths(numerator, denominator int) int {
	// Se trabaja con valores absolutos y se recupera el signo al final, para que el empate se resuelva
	// lejos de cero en ambos sentidos.
	negative := (numerator < 0) != (denominator < 0)
	if numerator < 0 {
		numerator = -numerator
	}
	if denominator < 0 {
		denominator = -denominator
	}

	// Un único redondeo entero con cociente y resto: si el resto es al menos la mitad del
	// denominador, el cociente sube una unidad.
	product := numerator * 10000
	quotient := product / denominator
	if 2*(product%denominator) >= denominator {
		quotient++
	}
	if negative {
		return -quotient
	}
	return quotient
}

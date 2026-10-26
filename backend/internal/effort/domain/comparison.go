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
	return Comparison{}
}

// RoundPercentHundredths devuelve numerator × 10000 / denominator redondeado mitad lejos de cero
// (RN-26).
func RoundPercentHundredths(numerator, denominator int) int {
	return 0
}

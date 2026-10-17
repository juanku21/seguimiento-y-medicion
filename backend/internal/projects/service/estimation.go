package service

// EstimatedHoursHundredths devuelve las horas estimadas del proyecto en centésimas de hora como el
// producto entero de los Story Points estimados por el factor en centésimas, sin divisiones
// intermedias (RN-08, FR-045). La división por 100 ocurre una sola vez, al serializar la respuesta.
func EstimatedHoursHundredths(estimatedStoryPoints, hoursPerStoryPointHundredths int) int {
	return estimatedStoryPoints * hoursPerStoryPointHundredths
}

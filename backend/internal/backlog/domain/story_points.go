package domain

import "slices"

// StoryPointsScale es la escala cerrada de Story Points (RN-06), declarada una sola vez y en orden
// ascendente para que la validación pueda enumerarla en sus mensajes. El 0 es una estimación válida
// y no la ausencia de ella (RN-07).
var StoryPointsScale = []int{0, 1, 2, 3, 5, 8, 13, 20, 40, 100}

// IsValidStoryPoints indica si un valor pertenece a la escala cerrada de Story Points (RN-06).
func IsValidStoryPoints(value int) bool {
	return slices.Contains(StoryPointsScale, value)
}

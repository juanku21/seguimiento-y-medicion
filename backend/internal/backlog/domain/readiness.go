package domain

// Readiness indica si una historia está lista para planificar y qué condiciones le faltan
// (RN-12, FR-028, FR-029). Es la única fuente de la regla: falta "story_points" si la historia no
// está estimada (el 0 es una estimación válida) y "acceptance_criteria" si no tiene criterios, en
// ese orden. La lista arranca vacía y no nula para que se serialice como [] en JSON.
func Readiness(storyPoints *int, criteriaCount int) (bool, []string) {
	missing := []string{}
	if storyPoints == nil {
		missing = append(missing, "story_points")
	}
	if criteriaCount == 0 {
		missing = append(missing, "acceptance_criteria")
	}
	return len(missing) == 0, missing
}

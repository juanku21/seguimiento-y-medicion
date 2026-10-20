package domain

// Readiness indica si una historia está lista para planificar y qué condiciones le faltan
// (RN-12, FR-028, FR-029). Firma mínima para las pruebas de T012; la implementación corresponde a T018.
func Readiness(storyPoints *int, criteriaCount int) (bool, []string) {
	return false, nil
}

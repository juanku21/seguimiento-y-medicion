package domain

// DefectSeverity es la severidad de un defecto; se guarda como texto en la columna defects.severity.
// Los valores están en inglés y la etiqueta en español vive en el frontend.
type DefectSeverity string

// Severidades posibles de un defecto: critical (Crítica), high (Alta), medium (Media) y low (Baja).
// La validación y el orden de severidad se implementan en las historias que los usan.
const (
	DefectSeverityCritical DefectSeverity = "critical"
	DefectSeverityHigh     DefectSeverity = "high"
	DefectSeverityMedium   DefectSeverity = "medium"
	DefectSeverityLow      DefectSeverity = "low"
)

// Valid indica si la severidad es una de las cuatro admitidas (RN-04).
func (s DefectSeverity) Valid() bool {
	// Solo las cuatro constantes definidas son válidas; cualquier otro valor se rechaza.
	switch s {
	case DefectSeverityCritical, DefectSeverityHigh, DefectSeverityMedium, DefectSeverityLow:
		return true
	default:
		return false
	}
}

// SeverityRank devuelve el orden de severidad: critical 0, high 1, medium 2, low 3.
func SeverityRank(s DefectSeverity) int {
	// Mapea cada severidad a su posición; el valor inválido conserva el -1 previo (no definido por la spec).
	switch s {
	case DefectSeverityCritical:
		return 0
	case DefectSeverityHigh:
		return 1
	case DefectSeverityMedium:
		return 2
	case DefectSeverityLow:
		return 3
	default:
		return -1
	}
}

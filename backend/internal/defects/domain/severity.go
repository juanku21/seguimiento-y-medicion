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

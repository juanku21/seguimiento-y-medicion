package domain

// StoryPriority es la prioridad de una historia de usuario; se guarda como texto en la columna
// user_stories.priority y no como enum de PostgreSQL (data-model.md sección 3). Los valores están en
// inglés y la etiqueta en español vive en el frontend.
type StoryPriority string

// Prioridades posibles de una historia: high (Alta), medium (Media) y low (Baja). El rango de orden
// del listado es regla de negocio y no se declara acá.
const (
	StoryPriorityHigh   StoryPriority = "high"
	StoryPriorityMedium StoryPriority = "medium"
	StoryPriorityLow    StoryPriority = "low"
)

// Rank devuelve el rango de orden de la prioridad en el listado: high 0, medium 1 y low 2
// (data-model.md sección 3; RN-17).
func (p StoryPriority) Rank() int {
	return 0
}

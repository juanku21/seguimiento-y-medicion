package domain

// StoryPriority es la prioridad de una historia de usuario; se guarda como texto en la columna
// user_stories.priority y no como enum de PostgreSQL (data-model.md sección 3). Los valores están en
// inglés y la etiqueta en español vive en el frontend.
type StoryPriority string

// Prioridades posibles de una historia: high (Alta), medium (Media) y low (Baja). Su rango de orden
// en el listado lo define Rank.
const (
	StoryPriorityHigh   StoryPriority = "high"
	StoryPriorityMedium StoryPriority = "medium"
	StoryPriorityLow    StoryPriority = "low"
)

// Rank devuelve el rango de orden de la prioridad en el listado: high 0, medium 1 y low 2
// (data-model.md sección 3; RN-17). La prioridad llega ya validada, así que low es el caso restante.
func (p StoryPriority) Rank() int {
	switch p {
	case StoryPriorityHigh:
		return 0
	case StoryPriorityMedium:
		return 1
	default:
		return 2
	}
}

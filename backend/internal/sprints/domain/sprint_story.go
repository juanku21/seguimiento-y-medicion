package domain

// StoryOutcome es el resultado de una historia en un sprint; se guarda como texto en la columna
// sprint_stories.outcome y no como enum de PostgreSQL (data-model.md sección 2). Queda NULL mientras
// el sprint está abierto y se completa una sola vez, al cerrarlo (FR-042, FR-043).
type StoryOutcome string

// Resultados posibles al cerrar el sprint: completed si la historia estaba Completada y
// not_completed en cualquier otro caso (FR-043).
const (
	StoryOutcomeCompleted    StoryOutcome = "completed"
	StoryOutcomeNotCompleted StoryOutcome = "not_completed"
)

package domain

// StoryStatus es el estado de una historia de usuario; se guarda como texto en la columna
// user_stories.status y no como enum de PostgreSQL (data-model.md sección 4). Los valores están en
// inglés y la etiqueta en español vive en el frontend.
type StoryStatus string

// Estados posibles de una historia: pending (Pendiente), in_progress (En progreso) y completed
// (Completada). Toda historia nace pending y esta feature nunca escribe el estado después de la
// creación: solo lo lee. Sus transiciones las produce specs/004-sprint-management (FR-027, RC-04).
const (
	StoryStatusPending    StoryStatus = "pending"
	StoryStatusInProgress StoryStatus = "in_progress"
	StoryStatusCompleted  StoryStatus = "completed"
)

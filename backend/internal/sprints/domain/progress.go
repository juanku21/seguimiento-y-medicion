package domain

import (
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// ValidateStoryProgress decide si una historia del sprint Activo puede pasar del estado from al
// estado to (data-model.md sección 5; FR-033, FR-038). Solo se admiten pending -> in_progress,
// in_progress -> completed y completed -> in_progress; cualquier otro par, incluidos el salto directo
// a completed, la permanencia en el mismo estado y el regreso a pending, devuelve
// ErrInvalidStoryTransition.
func ValidateStoryProgress(from, to backlogdomain.StoryStatus) error {
	switch {
	case from == backlogdomain.StoryStatusPending && to == backlogdomain.StoryStatusInProgress,
		from == backlogdomain.StoryStatusInProgress && to == backlogdomain.StoryStatusCompleted,
		from == backlogdomain.StoryStatusCompleted && to == backlogdomain.StoryStatusInProgress:
		return nil
	default:
		return ErrInvalidStoryTransition
	}
}

package domain

import (
	"context"

	"github.com/google/uuid"
)

// SprintEffort es el puerto hacia el registro de esfuerzo: indica si el sprint tiene esfuerzo
// registrado, lo que impide eliminarlo (FR-048, RN-20). El dato lo producirá
// specs/006-effort-tracking; hasta entonces la implementación de sprints/repository informa que
// ningún sprint tiene esfuerzo (research.md sección 16).
type SprintEffort interface {
	HasEffort(ctx context.Context, sprintID uuid.UUID) (bool, error)
}

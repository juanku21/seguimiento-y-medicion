package domain

import (
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// OpenCommitment es el compromiso abierto de una historia (fila de sprint_stories con outcome NULL),
// cuyo sprint está Planificado o Activo; si existe, el alta del registro se ancla a su sprint
// (data-model.md secciones 6.1 y 8).
type OpenCommitment struct {
	SprintID uuid.UUID
}

// ClosedCommitment es el último compromiso congelado de una historia: el de cierre más reciente.
// ClosedAt es el instante de cierre del sprint, desde el que corre el plazo de gracia, y Outcome es el
// resultado congelado de la historia en ese sprint (data-model.md secciones 6.1 y 8).
type ClosedCommitment struct {
	SprintID uuid.UUID
	ClosedAt time.Time
	Outcome  sprintsdomain.StoryOutcome
}

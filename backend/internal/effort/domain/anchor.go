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

// ResolveAnchor decide a qué sprint se ancla el alta de un registro de esfuerzo con la tabla de
// data-model.md sección 6.1 (FR-014, FR-016, RN-07, RN-24, RN-25): un compromiso abierto manda
// siempre; sin él, el último compromiso cerrado ancla a su sprint mientras su plazo de gracia siga
// vigente (GraceOpen). Vencido el plazo, la historia completada rechaza el alta con
// ErrSprintFinalized y la no completada queda sin sprint, igual que la historia sin compromisos.
func ResolveAnchor(open *OpenCommitment, lastClosed *ClosedCommitment, now time.Time, grace time.Duration) (*uuid.UUID, error) {
	if open != nil {
		sprintID := open.SprintID
		return &sprintID, nil
	}
	if lastClosed == nil {
		return nil, nil
	}
	if GraceOpen(lastClosed.ClosedAt, now, grace) {
		sprintID := lastClosed.SprintID
		return &sprintID, nil
	}
	if lastClosed.Outcome == sprintsdomain.StoryOutcomeCompleted {
		return nil, ErrSprintFinalized
	}
	return nil, nil
}

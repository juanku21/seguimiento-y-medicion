package domain

import (
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// DefectStatus es el estado de un defecto; se guarda como texto en la columna defects.status.
type DefectStatus string

// Estados posibles de un defecto: open (Abierto), in_progress (En progreso), resolved (Resuelto) y
// discarded (Descartado). Las transiciones admitidas se implementan en las historias que las usan.
const (
	DefectStatusOpen       DefectStatus = "open"
	DefectStatusInProgress DefectStatus = "in_progress"
	DefectStatusResolved   DefectStatus = "resolved"
	DefectStatusDiscarded  DefectStatus = "discarded"
)

// SprintAnchor es la vista de lectura de un sprint que el módulo de defectos necesita para anclar
// un defecto: su identificador, su estado y su fecha de inicio.
type SprintAnchor struct {
	ID        uuid.UUID
	Status    sprintsdomain.SprintStatus
	StartDate time.Time
}

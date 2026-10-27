package domain

import (
	"errors"
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

// Valid indica si el estado es uno de los cuatro admitidos (RN-11).
func (s DefectStatus) Valid() bool {
	return false
}

// AllowedTransitions devuelve los estados de destino admitidos desde current (RN-12).
func AllowedTransitions(current DefectStatus) []DefectStatus {
	return nil
}

// PlanTransition decide el resultado de pasar un defecto de current a to: devuelve el sprint de
// resolución que debe quedar registrado, o nil, y el error de dominio si la transición se rechaza
// (RN-12 a RN-14). Nunca devuelve ni modifica la historia ni el sprint de detección (FR-027).
func PlanTransition(current, to DefectStatus, detected SprintAnchor, resolution *SprintAnchor) (*uuid.UUID, error) {
	return nil, errors.New("not implemented")
}

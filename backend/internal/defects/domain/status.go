package domain

import (
	"slices"
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

// allowedDefectTransitions es la fuente única de las transiciones admitidas: cada estado de origen
// lista sus destinos posibles (RN-12). Un estado sin entrada no admite ninguna transición.
var allowedDefectTransitions = map[DefectStatus][]DefectStatus{
	DefectStatusOpen:       {DefectStatusInProgress},
	DefectStatusInProgress: {DefectStatusResolved},
}

// Valid indica si el estado es uno de los cuatro admitidos (RN-11).
func (s DefectStatus) Valid() bool {
	switch s {
	case DefectStatusOpen, DefectStatusInProgress, DefectStatusResolved, DefectStatusDiscarded:
		return true
	}
	return false
}

// AllowedTransitions devuelve los estados de destino admitidos desde current (RN-12). Devuelve una
// copia para que quien la recibe no pueda alterar el mapa de transiciones.
func AllowedTransitions(current DefectStatus) []DefectStatus {
	return slices.Clone(allowedDefectTransitions[current])
}

// PlanTransition decide el resultado de pasar un defecto de current a to: devuelve el sprint de
// resolución que debe quedar registrado, o nil, y el error de dominio si la transición se rechaza
// (RN-12 a RN-14). Nunca devuelve ni modifica la historia ni el sprint de detección (FR-027).
func PlanTransition(current, to DefectStatus, detected SprintAnchor, resolution *SprintAnchor) (*uuid.UUID, error) {
	// Primero se rechaza todo destino que el mapa no admita desde el estado vigente.
	allowed := AllowedTransitions(current)
	if !slices.Contains(allowed, to) {
		return nil, &InvalidTransitionError{Current: current, Requested: to, Allowed: allowed}
	}
	if to != DefectStatusResolved {
		return nil, nil
	}

	// Resolver exige un sprint Activo o Cerrado que no empiece antes que el de detección; el mismo
	// sprint de detección es válido (RN-13, RN-14, FR-025).
	switch {
	case resolution == nil:
		return nil, ErrResolutionSprintRequired
	case resolution.Status == sprintsdomain.SprintStatusPlanned:
		return nil, ErrResolutionSprintPlanned
	case resolution.StartDate.Before(detected.StartDate):
		return nil, ErrResolutionBeforeDetection
	}
	resolvedID := resolution.ID
	return &resolvedID, nil
}

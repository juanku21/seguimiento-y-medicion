package domain

// SprintStatus es el estado de un sprint; se guarda como texto en la columna sprints.status y no
// como enum de PostgreSQL (data-model.md sección 4). Los valores están en inglés y la etiqueta en
// español vive en el frontend.
type SprintStatus string

// Estados posibles de un sprint: planned (Planificado), active (Activo) y closed (Cerrado). Todo
// sprint nace planned; las transiciones admitidas (planned → active → closed) son regla de negocio
// y se validan en la historia de inicio y cierre (FR-047, RN-02).
const (
	SprintStatusPlanned SprintStatus = "planned"
	SprintStatusActive  SprintStatus = "active"
	SprintStatusClosed  SprintStatus = "closed"
)

// allowedSprintTransitions asocia cada estado con el único estado al que puede avanzar; closed no
// figura porque es terminal (RN-02).
var allowedSprintTransitions = map[SprintStatus]SprintStatus{
	SprintStatusPlanned: SprintStatusActive,
	SprintStatusActive:  SprintStatusClosed,
}

// ValidateTransition indica si el sprint puede pasar del estado actual al destino: devuelve nil
// solo para planned -> active y active -> closed, y ErrInvalidSprintTransition para cualquier
// salto, retroceso o permanencia en el mismo estado (FR-047, RN-02).
func (s SprintStatus) ValidateTransition(to SprintStatus) error {
	if next, ok := allowedSprintTransitions[s]; ok && next == to {
		return nil
	}
	return ErrInvalidSprintTransition
}

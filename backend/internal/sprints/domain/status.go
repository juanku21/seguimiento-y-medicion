package domain

import "errors"

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

// ValidateTransition indica si el sprint puede pasar del estado actual al destino. Firma mínima
// para que compilen las pruebas de T039; la implementación corresponde a T045.
func (s SprintStatus) ValidateTransition(to SprintStatus) error {
	return errors.New("not implemented")
}

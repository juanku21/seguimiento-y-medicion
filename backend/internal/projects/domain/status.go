package domain

import "errors"

// ProjectStatus es el punto del ciclo de vida en que está un proyecto; se guarda como texto en la
// columna projects.status y no como enum de PostgreSQL (data-model.md sección 3). Los valores están
// en inglés y la etiqueta en español vive en el frontend.
type ProjectStatus string

// Estados posibles de un proyecto: planned es el estado inicial, in_progress indica que el proyecto
// arrancó y finished que cerró y queda de solo lectura.
const (
	ProjectStatusPlanned    ProjectStatus = "planned"
	ProjectStatusInProgress ProjectStatus = "in_progress"
	ProjectStatusFinished   ProjectStatus = "finished"
)

// ValidateTransition indica si el proyecto puede pasar del estado actual al destino: devuelve nil
// para las transiciones permitidas y ErrInvalidStatusTransition para las demás (FR-037, FR-038).
// Firma mínima de T070; la regla la implementa T074.
func (s ProjectStatus) ValidateTransition(to ProjectStatus) error {
	return errors.New("not implemented")
}

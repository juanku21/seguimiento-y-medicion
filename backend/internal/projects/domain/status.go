package domain

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

// allowedTransitions asocia cada estado con el único estado al que puede avanzar; finished no
// figura porque es terminal (FR-037).
var allowedTransitions = map[ProjectStatus]ProjectStatus{
	ProjectStatusPlanned:    ProjectStatusInProgress,
	ProjectStatusInProgress: ProjectStatusFinished,
}

// ValidateTransition indica si el proyecto puede pasar del estado actual al destino: devuelve nil
// solo para planned -> in_progress e in_progress -> finished, y ErrInvalidStatusTransition para
// cualquier salto, retroceso o permanencia en el mismo estado (FR-037, FR-038).
func (s ProjectStatus) ValidateTransition(to ProjectStatus) error {
	if next, ok := allowedTransitions[s]; ok && next == to {
		return nil
	}
	return ErrInvalidStatusTransition
}

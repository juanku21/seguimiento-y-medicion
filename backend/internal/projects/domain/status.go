package domain

// ProjectStatus es el punto del ciclo de vida en que está un proyecto; se guarda como texto en la
// columna projects.status y no como enum de PostgreSQL (data-model.md sección 3). Los valores están
// en inglés y la etiqueta en español vive en el frontend. Las transiciones permitidas son regla de
// negocio y no se declaran acá.
type ProjectStatus string

// Estados posibles de un proyecto: planned es el estado inicial, in_progress indica que el proyecto
// arrancó y finished que cerró y queda de solo lectura.
const (
	ProjectStatusPlanned    ProjectStatus = "planned"
	ProjectStatusInProgress ProjectStatus = "in_progress"
	ProjectStatusFinished   ProjectStatus = "finished"
)

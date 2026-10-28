package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProjectData son los datos crudos de un proyecto que entrega el repositorio.
type ProjectData struct {
	ProjectID uuid.UUID
	// FactorHundredths es el factor vigente del proyecto.
	FactorHundredths int
	// Sprints incluye todos los sprints del proyecto, en orden start_date ASC, id ASC.
	Sprints []SprintData
	// OutsideSprintsHundredths es el esfuerzo sin sprint asociado (FR-029).
	OutsideSprintsHundredths int
	// UnestimatedBacklogStories cuenta las historias del proyecto sin estimar (FR-036).
	UnestimatedBacklogStories int
}

// VelocityPoint es la velocidad de un sprint Cerrado.
type VelocityPoint struct {
	SprintID             uuid.UUID
	SprintName           string
	ClosedAt             *time.Time
	CompletedStoryPoints int
}

// Velocity es la velocidad del proyecto sobre la ventana de los últimos Cerrados.
type Velocity struct {
	// Value es el promedio de la ventana, sin Finality.
	Value Metric
	// SprintsConsidered es el tamaño de la ventana, de 0 a 3.
	SprintsConsidered int
	// Series tiene un punto por sprint Cerrado, en orden start_date ASC, id ASC.
	Series []VelocityPoint
}

// Cumulative acumula los sprints Cerrados y el Activo; la marca de finalidad es del bloque (FR-023).
type Cumulative struct {
	Finality             Finality
	IncludesActiveSprint bool

	PlannedStoryPoints     Metric
	CompletedStoryPoints   Metric
	EstimatedHours         Metric
	ActualHours            Metric
	DeviationHours         Metric
	DeviationPercent       Metric
	StoryCompletionPercent Metric
}

// ProjectMetrics es el resultado del cálculo de un proyecto.
type ProjectMetrics struct {
	Velocity   Velocity
	Cumulative Cumulative
	// EffortOutsideSprintsHundredths es el esfuerzo sin sprint, siempre presente.
	EffortOutsideSprintsHundredths int
	// EffortInPlannedSprintsHundredths es el esfuerzo en sprints Planificados, siempre presente.
	EffortInPlannedSprintsHundredths int
	// UnestimatedBacklogStories es siempre presente, 0 incluido.
	UnestimatedBacklogStories int
	Warnings                  []Warning
}

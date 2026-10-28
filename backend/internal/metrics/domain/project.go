package domain

import (
	"bytes"
	"slices"
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// velocityWindowSize es la cantidad de sprints Cerrados que promedia la velocidad (RN-10).
const velocityWindowSize = 3

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

// CalculateProject calcula las métricas de un proyecto. Por ahora resuelve la velocidad (US2).
func CalculateProject(data ProjectData, now time.Time, grace time.Duration) ProjectMetrics {
	return ProjectMetrics{Velocity: calculateVelocity(data, now, grace)}
}

// calculateVelocity arma la serie de todos los Cerrados y el promedio de la ventana de los últimos 3.
func calculateVelocity(data ProjectData, now time.Time, grace time.Duration) Velocity {
	// La serie conserva el orden de Sprints y reutiliza CalculateSprint para el valor de cada punto.
	closed := []SprintData{}
	series := []VelocityPoint{}
	for _, sprint := range data.Sprints {
		if sprint.Status != sprintsdomain.SprintStatusClosed {
			continue
		}
		closed = append(closed, sprint)
		metrics, _ := CalculateSprint(sprint, data.FactorHundredths, now, grace)
		series = append(series, VelocityPoint{
			SprintID:             sprint.ID,
			SprintName:           sprint.Name,
			ClosedAt:             sprint.ClosedAt,
			CompletedStoryPoints: metrics.CompletedStoryPoints.Value,
		})
	}
	if len(series) == 0 {
		return Velocity{Value: NotCalculable(), Series: series}
	}

	// La ventana ordena por cierre descendente (sin cierre al final) y desempata por ID descendente.
	slices.SortFunc(closed, compareByClosure)
	window := closed[:min(velocityWindowSize, len(closed))]
	sum := 0
	for _, sprint := range window {
		for _, point := range series {
			if point.SprintID == sprint.ID {
				sum += point.CompletedStoryPoints
				break
			}
		}
	}
	return Velocity{
		Value:             Available(AverageHundredths(sum, len(window))),
		SprintsConsidered: len(window),
		Series:            series,
	}
}

// compareByClosure ordena por ClosedAt descendente con los nulos al final, y por ID descendente.
func compareByClosure(a, b SprintData) int {
	switch {
	case a.ClosedAt == nil && b.ClosedAt != nil:
		return 1
	case a.ClosedAt != nil && b.ClosedAt == nil:
		return -1
	case a.ClosedAt != nil && b.ClosedAt != nil:
		if c := b.ClosedAt.Compare(*a.ClosedAt); c != 0 {
			return c
		}
	}
	return bytes.Compare(b.ID[:], a.ID[:])
}

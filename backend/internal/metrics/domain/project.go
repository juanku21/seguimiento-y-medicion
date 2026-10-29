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
	// DefectCounts son los defectos del proyecto agrupados por estado y severidad.
	DefectCounts []DefectCount
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
	// Defects son los defectos del proyecto, siempre presentes.
	Defects  ProjectDefects
	Warnings []Warning
}

// CalculateProject calcula las métricas de un proyecto: velocidad, acumulados, esfuerzo separado,
// contador de historias sin estimar y advertencias.
func CalculateProject(data ProjectData, now time.Time, grace time.Duration) ProjectMetrics {
	result := ProjectMetrics{
		EffortOutsideSprintsHundredths: data.OutsideSprintsHundredths,
		UnestimatedBacklogStories:      data.UnestimatedBacklogStories,
		Defects:                        TallyDefects(data.DefectCounts),
		Warnings:                       []Warning{},
	}

	// La advertencia de planificación depende solo del contador del backlog (RN-26).
	if data.UnestimatedBacklogStories > 0 {
		count := data.UnestimatedBacklogStories
		result.Warnings = append(result.Warnings, Warning{
			Kind:  WarningPlanning,
			Code:  CodeUnestimatedBacklogStories,
			Count: &count,
		})
	}

	// Se acumulan los Cerrados y el Activo; los Planificados solo aportan su esfuerzo aparte.
	var planned, completed, estimated, actual, stories, completedStories int
	includesActive := false
	series := []VelocityPoint{}
	for _, sprint := range data.Sprints {
		if sprint.Status == sprintsdomain.SprintStatusPlanned {
			result.EffortInPlannedSprintsHundredths += sprint.ActualHundredths
			continue
		}
		metrics, warnings := CalculateSprint(sprint, data.FactorHundredths, now, grace)
		result.Warnings = append(result.Warnings, warnings...)
		planned += metrics.PlannedStoryPoints.Value
		completed += metrics.CompletedStoryPoints.Value
		estimated += metrics.EstimatedHours.Value
		actual += metrics.ActualHours.Value
		stories += len(sprint.Stories)
		for _, story := range sprint.Stories {
			if story.Completed {
				completedStories++
			}
		}
		if sprint.Status == sprintsdomain.SprintStatusActive {
			includesActive = true
		}
		// Cada Cerrado aporta un punto de velocidad con las métricas ya calculadas.
		if sprint.Status == sprintsdomain.SprintStatusClosed {
			series = append(series, VelocityPoint{
				SprintID:             sprint.ID,
				SprintName:           sprint.Name,
				ClosedAt:             sprint.ClosedAt,
				CompletedStoryPoints: metrics.CompletedStoryPoints.Value,
			})
		}
	}
	result.Velocity = calculateVelocity(series)

	// Los cocientes salen de los totales, nunca del promedio de los porcentajes de cada sprint (RN-11).
	deviation := actual - estimated
	finality := FinalityDefinitive
	if includesActive {
		finality = FinalityPartial
	}
	result.Cumulative = Cumulative{
		Finality:               finality,
		IncludesActiveSprint:   includesActive,
		PlannedStoryPoints:     Available(planned),
		CompletedStoryPoints:   Available(completed),
		EstimatedHours:         Available(estimated),
		ActualHours:            Available(actual),
		DeviationHours:         Available(deviation),
		DeviationPercent:       Percent(deviation, estimated),
		StoryCompletionPercent: Percent(completedStories, stories),
	}
	return result
}

// calculateVelocity promedia la ventana de los últimos 3 Cerrados de la serie, que conserva el orden de Sprints.
func calculateVelocity(series []VelocityPoint) Velocity {
	if len(series) == 0 {
		return Velocity{Value: NotCalculable(), Series: series}
	}

	// La ventana se toma de una copia ordenada por cierre, para no alterar el orden de la serie.
	byClosure := slices.Clone(series)
	slices.SortFunc(byClosure, compareByClosure)
	window := byClosure[:min(velocityWindowSize, len(byClosure))]
	sum := 0
	for _, point := range window {
		sum += point.CompletedStoryPoints
	}
	return Velocity{
		Value:             Available(AverageHundredths(sum, len(window))),
		SprintsConsidered: len(window),
		Series:            series,
	}
}

// compareByClosure ordena por ClosedAt descendente con los nulos al final, y por ID descendente.
func compareByClosure(a, b VelocityPoint) int {
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
	return bytes.Compare(b.SprintID[:], a.SprintID[:])
}

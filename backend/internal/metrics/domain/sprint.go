package domain

import (
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// SprintData son los datos crudos de un sprint que entrega el repositorio.
type SprintData struct {
	ID   uuid.UUID
	Name string
	// Status es planned, active o closed.
	Status sprintsdomain.SprintStatus
	// StartDate y EndDate son medianoche UTC; StartDate ordena las series.
	StartDate time.Time
	EndDate   time.Time
	// ClosedAt solo existe en sprints Cerrados; ordena la ventana de velocidad y corre el plazo de gracia.
	ClosedAt *time.Time
	// FrozenFactorHundredths es el factor copiado al cerrar; solo existe en sprints Cerrados.
	FrozenFactorHundredths *int
	// Stories es la instantánea en el Cerrado y los compromisos abiertos en el resto.
	Stories []SprintStoryData
	// ActualHundredths suma las centésimas de hora de esfuerzo anclado al sprint.
	ActualHundredths int
	DefectsDetected  int
	DefectsResolved  int
}

// SprintStoryData es una historia comprometida en un sprint.
type SprintStoryData struct {
	StoryID uuid.UUID
	Title   string
	// StoryPoints es congelado en el Cerrado y vigente en el resto; nil es dato roto (FR-037).
	StoryPoints *int
	// Completed indica si la historia se completó en el sprint.
	Completed bool
}

// MetricKind identifica cada una de las nueve métricas de un sprint para decidir su Finality.
type MetricKind string

const (
	// MetricPlannedStoryPoints identifica los Story Points planificados.
	MetricPlannedStoryPoints MetricKind = "planned_story_points"
	// MetricCompletedStoryPoints identifica los Story Points completados.
	MetricCompletedStoryPoints MetricKind = "completed_story_points"
	// MetricEstimatedHours identifica las horas estimadas.
	MetricEstimatedHours MetricKind = "estimated_hours"
	// MetricActualHours identifica las horas reales.
	MetricActualHours MetricKind = "actual_hours"
	// MetricDeviationHours identifica el desvío en horas.
	MetricDeviationHours MetricKind = "deviation_hours"
	// MetricDeviationPercent identifica el desvío porcentual.
	MetricDeviationPercent MetricKind = "deviation_percent"
	// MetricStoryCompletionPercent identifica el porcentaje de historias completadas.
	MetricStoryCompletionPercent MetricKind = "story_completion_percent"
	// MetricDefectsDetected identifica los defectos detectados.
	MetricDefectsDetected MetricKind = "defects_detected"
	// MetricDefectsResolved identifica los defectos resueltos.
	MetricDefectsResolved MetricKind = "defects_resolved"
)

// SprintMetrics es el resultado del cálculo de un sprint: el sprint y sus nueve métricas.
type SprintMetrics struct {
	ID        uuid.UUID
	Name      string
	Status    sprintsdomain.SprintStatus
	StartDate time.Time
	EndDate   time.Time
	ClosedAt  *time.Time

	PlannedStoryPoints     Metric
	CompletedStoryPoints   Metric
	EstimatedHours         Metric
	ActualHours            Metric
	DeviationHours         Metric
	DeviationPercent       Metric
	StoryCompletionPercent Metric
	DefectsDetected        Metric
	DefectsResolved        Metric
}

// SprintFinality decide el estado de valor de una métrica según el estado del sprint (firma mínima, pendiente de T012).
func SprintFinality(status sprintsdomain.SprintStatus, kind MetricKind, graceOpen bool) *Finality {
	return nil
}

// CalculateSprint calcula las nueve métricas de un sprint (firma mínima, pendiente de T012).
func CalculateSprint(sprint SprintData, currentFactorHundredths int, now time.Time, grace time.Duration) (SprintMetrics, []Warning) {
	return SprintMetrics{}, nil
}

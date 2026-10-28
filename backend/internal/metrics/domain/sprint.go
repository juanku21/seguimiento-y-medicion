package domain

import (
	"time"

	"github.com/google/uuid"

	effortdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
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

// SprintFinality decide el estado de valor de una métrica según el estado del sprint
// (research.md sección 7). El sprint planificado devuelve nil hasta T024.
func SprintFinality(status sprintsdomain.SprintStatus, kind MetricKind, graceOpen bool) *Finality {
	// En un sprint activo las nueve métricas son parciales.
	if status == sprintsdomain.SprintStatusActive {
		f := FinalityPartial
		return &f
	}
	if status != sprintsdomain.SprintStatusClosed {
		return nil
	}

	var f Finality
	switch kind {
	case MetricActualHours, MetricDeviationHours, MetricDeviationPercent:
		// Dependen del esfuerzo, que puede cargarse durante el plazo de gracia.
		f = FinalityDefinitive
		if graceOpen {
			f = FinalityProvisional
		}
	case MetricDefectsDetected, MetricDefectsResolved:
		// Los defectos pueden seguir cambiando después del cierre.
		f = FinalityProvisional
	default:
		// Puntos, horas estimadas y porcentaje de historias salen de la instantánea congelada.
		f = FinalityDefinitive
	}
	return &f
}

// withFinality asigna a la métrica la finalidad que corresponde a su tipo.
func withFinality(m Metric, status sprintsdomain.SprintStatus, kind MetricKind, graceOpen bool) Metric {
	m.Finality = SprintFinality(status, kind, graceOpen)
	return m
}

// CalculateSprint calcula las nueve métricas de un sprint. Implementa el sprint cerrado y el activo;
// el planificado queda pendiente de T024.
func CalculateSprint(sprint SprintData, currentFactorHundredths int, now time.Time, grace time.Duration) (SprintMetrics, []Warning) {
	result := SprintMetrics{
		ID:        sprint.ID,
		Name:      sprint.Name,
		Status:    sprint.Status,
		StartDate: sprint.StartDate,
		EndDate:   sprint.EndDate,
		ClosedAt:  sprint.ClosedAt,
	}
	closed := sprint.Status == sprintsdomain.SprintStatusClosed
	if !closed && sprint.Status != sprintsdomain.SprintStatusActive {
		return result, nil
	}

	warnings := []Warning{}
	sprintID := sprint.ID

	// Suma de puntos: una historia sin puntos cuenta como 0 y genera una advertencia (FR-037).
	planned, completed, completedCount := 0, 0, 0
	for _, story := range sprint.Stories {
		points := 0
		if story.StoryPoints == nil {
			storyID := story.StoryID
			warnings = append(warnings, Warning{
				Kind:     WarningDataInconsistency,
				Code:     CodeUnestimatedStoryInSprint,
				SprintID: &sprintID,
				StoryID:  &storyID,
			})
		} else {
			points = *story.StoryPoints
		}
		planned += points
		if story.Completed {
			completed += points
			completedCount++
		}
	}

	// Factor congelado solo en el cerrado; sin él se usa el vigente y se advierte. El activo usa el vigente.
	factor := currentFactorHundredths
	switch {
	case closed && sprint.FrozenFactorHundredths != nil:
		factor = *sprint.FrozenFactorHundredths
	case closed:
		warnings = append(warnings, Warning{
			Kind:     WarningDataInconsistency,
			Code:     CodeMissingFrozenFactor,
			SprintID: &sprintID,
		})
	}

	// Plazo de gracia, solo en el cerrado; sin fecha de cierre se lo considera vencido y se advierte.
	graceOpen := false
	switch {
	case closed && sprint.ClosedAt != nil:
		graceOpen = effortdomain.GraceOpen(*sprint.ClosedAt, now, grace)
	case closed:
		warnings = append(warnings, Warning{
			Kind:     WarningDataInconsistency,
			Code:     CodeMissingCloseTime,
			SprintID: &sprintID,
		})
	}

	estimated := planned * factor
	deviation := sprint.ActualHundredths - estimated
	status := sprint.Status

	result.PlannedStoryPoints = withFinality(Available(planned), status, MetricPlannedStoryPoints, graceOpen)
	result.CompletedStoryPoints = withFinality(Available(completed), status, MetricCompletedStoryPoints, graceOpen)
	result.EstimatedHours = withFinality(Available(estimated), status, MetricEstimatedHours, graceOpen)
	result.ActualHours = withFinality(Available(sprint.ActualHundredths), status, MetricActualHours, graceOpen)
	result.DeviationHours = withFinality(Available(deviation), status, MetricDeviationHours, graceOpen)
	result.DeviationPercent = withFinality(Percent(deviation, estimated), status, MetricDeviationPercent, graceOpen)
	result.StoryCompletionPercent = withFinality(Percent(completedCount, len(sprint.Stories)), status, MetricStoryCompletionPercent, graceOpen)
	result.DefectsDetected = withFinality(Available(sprint.DefectsDetected), status, MetricDefectsDetected, graceOpen)
	result.DefectsResolved = withFinality(Available(sprint.DefectsResolved), status, MetricDefectsResolved, graceOpen)
	return result, warnings
}

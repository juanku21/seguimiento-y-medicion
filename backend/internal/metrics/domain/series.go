package domain

import (
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// SeriesPoint es el valor de una métrica en un sprint; conserva la Finality de su sprint.
type SeriesPoint struct {
	SprintID     uuid.UUID
	SprintName   string
	SprintStatus sprintsdomain.SprintStatus
	Value        Metric
}

// ProjectSeries agrupa las seis series del proyecto con los mismos sprints en el mismo orden
// (start_date ASC, id ASC), solo Cerrados y Activo.
type ProjectSeries struct {
	PlannedStoryPoints   []SeriesPoint
	CompletedStoryPoints []SeriesPoint
	EstimatedHours       []SeriesPoint
	ActualHours          []SeriesPoint
	DefectsDetected      []SeriesPoint
	DefectsResolved      []SeriesPoint
}

// BuildSeries arma las seis series del proyecto con los sprints Cerrados y el Activo.
func BuildSeries(data ProjectData, now time.Time, grace time.Duration) (ProjectSeries, []Warning) {
	series := ProjectSeries{
		PlannedStoryPoints:   []SeriesPoint{},
		CompletedStoryPoints: []SeriesPoint{},
		EstimatedHours:       []SeriesPoint{},
		ActualHours:          []SeriesPoint{},
		DefectsDetected:      []SeriesPoint{},
		DefectsResolved:      []SeriesPoint{},
	}
	warnings := []Warning{}

	// Cada punto sale de CalculateSprint, así que nunca difiere del detalle de su sprint.
	for _, sprint := range data.Sprints {
		if sprint.Status != sprintsdomain.SprintStatusClosed && sprint.Status != sprintsdomain.SprintStatusActive {
			continue
		}
		metrics, sprintWarnings := CalculateSprint(sprint, data.FactorHundredths, now, grace)
		warnings = append(warnings, sprintWarnings...)

		point := func(value Metric) SeriesPoint {
			return SeriesPoint{SprintID: sprint.ID, SprintName: sprint.Name, SprintStatus: sprint.Status, Value: value}
		}
		series.PlannedStoryPoints = append(series.PlannedStoryPoints, point(metrics.PlannedStoryPoints))
		series.CompletedStoryPoints = append(series.CompletedStoryPoints, point(metrics.CompletedStoryPoints))
		series.EstimatedHours = append(series.EstimatedHours, point(metrics.EstimatedHours))
		series.ActualHours = append(series.ActualHours, point(metrics.ActualHours))
		series.DefectsDetected = append(series.DefectsDetected, point(metrics.DefectsDetected))
		series.DefectsResolved = append(series.DefectsResolved, point(metrics.DefectsResolved))
	}
	return series, warnings
}

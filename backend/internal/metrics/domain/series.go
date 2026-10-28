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
	return ProjectSeries{}, nil
}

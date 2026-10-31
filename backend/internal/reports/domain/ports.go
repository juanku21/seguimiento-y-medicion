package domain

import (
	"context"

	"github.com/google/uuid"

	metricsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// ReportRepository agrupa las lecturas con las que se arma el reporte; cada una es independiente
// para que la falla de una no aborte las demás.
type ReportRepository interface {
	LoadHeader(ctx context.Context, projectID, userID uuid.UUID) (ProjectInfo, UserInfo, error)
	LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (SprintRef, error)
	ListSprints(ctx context.Context, projectID uuid.UUID) ([]SprintRef, error)
	LoadSprintStories(ctx context.Context, sprint SprintRef) ([]StoryLine, error)
	LoadUnplannedStories(ctx context.Context, projectID uuid.UUID) ([]StoryLine, error)
	LoadEstimationSessions(ctx context.Context, storyIDs []uuid.UUID) (map[uuid.UUID][]SessionRecord, error)
	LoadEffort(ctx context.Context, sprintIDs []uuid.UUID) ([]EffortRow, error)
	LoadDefects(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID) ([]DefectLine, error)
}

// GenerationRepository persiste el registro de cada reporte entregado.
type GenerationRepository interface {
	Save(ctx context.Context, generation ReportGeneration) error
}

// MetricsSource es la fuente de métricas que reutiliza el reporte; la cumple el servicio del módulo
// de métricas.
type MetricsSource interface {
	SprintMetrics(ctx context.Context, projectID, sprintID uuid.UUID) (metricsdomain.SprintMetrics, []metricsdomain.Warning, error)
	ProjectMetrics(ctx context.Context, projectID uuid.UUID) (metricsdomain.ProjectMetrics, error)
}

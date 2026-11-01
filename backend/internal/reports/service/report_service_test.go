package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metricsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// failingGenerations es un GenerationRepository de prueba cuyo Save siempre devuelve el error dado.
type failingGenerations struct {
	err   error
	saved int
}

func (g *failingGenerations) Save(ctx context.Context, generation domain.ReportGeneration) error {
	g.saved++
	return g.err
}

// stubReportRepository devuelve un sprint Activo vacío y un proyecto cualquiera, de modo que la
// única lectura que falla en la prueba es el guardado del registro de generación.
type stubReportRepository struct {
	sprint domain.SprintRef
}

func (r stubReportRepository) LoadHeader(ctx context.Context, projectID, userID uuid.UUID) (domain.ProjectInfo, domain.UserInfo, error) {
	return domain.ProjectInfo{ID: projectID, Name: "Proyecto"}, domain.UserInfo{ID: userID, FullName: "Ana Pérez"}, nil
}

func (r stubReportRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (domain.SprintRef, error) {
	return r.sprint, nil
}

func (r stubReportRepository) ListSprints(ctx context.Context, projectID uuid.UUID) ([]domain.SprintRef, error) {
	return []domain.SprintRef{r.sprint}, nil
}

func (r stubReportRepository) LoadSprintStories(ctx context.Context, sprint domain.SprintRef) ([]domain.StoryLine, error) {
	return nil, nil
}

func (r stubReportRepository) LoadUnplannedStories(ctx context.Context, projectID uuid.UUID) ([]domain.StoryLine, error) {
	return nil, nil
}

func (r stubReportRepository) LoadEstimationSessions(ctx context.Context, projectID uuid.UUID, storyIDs []uuid.UUID) (map[uuid.UUID][]domain.SessionRecord, error) {
	return map[uuid.UUID][]domain.SessionRecord{}, nil
}

func (r stubReportRepository) LoadEffort(ctx context.Context, projectID uuid.UUID, sprintIDs []uuid.UUID) ([]domain.EffortRow, error) {
	return nil, nil
}

func (r stubReportRepository) LoadDefects(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID) ([]domain.DefectLine, error) {
	return nil, nil
}

// stubMetricsSource devuelve métricas vacías sin error.
type stubMetricsSource struct{}

func (stubMetricsSource) SprintMetrics(ctx context.Context, projectID, sprintID uuid.UUID) (metricsdomain.SprintMetrics, []metricsdomain.Warning, error) {
	return metricsdomain.SprintMetrics{}, nil, nil
}

func (stubMetricsSource) ProjectMetrics(ctx context.Context, projectID uuid.UUID) (metricsdomain.ProjectMetrics, error) {
	return metricsdomain.ProjectMetrics{}, nil
}

// Si no se puede guardar el registro de generación, el reporte no se entrega (SC-018).
func TestSprintReportSaveFails(t *testing.T) {
	errSave := errors.New("falla simulada al guardar la generación")
	generations := &failingGenerations{err: errSave}
	sprint := domain.SprintRef{
		ID:        uuid.New(),
		Name:      "Sprint 1",
		Status:    sprintsdomain.SprintStatusActive,
		StartDate: time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC),
	}
	svc := NewReportService(stubReportRepository{sprint: sprint}, generations, stubMetricsSource{},
		func() time.Time { return time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC) })

	report, err := svc.SprintReport(context.Background(), uuid.New(), sprint.ID, uuid.New())

	// Devuelve el error del guardado (envuelto) y un reporte vacío, tras intentar guardar una vez.
	require.ErrorIs(t, err, errSave)
	assert.Equal(t, domain.Report{}, report, "no debe entregar un reporte")
	assert.Equal(t, 1, generations.saved, "debe intentar guardar el registro una vez")
}

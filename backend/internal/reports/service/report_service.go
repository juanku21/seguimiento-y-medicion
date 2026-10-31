// Package service contiene la lógica de negocio del módulo de reportes (Principio III).
package service

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// ReportService arma los reportes de sprint y de proyecto a partir de las lecturas del repositorio
// y de las métricas del módulo de métricas.
type ReportService struct {
	repo        domain.ReportRepository
	generations domain.GenerationRepository
	metrics     domain.MetricsSource
	now         func() time.Time
}

// NewReportService crea el servicio con sus puertos y el reloj.
func NewReportService(repo domain.ReportRepository, generations domain.GenerationRepository, metrics domain.MetricsSource, now func() time.Time) *ReportService {
	return &ReportService{repo: repo, generations: generations, metrics: metrics, now: now}
}

// SprintReport arma el reporte de un sprint del proyecto y registra su generación. Toda falla de
// lectura se propaga con contexto; si el guardado del registro falla, no se entrega el reporte
// (research.md sección 10).
func (s *ReportService) SprintReport(ctx context.Context, projectID, sprintID, userID uuid.UUID) (domain.Report, error) {
	// El instante se toma una sola vez: es el generatedAt del encabezado y del registro.
	now := s.now()

	// El sprint se lee acotado al proyecto y debe ser reportable (RN-04).
	sprint, err := s.repo.LoadSprint(ctx, projectID, sprintID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar el reporte de sprint: %w", err)
	}
	if err := domain.EnsureReportable(sprint.Status); err != nil {
		return domain.Report{}, fmt.Errorf("armar el reporte de sprint: %w", err)
	}

	project, user, err := s.repo.LoadHeader(ctx, projectID, userID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar el encabezado del reporte: %w", err)
	}

	// Historias del sprint en orden estable y estado de la sección.
	stories, err := s.repo.LoadSprintStories(ctx, sprint)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar las historias del reporte: %w", err)
	}
	domain.SortStories(stories)
	storyGroups := []domain.StoryGroup{{Sprint: &sprint, State: groupState(len(stories)), Stories: stories}}
	storiesSection := domain.StoriesSection{State: domain.StoriesState(storyGroups), Groups: storyGroups}

	estimations, err := s.sprintEstimations(ctx, sprint, stories)
	if err != nil {
		return domain.Report{}, err
	}

	effort, err := s.sprintEffort(ctx, sprint, stories)
	if err != nil {
		return domain.Report{}, err
	}

	// Métricas del sprint reutilizadas del módulo de métricas.
	sprintMetrics, warnings, err := s.metrics.SprintMetrics(ctx, projectID, sprintID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar las métricas del reporte: %w", err)
	}

	// Defectos del sprint; los totales salen de las métricas.
	defects, err := s.repo.LoadDefects(ctx, projectID, &sprintID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar los defectos del reporte: %w", err)
	}
	defectsSection := domain.DefectsSection{
		State:   groupState(len(defects)),
		Defects: defects,
		Totals: &domain.DefectTotals{
			Detected: sprintMetrics.DefectsDetected.Value,
			Resolved: sprintMetrics.DefectsResolved.Value,
		},
	}

	report := domain.Report{
		Scope:   domain.ScopeSprint,
		Partial: domain.IsPartial(domain.ScopeSprint, []domain.SprintRef{sprint}),
		Header: domain.Header{
			Project:     project,
			Sprint:      &sprint,
			Period:      &domain.Period{StartDate: sprint.StartDate, EndDate: sprint.EndDate},
			GeneratedAt: now,
			GeneratedBy: user,
		},
		Stories:     storiesSection,
		Estimations: estimations,
		Effort:      effort,
		Metrics: domain.MetricsSection{
			State:          domain.SectionComplete,
			Sprint:         &sprintMetrics,
			SprintWarnings: warnings,
		},
		Defects: defectsSection,
	}

	// El registro es la última operación: si falla, no se entrega el reporte.
	generation := domain.ReportGeneration{
		ID:            uuid.New(),
		ProjectID:     projectID,
		SprintID:      &sprintID,
		Scope:         domain.ScopeSprint,
		GeneratedByID: userID,
		GeneratedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.generations.Save(ctx, generation); err != nil {
		return domain.Report{}, fmt.Errorf("registrar la generación del reporte: %w", err)
	}
	return report, nil
}

// groupState devuelve empty si no hay elementos y complete en caso contrario (RN-11).
func groupState(count int) domain.SectionState {
	if count == 0 {
		return domain.SectionEmpty
	}
	return domain.SectionComplete
}

// sprintEstimations arma la sección de estimaciones con las mismas historias y el mismo orden que la
// de historias; el corte es el cierre del sprint Cerrado y no existe en el Activo (RN-09, RN-10).
func (s *ReportService) sprintEstimations(ctx context.Context, sprint domain.SprintRef, stories []domain.StoryLine) (domain.EstimationsSection, error) {
	storyIDs := make([]uuid.UUID, 0, len(stories))
	for _, story := range stories {
		storyIDs = append(storyIDs, story.StoryID)
	}
	sessions, err := s.repo.LoadEstimationSessions(ctx, storyIDs)
	if err != nil {
		return domain.EstimationsSection{}, fmt.Errorf("armar las estimaciones del reporte: %w", err)
	}

	// Una línea por historia con el estado de Planning Poker al corte.
	items := make([]domain.EstimationLine, 0, len(stories))
	for _, story := range stories {
		poker, summary := domain.SelectEstimation(sessions[story.StoryID], sprint.ClosedAt)
		items = append(items, domain.EstimationLine{
			StoryID:     story.StoryID,
			Title:       story.Title,
			StoryPoints: story.StoryPoints,
			Poker:       poker,
			Session:     summary,
		})
	}
	groups := []domain.EstimationGroup{{Sprint: &sprint, State: groupState(len(items)), Items: items}}
	return domain.EstimationsSection{State: groupState(len(items)), Groups: groups}, nil
}

// sprintEffort arma la sección de esfuerzo del sprint: horas por historia, por integrante y total,
// todo sumado desde las mismas filas para que no se contradigan (research.md sección 6).
func (s *ReportService) sprintEffort(ctx context.Context, sprint domain.SprintRef, stories []domain.StoryLine) (domain.EffortSection, error) {
	rows, err := s.repo.LoadEffort(ctx, []uuid.UUID{sprint.ID})
	if err != nil {
		return domain.EffortSection{}, fmt.Errorf("armar el esfuerzo del reporte: %w", err)
	}

	// Títulos de la instantánea o vigentes de las historias del sprint.
	snapshot := make(map[uuid.UUID]string, len(stories))
	for _, story := range stories {
		snapshot[story.StoryID] = story.Title
	}

	byStory := map[uuid.UUID]*domain.StoryEffort{}
	byMember := map[uuid.UUID]*domain.MemberEffort{}
	total := 0
	for _, row := range rows {
		var title *string
		if t, ok := snapshot[row.StoryID]; ok {
			title = &t
		}
		if byStory[row.StoryID] == nil {
			byStory[row.StoryID] = &domain.StoryEffort{StoryID: row.StoryID, Title: domain.StoryTitle(title, row.CurrentTitle)}
		}
		byStory[row.StoryID].Hundredths += row.Hundredths
		if byMember[row.UserID] == nil {
			byMember[row.UserID] = &domain.MemberEffort{UserID: row.UserID, FullName: row.FullName}
		}
		byMember[row.UserID].Hundredths += row.Hundredths
		total += row.Hundredths
	}

	storyEfforts := make([]domain.StoryEffort, 0, len(byStory))
	for _, e := range byStory {
		storyEfforts = append(storyEfforts, *e)
	}
	// Historias por horas descendentes, después título y después id.
	slices.SortFunc(storyEfforts, func(a, b domain.StoryEffort) int {
		return cmp.Or(cmp.Compare(b.Hundredths, a.Hundredths), cmp.Compare(a.Title, b.Title), bytes.Compare(a.StoryID[:], b.StoryID[:]))
	})
	memberEfforts := make([]domain.MemberEffort, 0, len(byMember))
	for _, e := range byMember {
		memberEfforts = append(memberEfforts, *e)
	}
	// Integrantes por nombre y después id.
	slices.SortFunc(memberEfforts, func(a, b domain.MemberEffort) int {
		return cmp.Or(cmp.Compare(a.FullName, b.FullName), bytes.Compare(a.UserID[:], b.UserID[:]))
	})

	state := groupState(len(rows))
	group := domain.EffortGroup{Sprint: sprint, State: state, Stories: storyEfforts, TotalHundredths: total}
	return domain.EffortSection{
		State:           state,
		Groups:          []domain.EffortGroup{group},
		ByMember:        memberEfforts,
		TotalHundredths: total,
	}, nil
}

// ProjectReport arma el reporte de todo el proyecto.
func (s *ReportService) ProjectReport(ctx context.Context, projectID, userID uuid.UUID) (domain.Report, error) {
	return domain.Report{}, errors.New("no implementado")
}

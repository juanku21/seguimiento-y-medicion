// Package service contiene la lógica de negocio del módulo de reportes (Principio III).
package service

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
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

	// Historias y estimaciones del sprint; cada sección falla por separado (research.md sección 8).
	storyGroups, stories, estimations := s.storiesAndEstimations(ctx, projectID, []domain.SprintRef{sprint}, false)
	effort := s.effortSection(ctx, projectID, []domain.SprintRef{sprint}, storyGroups)

	// Métricas del sprint reutilizadas del módulo de métricas; si fallan, los defectos tampoco se
	// pueden informar porque sus totales salen de ellas.
	metricsSection := domain.MetricsSection{State: domain.SectionUnavailable}
	defects := domain.DefectsSection{State: domain.SectionUnavailable}
	sprintMetrics, warnings, err := s.metrics.SprintMetrics(ctx, projectID, sprintID)
	if err != nil {
		logSectionFailure("metrics", projectID, err)
	} else {
		metricsSection = domain.MetricsSection{
			State:          domain.ProjectMetricsState(domain.ScopeSprint, []domain.SprintRef{sprint}),
			Sprint:         &sprintMetrics,
			SprintWarnings: warnings,
		}
		defects = s.defectsSection(ctx, projectID, &sprintID, sprintMetrics.DefectsDetected.Value, sprintMetrics.DefectsResolved.Value, nil)
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
		Stories:     stories,
		Estimations: estimations,
		Effort:      effort,
		Metrics:     metricsSection,
		Defects:     defects,
	}

	// El registro es la última operación: si falla, no se entrega el reporte.
	if err := s.saveGeneration(ctx, projectID, &sprintID, domain.ScopeSprint, userID, now); err != nil {
		return domain.Report{}, err
	}
	return report, nil
}

// ProjectReport arma el reporte de todo el proyecto con un grupo por sprint Activo o Cerrado y, al
// final, el de historias no planificadas, y registra su generación.
func (s *ReportService) ProjectReport(ctx context.Context, projectID, userID uuid.UUID) (domain.Report, error) {
	// El instante se toma una sola vez: es el generatedAt del encabezado y del registro.
	now := s.now()

	project, user, err := s.repo.LoadHeader(ctx, projectID, userID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar el encabezado del reporte: %w", err)
	}

	// Solo los sprints Activo y Cerrado, en el orden de lectura (RN-05, RN-07).
	allSprints, err := s.repo.ListSprints(ctx, projectID)
	if err != nil {
		return domain.Report{}, fmt.Errorf("armar los sprints del reporte: %w", err)
	}
	sprints := domain.ReportableSprints(allSprints)

	// Historias y estimaciones con un grupo por sprint y el de no planificadas al final; cada
	// sección falla por separado (research.md sección 8).
	storyGroups, stories, estimations := s.storiesAndEstimations(ctx, projectID, sprints, true)
	effort := s.effortSection(ctx, projectID, sprints, storyGroups)

	// Métricas del proyecto reutilizadas del módulo de métricas; si fallan, los defectos tampoco se
	// pueden informar porque sus totales salen de ellas.
	metricsSection := domain.MetricsSection{State: domain.SectionUnavailable}
	defects := domain.DefectsSection{State: domain.SectionUnavailable}
	projectMetrics, err := s.metrics.ProjectMetrics(ctx, projectID)
	if err != nil {
		logSectionFailure("metrics", projectID, err)
	} else {
		metricsSection = domain.MetricsSection{State: domain.ProjectMetricsState(domain.ScopeProject, sprints)}
		if metricsSection.State == domain.SectionComplete {
			metricsSection.Project = &projectMetrics
		}
		defects = s.defectsSection(ctx, projectID, nil, projectMetrics.Defects.Detected.Total, projectMetrics.Defects.Resolved.Total, storyGroups)
	}

	report := domain.Report{
		Scope:   domain.ScopeProject,
		Partial: domain.IsPartial(domain.ScopeProject, sprints),
		Header: domain.Header{
			Project:     project,
			Period:      domain.ReportPeriod(sprints),
			GeneratedAt: now,
			GeneratedBy: user,
		},
		Stories:     stories,
		Estimations: estimations,
		Effort:      effort,
		Metrics:     metricsSection,
		Defects:     defects,
	}

	// El registro es la última operación: si falla, no se entrega el reporte.
	if err := s.saveGeneration(ctx, projectID, nil, domain.ScopeProject, userID, now); err != nil {
		return domain.Report{}, err
	}
	return report, nil
}

// saveGeneration registra la entrega de un reporte; sprintID es nil en el alcance proyecto.
func (s *ReportService) saveGeneration(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, scope domain.ReportScope, userID uuid.UUID, now time.Time) error {
	generation := domain.ReportGeneration{
		ID:            uuid.New(),
		ProjectID:     projectID,
		SprintID:      sprintID,
		Scope:         scope,
		GeneratedByID: userID,
		GeneratedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.generations.Save(ctx, generation); err != nil {
		return fmt.Errorf("registrar la generación del reporte: %w", err)
	}
	return nil
}

// groupState devuelve empty si no hay elementos y complete en caso contrario (RN-11).
func groupState(count int) domain.SectionState {
	if count == 0 {
		return domain.SectionEmpty
	}
	return domain.SectionComplete
}

// sprintStoryGroup lee las historias de un sprint en orden estable y arma su grupo.
func (s *ReportService) sprintStoryGroup(ctx context.Context, sprint domain.SprintRef) (domain.StoryGroup, error) {
	stories, err := s.repo.LoadSprintStories(ctx, sprint)
	if err != nil {
		return domain.StoryGroup{}, fmt.Errorf("armar las historias del reporte: %w", err)
	}
	domain.SortStories(stories)
	return domain.StoryGroup{Sprint: &sprint, State: groupState(len(stories)), Stories: stories}, nil
}

// estimationGroup arma el grupo de estimaciones de las historias de un grupo, en el mismo orden; el
// corte es el cierre del sprint Cerrado y no existe en el Activo ni en las no planificadas, donde
// sprint es nil (RN-09, RN-10).
func (s *ReportService) estimationGroup(ctx context.Context, projectID uuid.UUID, sprint *domain.SprintRef, stories []domain.StoryLine) (domain.EstimationGroup, error) {
	storyIDs := make([]uuid.UUID, 0, len(stories))
	for _, story := range stories {
		storyIDs = append(storyIDs, story.StoryID)
	}
	sessions, err := s.repo.LoadEstimationSessions(ctx, projectID, storyIDs)
	if err != nil {
		return domain.EstimationGroup{}, fmt.Errorf("armar las estimaciones del reporte: %w", err)
	}

	// Corte de la selección: solo un sprint Cerrado lo tiene.
	var cutoff *time.Time
	if sprint != nil {
		cutoff = sprint.ClosedAt
	}

	// Una línea por historia con el estado de Planning Poker al corte.
	items := make([]domain.EstimationLine, 0, len(stories))
	for _, story := range stories {
		poker, summary := domain.SelectEstimation(sessions[story.StoryID], cutoff)
		items = append(items, domain.EstimationLine{
			StoryID:     story.StoryID,
			Title:       story.Title,
			StoryPoints: story.StoryPoints,
			Poker:       poker,
			Session:     summary,
		})
	}
	return domain.EstimationGroup{Sprint: sprint, State: groupState(len(items)), Items: items}, nil
}

// logSectionFailure registra una sección que no se pudo obtener; la falla no se propaga.
func logSectionFailure(section string, projectID uuid.UUID, err error) {
	slog.Error("no se pudo obtener la sección del reporte", "section", section, "projectId", projectID, "error", err)
}

// storiesAndEstimations arma los grupos de historias y de estimaciones. Si falla la lectura de
// historias, ambas secciones quedan unavailable; si falla solo la de sesiones, únicamente las
// estimaciones. includeUnplanned agrega al final el grupo de historias no planificadas.
func (s *ReportService) storiesAndEstimations(ctx context.Context, projectID uuid.UUID, sprints []domain.SprintRef, includeUnplanned bool) ([]domain.StoryGroup, domain.StoriesSection, domain.EstimationsSection) {
	unavailableStories := domain.StoriesSection{State: domain.SectionUnavailable}
	unavailableEstimations := domain.EstimationsSection{State: domain.SectionUnavailable}

	// Un grupo de historias por sprint; cualquier falla de lectura deja sin historias ni estimaciones.
	storyGroups := make([]domain.StoryGroup, 0, len(sprints)+1)
	for _, sprint := range sprints {
		storyGroup, err := s.sprintStoryGroup(ctx, sprint)
		if err != nil {
			logSectionFailure("stories", projectID, err)
			return nil, unavailableStories, unavailableEstimations
		}
		storyGroups = append(storyGroups, storyGroup)
	}

	// Grupo de no planificadas al final, siempre presente y sin corte.
	if includeUnplanned {
		unplanned, err := s.repo.LoadUnplannedStories(ctx, projectID)
		if err != nil {
			logSectionFailure("stories", projectID, err)
			return nil, unavailableStories, unavailableEstimations
		}
		domain.SortStories(unplanned)
		storyGroups = append(storyGroups, domain.StoryGroup{State: groupState(len(unplanned)), Stories: unplanned})
	}
	stories := domain.StoriesSection{State: domain.StoriesState(storyGroups), Groups: storyGroups}

	// Un grupo de estimaciones por grupo de historias, en el mismo orden.
	estimationGroups := make([]domain.EstimationGroup, 0, len(storyGroups))
	for _, group := range storyGroups {
		estimationGroup, err := s.estimationGroup(ctx, projectID, group.Sprint, group.Stories)
		if err != nil {
			logSectionFailure("estimations", projectID, err)
			return storyGroups, stories, unavailableEstimations
		}
		estimationGroups = append(estimationGroups, estimationGroup)
	}
	return storyGroups, stories, estimationsSection(estimationGroups)
}

// defectsSection lee el listado de defectos y arma la sección; los totales vienen de las métricas,
// no del listado. Si la lectura falla, la sección queda unavailable. Con storyGroups no nil aplica la
// instantánea de títulos (FR-021).
func (s *ReportService) defectsSection(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, detected, resolved int, storyGroups []domain.StoryGroup) domain.DefectsSection {
	defects, err := s.repo.LoadDefects(ctx, projectID, sprintID)
	if err != nil {
		logSectionFailure("defects", projectID, err)
		return domain.DefectsSection{State: domain.SectionUnavailable}
	}
	applyDetectionSnapshot(defects, storyGroups)
	return domain.DefectsSection{
		State:   domain.DefectsState(defects),
		Defects: defects,
		Totals:  &domain.DefectTotals{Detected: detected, Resolved: resolved},
	}
}

// estimationsSection arma la sección: vacía si ningún grupo tiene líneas (RN-11).
func estimationsSection(groups []domain.EstimationGroup) domain.EstimationsSection {
	lines := 0
	for _, g := range groups {
		lines += len(g.Items)
	}
	return domain.EstimationsSection{State: groupState(lines), Groups: groups}
}

// snapshotTitles indexa los títulos de las historias de cada grupo con sprint por id de sprint y de
// historia; si onlyClosed es true, solo incluye los sprints Cerrados.
func snapshotTitles(storyGroups []domain.StoryGroup, onlyClosed bool) map[uuid.UUID]map[uuid.UUID]string {
	snapshot := map[uuid.UUID]map[uuid.UUID]string{}
	for _, group := range storyGroups {
		if group.Sprint == nil || (onlyClosed && group.Sprint.Status != sprintsdomain.SprintStatusClosed) {
			continue
		}
		titles := make(map[uuid.UUID]string, len(group.Stories))
		for _, story := range group.Stories {
			titles[story.StoryID] = story.Title
		}
		snapshot[group.Sprint.ID] = titles
	}
	return snapshot
}

// snapshotTitle devuelve el título de la instantánea de una historia en un sprint, o nil si el
// sprint o la historia no están en ella.
func snapshotTitle(snapshot map[uuid.UUID]map[uuid.UUID]string, sprintID, storyID uuid.UUID) *string {
	title, ok := snapshot[sprintID][storyID]
	if !ok {
		return nil
	}
	return &title
}

// effortSection arma la sección de esfuerzo de uno o varios sprints: horas por historia en cada
// grupo, por integrante y total, todo sumado desde las mismas filas para que no se contradigan
// (research.md sección 6); si la lectura falla, la sección queda unavailable. storyGroups aporta los
// títulos de la instantánea de cada sprint.
func (s *ReportService) effortSection(ctx context.Context, projectID uuid.UUID, sprints []domain.SprintRef, storyGroups []domain.StoryGroup) domain.EffortSection {
	// Sin sprints no hay nada que leer.
	var rows []domain.EffortRow
	if len(sprints) > 0 {
		sprintIDs := make([]uuid.UUID, 0, len(sprints))
		for _, sprint := range sprints {
			sprintIDs = append(sprintIDs, sprint.ID)
		}
		var err error
		rows, err = s.repo.LoadEffort(ctx, projectID, sprintIDs)
		if err != nil {
			logSectionFailure("effort", projectID, err)
			return domain.EffortSection{State: domain.SectionUnavailable}
		}
	}

	// Títulos de la instantánea o vigentes de las historias de cada sprint.
	snapshot := snapshotTitles(storyGroups, false)

	bySprintStory := map[uuid.UUID]map[uuid.UUID]*domain.StoryEffort{}
	sprintTotals := map[uuid.UUID]int{}
	sprintRows := map[uuid.UUID]int{}
	byMember := map[uuid.UUID]*domain.MemberEffort{}
	total := 0
	for _, row := range rows {
		title := snapshotTitle(snapshot, row.SprintID, row.StoryID)
		if bySprintStory[row.SprintID] == nil {
			bySprintStory[row.SprintID] = map[uuid.UUID]*domain.StoryEffort{}
		}
		if bySprintStory[row.SprintID][row.StoryID] == nil {
			bySprintStory[row.SprintID][row.StoryID] = &domain.StoryEffort{StoryID: row.StoryID, Title: domain.StoryTitle(title, row.CurrentTitle)}
		}
		bySprintStory[row.SprintID][row.StoryID].Hundredths += row.Hundredths
		if byMember[row.UserID] == nil {
			byMember[row.UserID] = &domain.MemberEffort{UserID: row.UserID, FullName: row.FullName}
		}
		byMember[row.UserID].Hundredths += row.Hundredths
		sprintTotals[row.SprintID] += row.Hundredths
		sprintRows[row.SprintID]++
		total += row.Hundredths
	}

	// Un grupo por sprint, con las historias por horas descendentes, después título y después id.
	groups := make([]domain.EffortGroup, 0, len(sprints))
	for _, sprint := range sprints {
		storyEfforts := make([]domain.StoryEffort, 0, len(bySprintStory[sprint.ID]))
		for _, e := range bySprintStory[sprint.ID] {
			storyEfforts = append(storyEfforts, *e)
		}
		slices.SortFunc(storyEfforts, func(a, b domain.StoryEffort) int {
			return cmp.Or(cmp.Compare(b.Hundredths, a.Hundredths), cmp.Compare(a.Title, b.Title), bytes.Compare(a.StoryID[:], b.StoryID[:]))
		})
		groups = append(groups, domain.EffortGroup{
			Sprint:          sprint,
			State:           groupState(sprintRows[sprint.ID]),
			Stories:         storyEfforts,
			TotalHundredths: sprintTotals[sprint.ID],
		})
	}

	// Integrantes por nombre y después id.
	memberEfforts := make([]domain.MemberEffort, 0, len(byMember))
	for _, e := range byMember {
		memberEfforts = append(memberEfforts, *e)
	}
	slices.SortFunc(memberEfforts, func(a, b domain.MemberEffort) int {
		return cmp.Or(cmp.Compare(a.FullName, b.FullName), bytes.Compare(a.UserID[:], b.UserID[:]))
	})

	return domain.EffortSection{
		State:           domain.EffortState(rows),
		Groups:          groups,
		ByMember:        memberEfforts,
		TotalHundredths: total,
	}
}

// applyDetectionSnapshot reemplaza el título de la historia de cada defecto por el de la instantánea
// del sprint de detección cuando ese sprint está Cerrado y la historia estaba en él (FR-021).
func applyDetectionSnapshot(defects []domain.DefectLine, storyGroups []domain.StoryGroup) {
	snapshot := snapshotTitles(storyGroups, true)
	for i := range defects {
		title := snapshotTitle(snapshot, defects[i].DetectedSprint.ID, defects[i].Story.ID)
		defects[i].Story.Title = domain.StoryTitle(title, defects[i].Story.Title)
	}
}

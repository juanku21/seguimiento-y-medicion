package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	effortdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del repositorio de métricas contra la base de test (smye_test). Es una
// prueba interna del paquete para poder ejercitar la función privada readOnly (RN-02).

// metricsFixture agrupa la conexión de test y los datos sembrados.
type metricsFixture struct {
	db        *gorm.DB
	authorID  uuid.UUID
	projectID uuid.UUID
	sprintID  uuid.UUID
	closedAt  time.Time
}

// seedProject siembra un proyecto del autor con el factor indicado.
func seedProject(t *testing.T, db *gorm.DB, authorID uuid.UUID, name string, factor int) uuid.UUID {
	t.Helper()
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      authorID,
		Name:                         name,
		NameNormalized:               name,
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: factor,
		Status:                       projectsdomain.ProjectStatusInProgress,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto")
	return project.ID
}

// seedStory siembra una historia del proyecto con los puntos vigentes indicados.
func seedStory(t *testing.T, db *gorm.DB, authorID, projectID uuid.UUID, title string, points int) uuid.UUID {
	t.Helper()
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       title,
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusCompleted,
		StoryPoints: &points,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	return story.ID
}

// newMetricsFixture siembra un proyecto con factor vigente 800 y un sprint Cerrado con factor
// congelado 600, dos historias comprometidas (una completada con 5 puntos congelados y una no
// completada con 8) y tres registros de esfuerzo: 300 y 450 anclados al sprint y 999 sin sprint.
func newMetricsFixture(t *testing.T) metricsFixture {
	t.Helper()
	db, _ := testsupport.OpenDatabase(t)
	author := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        "ana@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&author).Error, "no se pudo sembrar el usuario")
	projectID := seedProject(t, db, author.ID, "portal de clientes", 800)

	closedAt := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	frozen := 600
	sprint := sprintsdomain.Sprint{
		ID:                           uuid.New(),
		ProjectID:                    projectID,
		Name:                         "Sprint 1",
		Goal:                         "Cerrar el alta de clientes",
		StartDate:                    time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		EndDate:                      time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Status:                       sprintsdomain.SprintStatusClosed,
		ClosedAt:                     &closedAt,
		HoursPerStoryPointHundredths: &frozen,
		CreatedByID:                  author.ID,
	}
	require.NoError(t, db.Create(&sprint).Error, "no se pudo sembrar el sprint")

	// Los puntos vigentes de las historias (13) difieren de los congelados para detectar lecturas
	// de la tabla equivocada.
	committed := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	var storyIDs []uuid.UUID
	for _, s := range []struct {
		title   string
		points  int
		outcome sprintsdomain.StoryOutcome
	}{
		{"Alta de clientes", 5, sprintsdomain.StoryOutcomeCompleted},
		{"Baja de clientes", 8, sprintsdomain.StoryOutcomeNotCompleted},
	} {
		storyID := seedStory(t, db, author.ID, projectID, s.title, 13)
		storyIDs = append(storyIDs, storyID)
		points, outcome, title := s.points, s.outcome, s.title
		row := sprintsdomain.SprintStory{
			ID:                 uuid.New(),
			SprintID:           sprint.ID,
			StoryID:            storyID,
			CommittedAt:        committed,
			TitleAtClose:       &title,
			StoryPointsAtClose: &points,
			Outcome:            &outcome,
		}
		require.NoError(t, db.Create(&row).Error, "no se pudo sembrar el compromiso")
	}

	entryTime := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for _, e := range []struct {
		sprintID *uuid.UUID
		hours    int
	}{{&sprint.ID, 300}, {&sprint.ID, 450}, {nil, 999}} {
		entry := effortdomain.EffortEntry{
			ID:              uuid.New(),
			StoryID:         storyIDs[0],
			UserID:          author.ID,
			SprintID:        e.sprintID,
			EntryDate:       time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC),
			Activity:        "Desarrollo",
			HoursHundredths: e.hours,
			CreatedAt:       entryTime,
			UpdatedAt:       entryTime,
		}
		require.NoError(t, db.Create(&entry).Error, "no se pudo sembrar el esfuerzo")
	}
	return metricsFixture{db: db, authorID: author.ID, projectID: projectID, sprintID: sprint.ID, closedAt: closedAt}
}

// Escenario: LoadSprint de un sprint Cerrado devuelve la instantánea y el esfuerzo (008/US1)
func TestUS1_LoadSprintCerrado(t *testing.T) {
	f := newMetricsFixture(t)

	got, currentFactor, err := NewMetricsRepository(f.db).LoadSprint(context.Background(), f.projectID, f.sprintID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, f.sprintID, got.ID)
	assert.Equal(t, "Sprint 1", got.Name)
	assert.Equal(t, sprintsdomain.SprintStatusClosed, got.Status)
	require.NotNil(t, got.ClosedAt)
	assert.True(t, f.closedAt.Equal(*got.ClosedAt), "closed_at: %v", got.ClosedAt)
	require.NotNil(t, got.FrozenFactorHundredths)
	assert.Equal(t, 600, *got.FrozenFactorHundredths)
	assert.Equal(t, 750, got.ActualHundredths, "solo suma el esfuerzo anclado al sprint")
	assert.Equal(t, 800, currentFactor, "factor vigente del proyecto")

	type story struct {
		title     string
		points    int
		completed bool
	}
	var stories []story
	for _, s := range got.Stories {
		require.NotNil(t, s.StoryPoints)
		stories = append(stories, story{s.Title, *s.StoryPoints, s.Completed})
	}
	assert.ElementsMatch(t, []story{{"Alta de clientes", 5, true}, {"Baja de clientes", 8, false}}, stories)
}

// Escenario: sprint inexistente o de otro proyecto produce ErrSprintNotFound (008/US1)
func TestUS1_LoadSprintNoEncontrado(t *testing.T) {
	f := newMetricsFixture(t)
	otherProject := seedProject(t, f.db, f.authorID, "otro proyecto", 600)

	t.Run("sprint_de_otro_proyecto", func(t *testing.T) {
		_, _, err := NewMetricsRepository(f.db).LoadSprint(context.Background(), otherProject, f.sprintID)
		assert.ErrorIs(t, err, domain.ErrSprintNotFound)
	})
	t.Run("sprint_inexistente", func(t *testing.T) {
		_, _, err := NewMetricsRepository(f.db).LoadSprint(context.Background(), f.projectID, uuid.New())
		assert.ErrorIs(t, err, domain.ErrSprintNotFound)
	})
}

// Escenario: la lectura corre en una transacción de solo lectura (008/US1, RN-02)
func TestUS1_ReadOnlyRechazaEscrituras(t *testing.T) {
	f := newMetricsFixture(t)

	err := NewMetricsRepository(f.db).readOnly(context.Background(), func(tx *gorm.DB) error {
		return tx.Exec("UPDATE projects SET name = name").Error
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "25006", "debe ser read_only_sql_transaction")

	var name string
	require.NoError(t, f.db.Raw("SELECT name FROM projects WHERE id = ?", f.projectID).Scan(&name).Error)
	assert.Equal(t, "portal de clientes", name)
}

// seedSprintWithStories siembra un sprint del proyecto con las historias indicadas. En un Cerrado
// las historias llevan la instantánea (puntos, título y resultado); en el resto, un compromiso
// abierto cuyos datos vigentes viven en la historia. Devuelve el sprint y los ID de sus historias.
func seedSprintWithStories(t *testing.T, f metricsFixture, name string, status sprintsdomain.SprintStatus,
	start time.Time, stories []seedSprintStory) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   f.projectID,
		Name:        name,
		Goal:        "Objetivo",
		StartDate:   start,
		EndDate:     start.AddDate(0, 0, 11),
		Status:      status,
		CreatedByID: f.authorID,
	}
	if status == sprintsdomain.SprintStatusClosed {
		closedAt := start.AddDate(0, 0, 11).Add(18 * time.Hour)
		frozen := 600
		sprint.ClosedAt = &closedAt
		sprint.HoursPerStoryPointHundredths = &frozen
	}
	require.NoError(t, f.db.Create(&sprint).Error, "no se pudo sembrar el sprint")

	var ids []uuid.UUID
	for _, s := range stories {
		storyID := seedStory(t, f.db, f.authorID, f.projectID, s.title, s.currentPoints)
		require.NoError(t, f.db.Exec("UPDATE user_stories SET status = ? WHERE id = ?", s.status, storyID).Error)
		ids = append(ids, storyID)
		row := sprintsdomain.SprintStory{
			ID:          uuid.New(),
			SprintID:    sprint.ID,
			StoryID:     storyID,
			CommittedAt: start.Add(9 * time.Hour),
		}
		if status == sprintsdomain.SprintStatusClosed {
			title, points, outcome := s.title, s.frozenPoints, s.outcome
			row.TitleAtClose, row.StoryPointsAtClose, row.Outcome = &title, &points, &outcome
		}
		require.NoError(t, f.db.Create(&row).Error, "no se pudo sembrar el compromiso")
	}
	return sprint.ID, ids
}

// seedSprintStory describe una historia comprometida a sembrar.
type seedSprintStory struct {
	title         string
	currentPoints int
	frozenPoints  int
	status        backlogdomain.StoryStatus
	outcome       sprintsdomain.StoryOutcome
}

// Escenario: LoadProject devuelve todos los sprints ordenados, con historias, horas y factor (008/US2)
func TestUS2_LoadProjectTodosLosSprints(t *testing.T) {
	f := newMetricsFixture(t)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

	// Sprint Activo y dos Planificados con la misma fecha de inicio (desempate por id), sembrados
	// fuera de orden para detectar un orden accidental de inserción.
	activeID, activeStories := seedSprintWithStories(t, f, "Sprint 2", sprintsdomain.SprintStatusActive, day(11, 16),
		[]seedSprintStory{
			{title: "Activa pendiente", currentPoints: 5, status: backlogdomain.StoryStatusInProgress},
			{title: "Activa completada", currentPoints: 8, status: backlogdomain.StoryStatusCompleted},
		})
	plannedA, _ := seedSprintWithStories(t, f, "Sprint 3A", sprintsdomain.SprintStatusPlanned, day(12, 14),
		[]seedSprintStory{{title: "Planificada A", currentPoints: 3, status: backlogdomain.StoryStatusPending}})
	plannedB, _ := seedSprintWithStories(t, f, "Sprint 3B", sprintsdomain.SprintStatusPlanned, day(12, 14), nil)
	require.NoError(t, f.db.Create(&effortdomain.EffortEntry{
		ID: uuid.New(), StoryID: activeStories[0], UserID: f.authorID, SprintID: &activeID,
		EntryDate: day(11, 17), Activity: "Desarrollo", HoursHundredths: 200,
	}).Error)

	// Sprint y esfuerzo de otro proyecto, que no deben aparecer.
	otherProject := seedProject(t, f.db, f.authorID, "otro proyecto", 600)
	otherSprint := sprintsdomain.Sprint{
		ID: uuid.New(), ProjectID: otherProject, Name: "Ajeno", Goal: "x", StartDate: day(10, 1),
		EndDate: day(10, 12), Status: sprintsdomain.SprintStatusPlanned, CreatedByID: f.authorID,
	}
	require.NoError(t, f.db.Create(&otherSprint).Error)

	got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), f.projectID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, f.projectID, got.ProjectID)
	assert.Equal(t, 800, got.FactorHundredths, "factor vigente del proyecto")

	// Orden start_date ASC, id ASC: Cerrado, Activo y los dos Planificados por id.
	tied := []uuid.UUID{plannedA, plannedB}
	if plannedB.String() < plannedA.String() {
		tied = []uuid.UUID{plannedB, plannedA}
	}
	var gotIDs []uuid.UUID
	for _, s := range got.Sprints {
		gotIDs = append(gotIDs, s.ID)
	}
	assert.Equal(t, append([]uuid.UUID{f.sprintID, activeID}, tied...), gotIDs)
	if len(got.Sprints) != 4 {
		return
	}

	// Cerrado: instantánea congelada, factor congelado y horas del sprint.
	closed := got.Sprints[0]
	assert.Equal(t, sprintsdomain.SprintStatusClosed, closed.Status)
	require.NotNil(t, closed.FrozenFactorHundredths)
	assert.Equal(t, 600, *closed.FrozenFactorHundredths)
	assert.Equal(t, 750, closed.ActualHundredths)
	assert.Len(t, closed.Stories, 2)

	// Activo: compromisos abiertos con puntos vigentes y estado actual de la historia.
	active := got.Sprints[1]
	assert.Equal(t, sprintsdomain.SprintStatusActive, active.Status)
	assert.Nil(t, active.FrozenFactorHundredths)
	assert.Equal(t, 200, active.ActualHundredths)
	type story struct {
		title     string
		points    int
		completed bool
	}
	var stories []story
	for _, s := range active.Stories {
		require.NotNil(t, s.StoryPoints)
		stories = append(stories, story{s.Title, *s.StoryPoints, s.Completed})
	}
	assert.ElementsMatch(t, []story{{"Activa pendiente", 5, false}, {"Activa completada", 8, true}}, stories)

	// Planificados: sus historias y sin horas.
	for _, s := range got.Sprints[2:] {
		assert.Equal(t, sprintsdomain.SprintStatusPlanned, s.Status)
		assert.Equal(t, 0, s.ActualHundredths)
		assert.NotEqual(t, otherSprint.ID, s.ID, "no debe incluir sprints de otro proyecto")
	}
}

// Escenario: LoadProject de un proyecto sin sprints devuelve la lista vacía y el factor (008/US2)
func TestUS2_LoadProjectSinSprints(t *testing.T) {
	f := newMetricsFixture(t)
	emptyProject := seedProject(t, f.db, f.authorID, "proyecto vacío", 700)

	got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), emptyProject)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.Sprints)
	assert.Equal(t, 700, got.FactorHundredths)
}

// seedUnestimatedStory siembra una historia del proyecto sin estimar (story_points NULL).
func seedUnestimatedStory(t *testing.T, db *gorm.DB, authorID, projectID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       title,
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia sin estimar")
	return story.ID
}

// seedEffort siembra un registro de esfuerzo de la historia, con o sin sprint.
func seedEffort(t *testing.T, db *gorm.DB, authorID, storyID uuid.UUID, sprintID *uuid.UUID, hours int) {
	t.Helper()
	require.NoError(t, db.Create(&effortdomain.EffortEntry{
		ID: uuid.New(), StoryID: storyID, UserID: authorID, SprintID: sprintID,
		EntryDate: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), Activity: "Desarrollo", HoursHundredths: hours,
	}).Error, "no se pudo sembrar el esfuerzo")
}

// Escenario: LoadProject suma el esfuerzo sin sprint solo de historias del proyecto (008/US3)
func TestUS3_LoadProjectEsfuerzoFueraDeSprints(t *testing.T) {
	f := newMetricsFixture(t)

	t.Run("suma_los_registros_sin_sprint_del_proyecto", func(t *testing.T) {
		// El fixture ya aporta 999 sin sprint; se suma otro registro del proyecto y uno anclado
		// (que no cuenta) y se agrega esfuerzo sin sprint de otro proyecto (que tampoco).
		story := seedStory(t, f.db, f.authorID, f.projectID, "Otra historia", 3)
		seedEffort(t, f.db, f.authorID, story, nil, 250)
		seedEffort(t, f.db, f.authorID, story, &f.sprintID, 70)
		otherProject := seedProject(t, f.db, f.authorID, "otro proyecto", 600)
		otherStory := seedStory(t, f.db, f.authorID, otherProject, "Historia ajena", 2)
		seedEffort(t, f.db, f.authorID, otherStory, nil, 5000)

		got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), f.projectID)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 1249, got.OutsideSprintsHundredths)
	})

	t.Run("sin_registros_sin_sprint_es_cero", func(t *testing.T) {
		emptyProject := seedProject(t, f.db, f.authorID, "proyecto vacío", 700)

		got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), emptyProject)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 0, got.OutsideSprintsHundredths)
	})
}

// Escenario: LoadProject cuenta las historias del proyecto sin estimar (008/US3)
func TestUS3_LoadProjectHistoriasSinEstimar(t *testing.T) {
	f := newMetricsFixture(t)

	t.Run("cuenta_solo_las_del_proyecto_con_story_points_nulo", func(t *testing.T) {
		seedUnestimatedStory(t, f.db, f.authorID, f.projectID, "Sin estimar 1")
		seedUnestimatedStory(t, f.db, f.authorID, f.projectID, "Sin estimar 2")
		otherProject := seedProject(t, f.db, f.authorID, "otro proyecto", 600)
		seedUnestimatedStory(t, f.db, f.authorID, otherProject, "Sin estimar ajena")

		got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), f.projectID)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 2, got.UnestimatedBacklogStories)
	})

	t.Run("sin_historias_sin_estimar_es_cero", func(t *testing.T) {
		// Se consulta un proyecto vacío: el del fixture ya tiene historias sin estimar del subtest anterior.
		emptyProject := seedProject(t, f.db, f.authorID, "proyecto vacío", 700)

		got, err := NewMetricsRepository(f.db).LoadProject(context.Background(), emptyProject)

		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 0, got.UnestimatedBacklogStories)
	})
}

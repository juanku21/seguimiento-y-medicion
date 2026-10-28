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

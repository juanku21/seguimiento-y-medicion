package repository_test

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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/repository"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del aislamiento por proyecto de las lecturas de esfuerzo y de sesiones de
// estimación del reporte (T050, hallazgo de seguridad de 010/US1). Corren contra la base de test y
// siembran dos proyectos, cada uno con su sprint, su historia, un registro de esfuerzo y una sesión.

// projectData reúne los ids sembrados de un proyecto.
type projectData struct {
	projectID uuid.UUID
	sprintID  uuid.UUID
	storyID   uuid.UUID
	userID    uuid.UUID
}

// seedProject siembra un usuario, un proyecto, un sprint, una historia, un registro de esfuerzo
// anclado al sprint y una sesión de estimación de la historia.
func seedProject(t *testing.T, db *gorm.DB, suffix string) projectData {
	t.Helper()
	user := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Usuario " + suffix,
		Email:        suffix + "@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&user).Error)
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      user.ID,
		Name:                         "proyecto " + suffix,
		NameNormalized:               "proyecto " + suffix,
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       projectsdomain.ProjectStatusInProgress,
	}
	require.NoError(t, db.Create(&project).Error)
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Name:        "Sprint " + suffix,
		Goal:        "Objetivo",
		StartDate:   time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Status:      sprintsdomain.SprintStatusPlanned,
		CreatedByID: user.ID,
	}
	require.NoError(t, db.Create(&sprint).Error)
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Title:       "Historia " + suffix,
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		CreatedByID: user.ID,
	}
	require.NoError(t, db.Create(&story).Error)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	entry := effortdomain.EffortEntry{
		ID:              uuid.New(),
		StoryID:         story.ID,
		UserID:          user.ID,
		SprintID:        &sprint.ID,
		EntryDate:       time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Activity:        "Desarrollo",
		HoursHundredths: 300,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, db.Create(&entry).Error)
	session := pokerdomain.EstimationSession{
		ID:             uuid.New(),
		StoryID:        story.ID,
		FacilitatorID:  user.ID,
		Status:         pokerdomain.SessionStatusOpen,
		OpenedByID:     user.ID,
		LastActivityAt: now,
	}
	require.NoError(t, db.Create(&session).Error)
	return projectData{projectID: project.ID, sprintID: sprint.ID, storyID: story.ID, userID: user.ID}
}

// Escenario: Sprint de otro proyecto (010/US1)
// Aunque se le pasen ids de sprints o historias de otro proyecto, LoadEffort y LoadEstimationSessions
// no devuelven filas que no pertenezcan al proyecto indicado.
func TestUS1_ConsultasAcotadasAlProyecto(t *testing.T) {
	db, _ := testsupport.OpenDatabase(t)
	repo := repository.NewReportRepository(db)
	a := seedProject(t, db, "a")
	b := seedProject(t, db, "b")
	ctx := context.Background()

	// Control: con los ids del propio proyecto se devuelven las filas.
	t.Run("esfuerzo del propio proyecto", func(t *testing.T) {
		rows, err := repo.LoadEffort(ctx, a.projectID, []uuid.UUID{a.sprintID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, a.sprintID, rows[0].SprintID)
	})
	t.Run("sesiones del propio proyecto", func(t *testing.T) {
		sessions, err := repo.LoadEstimationSessions(ctx, a.projectID, []uuid.UUID{a.storyID})
		require.NoError(t, err)
		assert.Len(t, sessions[a.storyID], 1)
	})

	// Con ids de otro proyecto no se devuelve nada.
	t.Run("esfuerzo de un sprint de otro proyecto", func(t *testing.T) {
		rows, err := repo.LoadEffort(ctx, a.projectID, []uuid.UUID{b.sprintID})
		require.NoError(t, err)
		assert.Empty(t, rows, "no debería filtrarse el esfuerzo de otro proyecto")
	})
	t.Run("sesiones de una historia de otro proyecto", func(t *testing.T) {
		sessions, err := repo.LoadEstimationSessions(ctx, a.projectID, []uuid.UUID{b.storyID})
		require.NoError(t, err)
		assert.Empty(t, sessions, "no deberían filtrarse las sesiones de otro proyecto")
	})

	// Mezcla de ids propios y ajenos: solo quedan los propios.
	t.Run("ids mezclados", func(t *testing.T) {
		rows, err := repo.LoadEffort(ctx, a.projectID, []uuid.UUID{a.sprintID, b.sprintID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, a.sprintID, rows[0].SprintID)
		sessions, err := repo.LoadEstimationSessions(ctx, a.projectID, []uuid.UUID{a.storyID, b.storyID})
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
		assert.Contains(t, sessions, a.storyID)
	})
}

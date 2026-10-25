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
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
)

// Pruebas de integración del puerto SprintEffort de la 004 reemplazado por la 006 y de la traducción
// de la clave foránea en SprintRepository.Delete (T017, US1). Corren contra la base de test y siembran
// directamente un sprint planned y registros de effort_entries anclados a él, para verificar que
// HasEffort lee los registros del sprint, que Delete responde ErrSprintHasEffort cuando un registro se
// ancló entre la verificación del servicio y el borrado (research.md sección 10, RC-07) y que la base
// impide borrar un sprint con registros (RN-21).

// sprintEffortFixture agrupa la conexión de test, el autor, el sprint planned y la historia sembrados.
type sprintEffortFixture struct {
	db       *gorm.DB
	authorID uuid.UUID
	sprintID uuid.UUID
	storyID  uuid.UUID
}

// newSprintEffortFixture abre la base de test vacía y siembra un usuario, un proyecto en curso suyo,
// un sprint planned del proyecto y una historia estimada del mismo proyecto.
func newSprintEffortFixture(t *testing.T) sprintEffortFixture {
	t.Helper()
	db, _ := testsupport.OpenDatabase(t)
	author := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        "ana@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&author).Error, "no se pudo sembrar el usuario")
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      author.ID,
		Name:                         "portal de clientes",
		NameNormalized:               "portal de clientes",
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       projectsdomain.ProjectStatusInProgress,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto")
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Name:        "Sprint 1",
		Goal:        "Cerrar el alta de clientes",
		StartDate:   time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Status:      sprintsdomain.SprintStatusPlanned,
		CreatedByID: author.ID,
	}
	require.NoError(t, db.Create(&sprint).Error, "no se pudo sembrar el sprint")
	storyPoints := 5
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Title:       "Alta de clientes",
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		StoryPoints: &storyPoints,
		CreatedByID: author.ID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	return sprintEffortFixture{db: db, authorID: author.ID, sprintID: sprint.ID, storyID: story.ID}
}

// seedAnchoredEntry inserta directamente en effort_entries un registro de la historia del fixture
// anclado al sprint indicado, sin pasar por el servicio, y devuelve su identificador.
func (f sprintEffortFixture) seedAnchoredEntry(t *testing.T, sprintID uuid.UUID) uuid.UUID {
	t.Helper()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	entry := effortdomain.EffortEntry{
		ID:              uuid.New(),
		StoryID:         f.storyID,
		UserID:          f.authorID,
		SprintID:        &sprintID,
		EntryDate:       time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Activity:        "Desarrollo del alta de clientes",
		HoursHundredths: 300,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, f.db.Create(&entry).Error, "no se pudo sembrar el registro de esfuerzo")
	return entry.ID
}

// Un sprint sin registros anclados no tiene esfuerzo (006/US1, RC-07).
func TestSprintEffort_SinRegistros(t *testing.T) {
	f := newSprintEffortFixture(t)

	got, err := repository.NewSprintEffort(f.db).HasEffort(context.Background(), f.sprintID)

	require.NoError(t, err)
	assert.False(t, got, "sin registros, HasEffort del sprint debe ser false")
}

// Un sprint con un registro anclado tiene esfuerzo (006/US1, RC-07).
func TestSprintEffort_ConRegistroAnclado(t *testing.T) {
	f := newSprintEffortFixture(t)
	f.seedAnchoredEntry(t, f.sprintID)

	got, err := repository.NewSprintEffort(f.db).HasEffort(context.Background(), f.sprintID)

	require.NoError(t, err)
	assert.True(t, got, "con un registro anclado, HasEffort del sprint debe ser true")
}

// Si un registro se ancla a un sprint planned después de la verificación del servicio, Delete traduce
// la violación de la clave foránea a ErrSprintHasEffort, no a un error interno, y el sprint y el
// registro siguen existiendo (006/US1, RC-07, research.md sección 10).
func TestSprintRepository_DeleteConRegistroAnclado(t *testing.T) {
	f := newSprintEffortFixture(t)
	entryID := f.seedAnchoredEntry(t, f.sprintID)

	err := repository.NewSprintRepository(f.db).Delete(context.Background(), f.sprintID)

	assert.ErrorIs(t, err, sprintsdomain.ErrSprintHasEffort)
	var sprints, entries int64
	require.NoError(t, f.db.Model(&sprintsdomain.Sprint{}).Where("id = ?", f.sprintID).Count(&sprints).Error)
	require.NoError(t, f.db.Model(&effortdomain.EffortEntry{}).Where("id = ?", entryID).Count(&entries).Error)
	assert.Equal(t, int64(1), sprints, "el sprint no debe borrarse")
	assert.Equal(t, int64(1), entries, "el registro no debe borrarse como efecto colateral")
}

// El DELETE directo en la base de un sprint con registros anclados falla por la clave foránea NO
// ACTION de effort_entries.sprint_id, y el registro sigue existiendo (006/US1, RN-21).
func TestSprintEffort_BorradoEnBaseBloqueado(t *testing.T) {
	f := newSprintEffortFixture(t)
	entryID := f.seedAnchoredEntry(t, f.sprintID)

	err := f.db.Where("id = ?", f.sprintID).Delete(&sprintsdomain.Sprint{}).Error

	assert.ErrorIs(t, err, gorm.ErrForeignKeyViolated)
	var count int64
	require.NoError(t, f.db.Model(&effortdomain.EffortEntry{}).Where("id = ?", entryID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "el registro no debe borrarse como efecto colateral")
}

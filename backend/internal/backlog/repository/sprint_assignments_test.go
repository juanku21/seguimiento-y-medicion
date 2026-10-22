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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del puerto SprintAssignments de la 003 (T029 de la 004, US2). Corren contra
// la base de test y siembran directamente las filas de user_stories, sprints y sprint_stories tal como
// las dejarían la asignación, el cierre y la baja de un compromiso, para verificar que Find lee
// CommittedToOpenSprint del compromiso abierto (outcome IS NULL) y EverAssigned de
// user_stories.ever_committed (research.md secciones 7, 9 y 17.2).

// assignmentsFixture agrupa la conexión de test, el autor, el proyecto y el puerto bajo prueba.
type assignmentsFixture struct {
	db        *gorm.DB
	authorID  uuid.UUID
	projectID uuid.UUID
	port      *repository.SprintAssignments
}

// newAssignmentsFixture abre la base de test vacía, siembra un usuario y un proyecto en curso suyo y
// compone el puerto sobre esa conexión.
func newAssignmentsFixture(t *testing.T) assignmentsFixture {
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
	return assignmentsFixture{db: db, authorID: author.ID, projectID: project.ID, port: repository.NewSprintAssignments(db)}
}

// seedStory inserta una historia pending y estimada del proyecto, con ever_committed en el valor
// indicado, y devuelve su identificador.
func (f assignmentsFixture) seedStory(t *testing.T, everCommitted bool) uuid.UUID {
	t.Helper()
	storyPoints := 5
	story := backlogdomain.UserStory{
		ID:            uuid.New(),
		ProjectID:     f.projectID,
		Title:         "Alta de clientes",
		Priority:      backlogdomain.StoryPriorityMedium,
		Status:        backlogdomain.StoryStatusPending,
		StoryPoints:   &storyPoints,
		EverCommitted: everCommitted,
		CreatedByID:   f.authorID,
	}
	require.NoError(t, f.db.Create(&story).Error, "no se pudo sembrar la historia")
	return story.ID
}

// seedSprint inserta un sprint del proyecto en el estado indicado, con el período recibido, y
// devuelve su identificador.
func (f assignmentsFixture) seedSprint(t *testing.T, status sprintsdomain.SprintStatus, start, end time.Time) uuid.UUID {
	t.Helper()
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   f.projectID,
		Name:        "Sprint " + string(status),
		Goal:        "Cerrar el alta de clientes",
		StartDate:   start,
		EndDate:     end,
		Status:      status,
		CreatedByID: f.authorID,
	}
	require.NoError(t, f.db.Create(&sprint).Error, "no se pudo sembrar el sprint")
	return sprint.ID
}

// seedCommitment inserta el compromiso de la historia en el sprint. Con outcome nil la fila es un
// compromiso abierto; con outcome, es la fila congelada que deja el cierre del sprint.
func (f assignmentsFixture) seedCommitment(t *testing.T, sprintID, storyID uuid.UUID, outcome *sprintsdomain.StoryOutcome) {
	t.Helper()
	row := sprintsdomain.SprintStory{
		ID:          uuid.New(),
		SprintID:    sprintID,
		StoryID:     storyID,
		CommittedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		Outcome:     outcome,
	}
	if outcome != nil {
		title := "Alta de clientes"
		priority := backlogdomain.StoryPriorityMedium
		storyPoints := 5
		row.TitleAtClose = &title
		row.PriorityAtClose = &priority
		row.StoryPointsAtClose = &storyPoints
	}
	require.NoError(t, f.db.Create(&row).Error, "no se pudo sembrar el compromiso")
}

// Una historia que nunca se comprometió en un sprint no está en un sprint abierto ni estuvo nunca
// en uno.
func TestSprintAssignments_SinCompromisos(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)

	got, err := f.port.Find(context.Background(), storyID)

	require.NoError(t, err)
	assert.False(t, got.CommittedToOpenSprint, "sin compromisos, CommittedToOpenSprint debe ser false")
	assert.False(t, got.EverAssigned, "sin compromisos, EverAssigned debe ser false")
}

// Una historia con un compromiso abierto (outcome IS NULL) en un sprint Planificado, marcada con
// ever_committed como lo deja la asignación, está en un sprint abierto y estuvo en un sprint
// (FR-018, RN-22).
func TestSprintAssignments_CompromisoAbierto(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, true)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusPlanned,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	f.seedCommitment(t, sprintID, storyID, nil)

	got, err := f.port.Find(context.Background(), storyID)

	require.NoError(t, err)
	assert.True(t, got.CommittedToOpenSprint, "con compromiso abierto, CommittedToOpenSprint debe ser true")
	assert.True(t, got.EverAssigned, "con compromiso abierto, EverAssigned debe ser true")
}

// Una historia cuya única fila es la congelada de un sprint Cerrado ya no está en un sprint abierto,
// pero estuvo en un sprint (RN-17, RC-06).
func TestSprintAssignments_FilaCongeladaDeSprintCerrado(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, true)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusClosed,
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
	outcome := sprintsdomain.StoryOutcomeNotCompleted
	f.seedCommitment(t, sprintID, storyID, &outcome)

	got, err := f.port.Find(context.Background(), storyID)

	require.NoError(t, err)
	assert.False(t, got.CommittedToOpenSprint, "con solo la fila congelada, CommittedToOpenSprint debe ser false")
	assert.True(t, got.EverAssigned, "con la fila congelada, EverAssigned debe ser true")
}

// Una historia quitada de su sprint no tiene fila en sprint_stories pero conserva ever_committed:
// ya no está en un sprint abierto y sigue informando que estuvo en uno (RC-06, US7-7).
func TestSprintAssignments_TrasQuitarLaHistoria(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, true)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusPlanned,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	f.seedCommitment(t, sprintID, storyID, nil)
	require.NoError(t, f.db.Where("sprint_id = ? AND story_id = ?", sprintID, storyID).
		Delete(&sprintsdomain.SprintStory{}).Error, "no se pudo quitar el compromiso")

	got, err := f.port.Find(context.Background(), storyID)

	require.NoError(t, err)
	assert.False(t, got.CommittedToOpenSprint, "tras quitarla, CommittedToOpenSprint debe ser false")
	assert.True(t, got.EverAssigned, "tras quitarla, EverAssigned debe seguir en true")
}

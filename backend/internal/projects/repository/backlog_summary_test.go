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
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
)

// Pruebas de integración del proveedor del resumen del backlog (T073, fase de integración de la
// 003 con la vista de estado de la 002, RC-07). Corren contra la base de test y siembran las
// historias con StoryRepository.Create del backlog, de modo que el proveedor lee las mismas filas
// que escribe la 003. Verifican FR-044 y FR-045 de la 002: backlog vacío informado en cero, conteo
// por estado, suma solo de las estimadas y aislamiento por proyecto.

// summaryFixture agrupa la conexión de test, el autor de las historias y el repositorio con el que
// se siembran.
type summaryFixture struct {
	db       *gorm.DB
	authorID uuid.UUID
	stories  *backlogrepository.StoryRepository
}

// newSummaryFixture abre la base de test vacía y siembra directamente un usuario que actúa como
// propietario de los proyectos y autor de las historias.
func newSummaryFixture(t *testing.T) summaryFixture {
	t.Helper()
	db, _ := testsupport.OpenDatabase(t)
	author := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        "ana@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&author).Error, "no se pudo sembrar el usuario")
	return summaryFixture{db: db, authorID: author.ID, stories: backlogrepository.NewStoryRepository(db)}
}

// seedProject inserta un proyecto en curso del autor con el nombre indicado y devuelve su
// identificador.
func (f summaryFixture) seedProject(t *testing.T, name string) uuid.UUID {
	t.Helper()
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      f.authorID,
		Name:                         name,
		NameNormalized:               name,
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       projectsdomain.ProjectStatusInProgress,
	}
	require.NoError(t, f.db.Create(&project).Error, "no se pudo sembrar el proyecto")
	return project.ID
}

// seedStory crea con StoryRepository.Create una historia del proyecto en el estado indicado y con
// los Story Points dados; nil la deja sin estimar.
func (f summaryFixture) seedStory(t *testing.T, projectID uuid.UUID, status backlogdomain.StoryStatus, storyPoints *int) {
	t.Helper()
	story := &backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       "Historia de prueba",
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      status,
		StoryPoints: storyPoints,
		CreatedByID: f.authorID,
	}
	require.NoError(t, f.stories.Create(context.Background(), story, nil), "no se pudo sembrar la historia")
}

// points devuelve un puntero al valor, para expresar una historia estimada.
func points(value int) *int {
	return &value
}

// Un proyecto sin historias informa CountsByState vacío, no nulo, y EstimatedStoryPoints en 0
// (FR-044 de la 002).
func TestBacklogSummaryProvider_SinHistorias(t *testing.T) {
	f := newSummaryFixture(t)
	projectID := f.seedProject(t, "portal de clientes")

	got, err := repository.NewBacklogSummaryProvider(f.db).Summarize(context.Background(), projectID)

	require.NoError(t, err)
	require.NotNil(t, got.CountsByState, "CountsByState debería ser un mapa vacío, no nil")
	assert.Empty(t, got.CountsByState)
	assert.Equal(t, 0, got.EstimatedStoryPoints)
}

// Con historias pending, in_progress y completed, el resumen informa la cantidad de cada estado
// presente y suma los Story Points de las estimadas.
func TestBacklogSummaryProvider_CuentaPorEstado(t *testing.T) {
	f := newSummaryFixture(t)
	projectID := f.seedProject(t, "portal de clientes")
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, points(3))
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, points(5))
	f.seedStory(t, projectID, backlogdomain.StoryStatusInProgress, points(8))
	f.seedStory(t, projectID, backlogdomain.StoryStatusCompleted, points(2))
	f.seedStory(t, projectID, backlogdomain.StoryStatusCompleted, points(1))
	f.seedStory(t, projectID, backlogdomain.StoryStatusCompleted, points(13))

	got, err := repository.NewBacklogSummaryProvider(f.db).Summarize(context.Background(), projectID)

	require.NoError(t, err)
	assert.Equal(t, map[string]int{"pending": 2, "in_progress": 1, "completed": 3}, got.CountsByState)
	assert.Equal(t, 3+5+8+2+1+13, got.EstimatedStoryPoints)
}

// Solo aparecen las claves de los estados que tienen historias: un backlog con historias pending
// únicamente no informa in_progress ni completed.
func TestBacklogSummaryProvider_SoloEstadosPresentes(t *testing.T) {
	f := newSummaryFixture(t)
	projectID := f.seedProject(t, "portal de clientes")
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, points(5))

	got, err := repository.NewBacklogSummaryProvider(f.db).Summarize(context.Background(), projectID)

	require.NoError(t, err)
	assert.Equal(t, map[string]int{"pending": 1}, got.CountsByState)
	assert.Equal(t, 5, got.EstimatedStoryPoints)
}

// Las historias sin estimar se cuentan en su estado pero no suman, y las estimadas en 0 suman 0
// (FR-045 de la 002), incluso cuando un estado tiene solo historias sin estimar.
func TestBacklogSummaryProvider_SinEstimarYCero(t *testing.T) {
	f := newSummaryFixture(t)
	projectID := f.seedProject(t, "portal de clientes")
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, nil)
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, points(0))
	f.seedStory(t, projectID, backlogdomain.StoryStatusPending, points(8))
	f.seedStory(t, projectID, backlogdomain.StoryStatusInProgress, nil)
	f.seedStory(t, projectID, backlogdomain.StoryStatusCompleted, points(0))

	got, err := repository.NewBacklogSummaryProvider(f.db).Summarize(context.Background(), projectID)

	require.NoError(t, err)
	assert.Equal(t, map[string]int{"pending": 3, "in_progress": 1, "completed": 1}, got.CountsByState)
	assert.Equal(t, 8, got.EstimatedStoryPoints)
}

// Las historias de otro proyecto no cuentan ni suman en el resumen del proyecto consultado.
func TestBacklogSummaryProvider_IgnoraOtrosProyectos(t *testing.T) {
	f := newSummaryFixture(t)
	projectID := f.seedProject(t, "portal de clientes")
	otherID := f.seedProject(t, "app de ventas")
	f.seedStory(t, projectID, backlogdomain.StoryStatusInProgress, points(3))
	f.seedStory(t, otherID, backlogdomain.StoryStatusInProgress, points(20))
	f.seedStory(t, otherID, backlogdomain.StoryStatusCompleted, points(40))
	f.seedStory(t, otherID, backlogdomain.StoryStatusPending, nil)

	got, err := repository.NewBacklogSummaryProvider(f.db).Summarize(context.Background(), projectID)

	require.NoError(t, err)
	assert.Equal(t, map[string]int{"in_progress": 1}, got.CountsByState)
	assert.Equal(t, 3, got.EstimatedStoryPoints)
}

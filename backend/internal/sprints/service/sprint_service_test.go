package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas unitarias obligatorias de la creación de sprints en el servicio (T012; FR-006, FR-060,
// RN-01, RN-19). El repositorio y el puerto de esfuerzo se reemplazan por dobles de testify y el
// reloj queda fijo, de modo que cada prueba aísla la regla del servicio sin tocar la base de datos.
// No llevan la etiqueta de un escenario de la spec: plan.md asigna los nombres TestUS1_* a las
// pruebas de integración de handler_test.go (T013), igual que T010 y T011.

// mockSprintRepository es el doble de testify de SprintRepository. Implementa la interfaz completa
// para compilar; las pruebas de creación solo configuran Create y FindDetail.
type mockSprintRepository struct {
	mock.Mock
}

func (m *mockSprintRepository) FindAccessible(ctx context.Context, sprintID, userID uuid.UUID) (*domain.SprintAccess, error) {
	args := m.Called(ctx, sprintID, userID)
	access, _ := args.Get(0).(*domain.SprintAccess)
	return access, args.Error(1)
}

func (m *mockSprintRepository) Create(ctx context.Context, sprint *domain.Sprint) error {
	args := m.Called(ctx, sprint)
	return args.Error(0)
}

func (m *mockSprintRepository) Update(ctx context.Context, sprintID uuid.UUID, input domain.SprintInput) error {
	args := m.Called(ctx, sprintID, input)
	return args.Error(0)
}

func (m *mockSprintRepository) Delete(ctx context.Context, sprintID uuid.UUID) error {
	args := m.Called(ctx, sprintID)
	return args.Error(0)
}

func (m *mockSprintRepository) Start(ctx context.Context, sprintID uuid.UUID, now time.Time) error {
	args := m.Called(ctx, sprintID, now)
	return args.Error(0)
}

func (m *mockSprintRepository) Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error {
	args := m.Called(ctx, sprintID, closedBy, now)
	return args.Error(0)
}

func (m *mockSprintRepository) AssignStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID, now time.Time) error {
	args := m.Called(ctx, sprintID, storyIDs, now)
	return args.Error(0)
}

func (m *mockSprintRepository) RemoveStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID) error {
	args := m.Called(ctx, sprintID, storyIDs)
	return args.Error(0)
}

func (m *mockSprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error {
	args := m.Called(ctx, storyID, to, now)
	return args.Error(0)
}

func (m *mockSprintRepository) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) ([]domain.SprintListItem, error) {
	args := m.Called(ctx, projectID, statuses)
	items, _ := args.Get(0).([]domain.SprintListItem)
	return items, args.Error(1)
}

func (m *mockSprintRepository) HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func (m *mockSprintRepository) FindDetail(ctx context.Context, sprintID uuid.UUID) (*domain.SprintDetail, error) {
	args := m.Called(ctx, sprintID)
	detail, _ := args.Get(0).(*domain.SprintDetail)
	return detail, args.Error(1)
}

func (m *mockSprintRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	args := m.Called(ctx, storyID)
	items, _ := args.Get(0).([]domain.StorySprintHistoryItem)
	return items, args.Error(1)
}

// mockSprintEffort es el doble de testify del puerto SprintEffort. La creación no lo consulta: sin
// expectativas, cualquier llamada falla la prueba.
type mockSprintEffort struct {
	mock.Mock
}

func (m *mockSprintEffort) HasEffort(ctx context.Context, sprintID uuid.UUID) (bool, error) {
	args := m.Called(ctx, sprintID)
	return args.Bool(0), args.Error(1)
}

// serviceFixedNow es el instante fijo del reloj inyectado en todas las pruebas del servicio.
var serviceFixedNow = time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

// serviceDoubles agrupa los dobles del servicio para configurar expectativas y verificarlas.
type serviceDoubles struct {
	repo   *mockSprintRepository
	effort *mockSprintEffort
}

// newSprintServiceWithDoubles compone el servicio con los dobles, la zona UTC y el reloj fijo. Los
// dobles se atan a t para que una llamada inesperada sea un fallo de la prueba y no un panic.
func newSprintServiceWithDoubles(t *testing.T) (*SprintService, serviceDoubles) {
	t.Helper()
	d := serviceDoubles{repo: &mockSprintRepository{}, effort: &mockSprintEffort{}}
	d.repo.Test(t)
	d.effort.Test(t)
	svc := NewSprintService(d.repo, d.effort, time.UTC, func() time.Time { return serviceFixedNow })
	return svc, d
}

// serviceDay construye una fecha de 2026 a medianoche UTC, como se representan las fechas de los
// sprints en el dominio.
func serviceDay(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 0, 0, 0, 0, time.UTC)
}

// serviceInput arma la entrada ya validada del caso normal de la spec (US1, escenario 1).
func serviceInput() domain.SprintInput {
	return domain.SprintInput{
		Name:      "Sprint 1",
		Goal:      "Permitir que el equipo registre y estime su backlog",
		StartDate: serviceDay(time.October, 1),
		EndDate:   serviceDay(time.October, 14),
	}
}

// expectSuccessfulCreate configura el repositorio para aceptar la creación, guarda en captured el
// sprint que recibe y devuelve como detalle uno armado con ese mismo sprint, de modo que la prueba
// pueda verificar que el servicio pide el detalle del sprint que acaba de crear.
func expectSuccessfulCreate(d serviceDoubles, captured **domain.Sprint) {
	d.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sprint")).
		Run(func(args mock.Arguments) {
			*captured = args.Get(1).(*domain.Sprint)
		}).
		Return(nil).Once()
	detail := &domain.SprintDetail{Stories: []domain.SprintStoryView{}}
	d.repo.On("FindDetail", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Run(func(args mock.Arguments) {
			detail.Sprint.ID = args.Get(1).(uuid.UUID)
		}).
		Return(detail, nil).Once()
}

// FR-006: el sprint nace planned, con un UUID nuevo, en el proyecto de la ruta y con los datos de
// la entrada, sin Story Points comprometidos, sin inicio y sin datos de cierre; la respuesta es el
// detalle del sprint recién creado.
func TestSprintService_Create_NacePlanificado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID, actorID := uuid.New(), uuid.New()
	var sprint *domain.Sprint
	expectSuccessfulCreate(d, &sprint)
	input := serviceInput()

	view, err := svc.Create(context.Background(), projectID, actorID, input)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, sprint, "el servicio no entregó el sprint al repositorio")
	is := assert.New(t)
	is.NotEqual(uuid.Nil, sprint.ID)
	is.Equal(projectID, sprint.ProjectID)
	is.Equal(domain.SprintStatusPlanned, sprint.Status)
	is.Equal(input.Name, sprint.Name)
	is.Equal(input.Goal, sprint.Goal)
	is.Equal(input.StartDate, sprint.StartDate)
	is.Equal(input.EndDate, sprint.EndDate)
	is.Nil(sprint.CommittedStoryPoints)
	is.Nil(sprint.StartedAt)
	is.Nil(sprint.ClosedAt)
	is.Nil(sprint.ClosedByID)
	is.Nil(sprint.HoursPerStoryPointHundredths)
	require.NotNil(t, view.Detail)
	is.Equal(sprint.ID, view.Detail.Sprint.ID, "el detalle devuelto no es el del sprint creado")
	d.repo.AssertExpectations(t)
	d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
}

// FR-006: cada creación genera un UUID nuevo, de modo que dos sprints creados con la misma entrada
// no comparten identificador.
func TestSprintService_Create_UUIDNuevoPorSprint(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID, actorID := uuid.New(), uuid.New()
	var first, second *domain.Sprint
	expectSuccessfulCreate(d, &first)

	_, err := svc.Create(context.Background(), projectID, actorID, serviceInput())
	require.NoError(t, err)
	expectSuccessfulCreate(d, &second)
	_, err = svc.Create(context.Background(), projectID, actorID, serviceInput())
	require.NoError(t, err)

	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotEqual(t, first.ID, second.ID)
}

// FR-060: el autor del sprint es el actor que recibe el servicio (el titular de la sesión), no un
// dato de la entrada, que no tiene campo de autor.
func TestSprintService_Create_AutorEsElActor(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID, actorID := uuid.New(), uuid.New()
	var sprint *domain.Sprint
	expectSuccessfulCreate(d, &sprint)

	_, err := svc.Create(context.Background(), projectID, actorID, serviceInput())

	require.NoError(t, err)
	require.NotNil(t, sprint, "el servicio no entregó el sprint al repositorio")
	assert.Equal(t, actorID, sprint.CreatedByID)
}

// RN-01: cualquier integrante crea sprints, sea o no el propietario del proyecto. El servicio no
// consulta la propiedad: el propietario y otro integrante crean en el mismo proyecto y cada sprint
// queda atribuido a quien lo creó.
func TestSprintService_Create_IntegranteNoPropietario(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID := uuid.New()
	ownerID, memberID := uuid.New(), uuid.New()
	var ownerSprint, memberSprint *domain.Sprint

	expectSuccessfulCreate(d, &ownerSprint)
	_, err := svc.Create(context.Background(), projectID, ownerID, serviceInput())
	require.NoError(t, err)

	memberInput := serviceInput()
	memberInput.Name = "Sprint 2"
	memberInput.StartDate = serviceDay(time.October, 14)
	memberInput.EndDate = serviceDay(time.October, 28)
	expectSuccessfulCreate(d, &memberSprint)
	view, err := svc.Create(context.Background(), projectID, memberID, memberInput)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, ownerSprint)
	require.NotNil(t, memberSprint)
	assert.Equal(t, ownerID, ownerSprint.CreatedByID)
	assert.Equal(t, memberID, memberSprint.CreatedByID)
	assert.Equal(t, projectID, memberSprint.ProjectID)
	d.repo.AssertExpectations(t)
}

// RN-19: el repositorio rechaza la creación en un proyecto Finalizado con ErrSprintsReadOnly; el
// servicio lo propaga reconocible con errors.Is, no devuelve vista y no pide el detalle.
func TestSprintService_Create_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	d.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sprint")).
		Return(domain.ErrSprintsReadOnly).Once()

	view, err := svc.Create(context.Background(), uuid.New(), uuid.New(), serviceInput())

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertExpectations(t)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
}

// FR-005: el repositorio rechaza un período superpuesto con SprintOverlapError; el servicio lo
// propaga de forma que delivery lo extraiga con errors.As y conserve el sprint en conflicto (su
// identificador y su nombre) para la respuesta 409.
func TestSprintService_Create_Superposicion(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	conflicting := domain.ConflictingSprint{ID: uuid.New(), Name: "Sprint 1"}
	d.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sprint")).
		Return(&domain.SprintOverlapError{Sprint: conflicting}).Once()

	view, err := svc.Create(context.Background(), uuid.New(), uuid.New(), serviceInput())

	var overlap *domain.SprintOverlapError
	require.ErrorAs(t, err, &overlap)
	assert.Equal(t, conflicting, overlap.Sprint)
	assert.Nil(t, view)
	d.repo.AssertExpectations(t)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
}

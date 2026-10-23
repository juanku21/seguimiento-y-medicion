package service

import (
	"context"
	"strconv"
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

func (m *mockSprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) (*domain.SprintStory, error) {
	args := m.Called(ctx, storyID, to, now)
	commitment, _ := args.Get(0).(*domain.SprintStory)
	return commitment, args.Error(1)
}

func (m *mockSprintRepository) FindOpenCommitment(ctx context.Context, storyID uuid.UUID) (*domain.SprintStory, error) {
	args := m.Called(ctx, storyID)
	commitment, _ := args.Get(0).(*domain.SprintStory)
	return commitment, args.Error(1)
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

// Pruebas unitarias obligatorias de la asignación y la baja de historias en el servicio (T027;
// RN-09, RN-18, RN-19, RN-21, FR-017 a FR-023). Siguen el orden de research.md sección 18: la solo
// lectura del proyecto Finalizado, que llega en SprintAccess, se evalúa antes que cualquier otra
// regla y sin tocar el repositorio; después, el estado del sprint leído con FindDetail. Los
// rechazos que el repositorio decide dentro de su transacción (historias no admitidas y quitar una
// completada) se propagan reconocibles con errors.Is y errors.As. Igual que las de creación, no
// llevan etiqueta de escenario: los TestUS2_* son las pruebas de integración de T028.

// sprintAccessFor arma el acceso que deja RequireSprintAccess para un sprint de un proyecto con el
// estado de finalización indicado.
func sprintAccessFor(finished bool) *domain.SprintAccess {
	return &domain.SprintAccess{SprintID: uuid.New(), ProjectID: uuid.New(), ProjectFinished: finished}
}

// sprintDetailWithStatus arma el detalle de un sprint en el estado indicado, como lo devuelve
// FindDetail.
func sprintDetailWithStatus(sprintID uuid.UUID, status domain.SprintStatus) *domain.SprintDetail {
	detail := &domain.SprintDetail{Stories: []domain.SprintStoryView{}}
	detail.Sprint.ID = sprintID
	detail.Sprint.Status = status
	return detail
}

// RN-09: en un sprint planned o active la asignación se delega al repositorio con el sprint del
// acceso, el lote completo y el instante del reloj inyectado, y la respuesta es el detalle
// actualizado del sprint.
func TestSprintService_AssignStories_SprintAbierto(t *testing.T) {
	for _, status := range []domain.SprintStatus{domain.SprintStatusPlanned, domain.SprintStatusActive} {
		t.Run(string(status), func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			access := sprintAccessFor(false)
			storyIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
			d.repo.On("FindDetail", mock.Anything, access.SprintID).
				Return(sprintDetailWithStatus(access.SprintID, status), nil)
			d.repo.On("AssignStories", mock.Anything, access.SprintID, storyIDs, serviceFixedNow).
				Return(nil).Once()

			view, err := svc.AssignStories(context.Background(), access, storyIDs)

			require.NoError(t, err)
			require.NotNil(t, view)
			require.NotNil(t, view.Detail)
			assert.Equal(t, access.SprintID, view.Detail.Sprint.ID)
			d.repo.AssertExpectations(t)
		})
	}
}

// RN-19: con el proyecto Finalizado la asignación se rechaza con ErrSprintsReadOnly sin leer el
// sprint ni escribir: la solo lectura vuelve irrelevante cualquier otra regla.
func TestSprintService_AssignStories_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	view, err := svc.AssignStories(context.Background(), access, []uuid.UUID{uuid.New()})

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "AssignStories", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// RN-09, RN-18: un sprint closed no admite historias; la asignación se rechaza con ErrSprintClosed
// y no llega al repositorio.
func TestSprintService_AssignStories_SprintCerrado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusClosed), nil).Once()

	view, err := svc.AssignStories(context.Background(), access, []uuid.UUID{uuid.New()})

	require.ErrorIs(t, err, domain.ErrSprintClosed)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "AssignStories", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// FR-019, SC-007: si el repositorio rechaza el lote con StoriesNotAdmittedError, el servicio lo
// propaga extraíble con errors.As y con todos los problemas intactos, incluida la historia con
// varios motivos, y no vuelve a leer el detalle después del rechazo.
func TestSprintService_AssignStories_HistoriasNoAdmitidas(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	notReadyID, completedID, multiID := uuid.New(), uuid.New(), uuid.New()
	storyIDs := []uuid.UUID{notReadyID, completedID, multiID}
	elsewhere := &domain.ConflictingSprint{ID: uuid.New(), Name: "Sprint 2"}
	problems := []domain.AdmissionProblem{
		{StoryID: notReadyID, Reason: domain.AdmissionReasonNotReady, Missing: []string{"story_points"}},
		{StoryID: completedID, Reason: domain.AdmissionReasonCompleted},
		{StoryID: multiID, Reason: domain.AdmissionReasonNotReady, Missing: []string{"story_points", "acceptance_criteria"}},
		{StoryID: multiID, Reason: domain.AdmissionReasonCommittedElsewhere, Sprint: elsewhere},
	}
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil).Once()
	d.repo.On("AssignStories", mock.Anything, access.SprintID, storyIDs, serviceFixedNow).
		Return(&domain.StoriesNotAdmittedError{Problems: problems}).Once()

	view, err := svc.AssignStories(context.Background(), access, storyIDs)

	var notAdmitted *domain.StoriesNotAdmittedError
	require.ErrorAs(t, err, &notAdmitted)
	assert.Equal(t, problems, notAdmitted.Problems)
	assert.Nil(t, view)
	d.repo.AssertExpectations(t)
}

// RN-09: en un sprint planned o active la baja se delega al repositorio con el sprint del acceso y
// el lote completo, y la respuesta es el detalle actualizado del sprint.
func TestSprintService_RemoveStories_SprintAbierto(t *testing.T) {
	for _, status := range []domain.SprintStatus{domain.SprintStatusPlanned, domain.SprintStatusActive} {
		t.Run(string(status), func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			access := sprintAccessFor(false)
			storyIDs := []uuid.UUID{uuid.New(), uuid.New()}
			d.repo.On("FindDetail", mock.Anything, access.SprintID).
				Return(sprintDetailWithStatus(access.SprintID, status), nil)
			d.repo.On("RemoveStories", mock.Anything, access.SprintID, storyIDs).
				Return(nil).Once()

			view, err := svc.RemoveStories(context.Background(), access, storyIDs)

			require.NoError(t, err)
			require.NotNil(t, view)
			require.NotNil(t, view.Detail)
			assert.Equal(t, access.SprintID, view.Detail.Sprint.ID)
			d.repo.AssertExpectations(t)
		})
	}
}

// RN-19: con el proyecto Finalizado la baja se rechaza con ErrSprintsReadOnly sin leer el sprint ni
// escribir.
func TestSprintService_RemoveStories_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	view, err := svc.RemoveStories(context.Background(), access, []uuid.UUID{uuid.New()})

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "RemoveStories", mock.Anything, mock.Anything, mock.Anything)
}

// RN-09, RN-18: de un sprint closed no se quitan historias; la baja se rechaza con ErrSprintClosed
// y no llega al repositorio.
func TestSprintService_RemoveStories_SprintCerrado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusClosed), nil).Once()

	view, err := svc.RemoveStories(context.Background(), access, []uuid.UUID{uuid.New()})

	require.ErrorIs(t, err, domain.ErrSprintClosed)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "RemoveStories", mock.Anything, mock.Anything, mock.Anything)
}

// RN-21: si el lote incluye una historia completed, el repositorio rechaza la baja con
// ErrCompletedStoryRemoval y el servicio lo propaga reconocible con errors.Is, sin respuesta de
// detalle.
func TestSprintService_RemoveStories_HistoriaCompletada(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	storyIDs := []uuid.UUID{uuid.New(), uuid.New()}
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusActive), nil).Once()
	d.repo.On("RemoveStories", mock.Anything, access.SprintID, storyIDs).
		Return(domain.ErrCompletedStoryRemoval).Once()

	view, err := svc.RemoveStories(context.Background(), access, storyIDs)

	require.ErrorIs(t, err, domain.ErrCompletedStoryRemoval)
	assert.Nil(t, view)
	d.repo.AssertExpectations(t)
}

// Pruebas unitarias obligatorias del inicio de un sprint en el servicio (T040; RN-02, RN-06, RN-07,
// RN-19, FR-026 a FR-032). Siguen el orden de data-model.md sección 4 y research.md sección 18: la
// solo lectura del proyecto Finalizado se evalúa primero y sin tocar el repositorio; después, el
// estado del sprint leído con FindDetail (closed → ErrSprintClosed, active → la transición no
// permitida). El repositorio decide dentro de su transacción si hay otro sprint activo. El servicio
// no tiene dependencia hacia proyectos: los dobles fallan ante cualquier llamada no configurada, lo
// que prueba que iniciar no dispara otra operación (RN-07). Igual que las anteriores, no llevan
// etiqueta de escenario: los TestUS3_* son las pruebas de integración de T041.

// expectSuccessfulStart configura el detalle planned del sprint, que el servicio lee antes y después
// del inicio, y el repositorio para aceptar el inicio con el instante del reloj fijo.
func expectSuccessfulStart(d serviceDoubles, sprintID uuid.UUID) {
	d.repo.On("FindDetail", mock.Anything, sprintID).
		Return(sprintDetailWithStatus(sprintID, domain.SprintStatusPlanned), nil)
	d.repo.On("Start", mock.Anything, sprintID, serviceFixedNow).Return(nil).Once()
}

// FR-026, FR-028: un sprint planned se inicia delegando en el repositorio con el sprint del acceso
// y el instante del reloj inyectado, que es el que queda como momento real de inicio; la respuesta
// es el detalle del sprint.
func TestSprintService_Start_InstanteDelReloj(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	expectSuccessfulStart(d, access.SprintID)

	view, err := svc.Start(context.Background(), access)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.Detail)
	assert.Equal(t, access.SprintID, view.Detail.Sprint.ID)
	d.repo.AssertCalled(t, "Start", mock.Anything, access.SprintID, serviceFixedNow)
	d.repo.AssertExpectations(t)
}

// RN-19: con el proyecto Finalizado el inicio se rechaza con ErrSprintsReadOnly sin leer el sprint
// ni escribir.
func TestSprintService_Start_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	view, err := svc.Start(context.Background(), access)

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Start", mock.Anything, mock.Anything, mock.Anything)
}

// RN-02: un sprint que ya está active no se vuelve a iniciar; el inicio se rechaza con
// ErrInvalidSprintTransition y no llega al repositorio.
func TestSprintService_Start_SprintYaActivo(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusActive), nil).Once()

	view, err := svc.Start(context.Background(), access)

	require.ErrorIs(t, err, domain.ErrInvalidSprintTransition)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Start", mock.Anything, mock.Anything, mock.Anything)
}

// RN-02, RN-18: un sprint closed es definitivo; el inicio se rechaza con ErrSprintClosed (no con la
// transición genérica) y no llega al repositorio.
func TestSprintService_Start_SprintCerrado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusClosed), nil).Once()

	view, err := svc.Start(context.Background(), access)

	require.ErrorIs(t, err, domain.ErrSprintClosed)
	assert.NotErrorIs(t, err, domain.ErrInvalidSprintTransition)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Start", mock.Anything, mock.Anything, mock.Anything)
}

// RN-06: si el proyecto ya tiene otro sprint activo, el repositorio rechaza el inicio con
// ActiveSprintExistsError; el servicio lo propaga extraíble con errors.As, con el identificador y el
// nombre de ese sprint para la respuesta 409, y no vuelve a leer el detalle.
func TestSprintService_Start_OtroSprintActivo(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	active := domain.ConflictingSprint{ID: uuid.New(), Name: "Sprint 1"}
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil).Once()
	d.repo.On("Start", mock.Anything, access.SprintID, serviceFixedNow).
		Return(&domain.ActiveSprintExistsError{Sprint: active}).Once()

	view, err := svc.Start(context.Background(), access)

	var exists *domain.ActiveSprintExistsError
	require.ErrorAs(t, err, &exists)
	assert.Equal(t, active, exists.Sprint)
	assert.Nil(t, view)
	d.repo.AssertExpectations(t)
}

// RN-07: iniciar un sprint no cambia el estado del proyecto. El servicio solo lee el sprint e
// invoca Start una vez, sin consultar el puerto de esfuerzo. Los dobles atados a t fallan ante
// cualquier llamada no configurada (m.Called), de modo que otra operación del repositorio haría
// fallar la prueba; y el servicio no recibe dependencia alguna del módulo de proyectos.
func TestSprintService_Start_NoTocaElProyecto(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	expectSuccessfulStart(d, access.SprintID)

	_, err := svc.Start(context.Background(), access)

	require.NoError(t, err)
	d.repo.AssertExpectations(t)
	d.repo.AssertNumberOfCalls(t, "Start", 1)
	d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
}

// Pruebas unitarias obligatorias del avance de las historias en el servicio (T054; RN-11, SC-011,
// FR-033 a FR-038). Las reglas del avance (proyecto Finalizado, compromiso en el sprint activo,
// sprint cerrado y transición admitida) las decide el repositorio dentro de su transacción, con la
// historia bloqueada; el servicio solo le entrega el instante del reloj inyectado y propaga sus
// rechazos reconocibles con errors.Is. Igual que las anteriores, no llevan etiqueta de escenario:
// los TestUS4_* son las pruebas de integración de T055.

// SC-011, FR-035: el avance se delega al repositorio con la historia, el estado destino y el
// instante del reloj inyectado, que es el que queda como fecha de completado; cada transición
// admitida llega intacta y sin error.
func TestSprintService_ChangeStoryProgress_InstanteDelReloj(t *testing.T) {
	targets := []backlogdomain.StoryStatus{backlogdomain.StoryStatusInProgress, backlogdomain.StoryStatusCompleted}
	for _, to := range targets {
		t.Run(string(to), func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			storyID := uuid.New()
			d.repo.On("ChangeStoryProgress", mock.Anything, storyID, to, serviceFixedNow).
				Return(nil, nil).Once()

			_, err := svc.ChangeStoryProgress(context.Background(), storyID, to)

			require.NoError(t, err)
			d.repo.AssertExpectations(t)
			d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
		})
	}
}

// RN-11, RN-18, RN-19: los rechazos que el repositorio decide con la historia bloqueada se propagan
// reconocibles con errors.Is: historia fuera del sprint activo, sprint cerrado, transición no
// admitida y proyecto Finalizado. El servicio invoca el repositorio una sola vez con el instante del
// reloj y no traduce el error a otro.
func TestSprintService_ChangeStoryProgress_PropagaRechazos(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"fuera del sprint activo", domain.ErrProgressOutsideActiveSprint},
		{"sprint cerrado", domain.ErrSprintClosed},
		{"transicion no admitida", domain.ErrInvalidStoryTransition},
		{"proyecto finalizado", domain.ErrSprintsReadOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			storyID := uuid.New()
			to := backlogdomain.StoryStatusCompleted
			d.repo.On("ChangeStoryProgress", mock.Anything, storyID, to, serviceFixedNow).
				Return(nil, tc.err).Once()

			_, err := svc.ChangeStoryProgress(context.Background(), storyID, to)

			require.ErrorIs(t, err, tc.err)
			d.repo.AssertExpectations(t)
			d.repo.AssertNumberOfCalls(t, "ChangeStoryProgress", 1)
		})
	}
}

// Sin etiqueta de escenario: el avance devuelve el compromiso que el repositorio releyó dentro de su
// transacción, con su sprint y su fecha de completado, aunque un cierre se haya confirmado justo
// después y el compromiso ya no esté abierto (research.md sección 13, T109). El doble de
// FindOpenCommitment responde nil, como lo haría tras el cierre intercalado: el servicio no debe
// releer fuera de la transacción ni convertir esa ausencia en un error interno.
func TestSprintService_ChangeStoryProgress_CompromisoDeLaTransaccion(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	storyID, sprintID := uuid.New(), uuid.New()
	completedAt := serviceFixedNow
	committed := &domain.SprintStory{
		ID:          uuid.New(),
		SprintID:    sprintID,
		StoryID:     storyID,
		CommittedAt: serviceDay(time.October, 1),
		CompletedAt: &completedAt,
	}
	to := backlogdomain.StoryStatusCompleted
	d.repo.On("ChangeStoryProgress", mock.Anything, storyID, to, serviceFixedNow).
		Return(committed, nil).Once()
	d.repo.On("FindOpenCommitment", mock.Anything, storyID).Return(nil, nil).Maybe()

	commitment, err := svc.ChangeStoryProgress(context.Background(), storyID, to)

	require.NoError(t, err)
	require.NotNil(t, commitment, "el avance debe devolver el compromiso leído en la transacción")
	assert.Equal(t, sprintID, commitment.SprintID)
	require.NotNil(t, commitment.CompletedAt)
	assert.True(t, serviceFixedNow.Equal(*commitment.CompletedAt))
	d.repo.AssertExpectations(t)
	d.repo.AssertNotCalled(t, "FindOpenCommitment", mock.Anything, mock.Anything)
}

// Pruebas unitarias obligatorias del cierre de un sprint en el servicio (T063; RN-02, RN-07, RN-18,
// RN-19, FR-039 a FR-047). Siguen la tabla de operaciones por estado de data-model.md sección 4: la
// solo lectura del proyecto Finalizado se evalúa primero y sin tocar el repositorio; después, el
// estado del sprint leído con FindDetail (planned → ErrSprintNotActive, closed → ErrSprintClosed).
// Superados los controles, el servicio delega en el repositorio con el actor como closedBy y el
// instante del reloj inyectado. Igual que las anteriores, no llevan etiqueta de escenario: los
// TestUS5_* son las pruebas de integración de T064.

// expectSuccessfulClose configura el detalle active del sprint, que el servicio lee antes y después
// del cierre, y el repositorio para aceptar el cierre del actor con el instante del reloj fijo.
func expectSuccessfulClose(d serviceDoubles, sprintID, actorID uuid.UUID) {
	d.repo.On("FindDetail", mock.Anything, sprintID).
		Return(sprintDetailWithStatus(sprintID, domain.SprintStatusActive), nil)
	d.repo.On("Close", mock.Anything, sprintID, actorID, serviceFixedNow).Return(nil).Once()
}

// RN-19: con el proyecto Finalizado el cierre se rechaza con ErrSprintsReadOnly sin leer el sprint
// ni escribir.
func TestSprintService_Close_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	view, err := svc.Close(context.Background(), access, uuid.New())

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Close", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// RN-02: solo se cierra un sprint Activo; uno planned se rechaza con ErrSprintNotActive (no con la
// transición genérica) y no llega al repositorio.
func TestSprintService_Close_SprintPlanificado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil).Once()

	view, err := svc.Close(context.Background(), access, uuid.New())

	require.ErrorIs(t, err, domain.ErrSprintNotActive)
	assert.NotErrorIs(t, err, domain.ErrInvalidSprintTransition)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Close", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// RN-18: un sprint closed es definitivo; cerrarlo de nuevo se rechaza con ErrSprintClosed (no con
// ErrSprintNotActive ni con la transición genérica) y no llega al repositorio.
func TestSprintService_Close_SprintCerrado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusClosed), nil).Once()

	view, err := svc.Close(context.Background(), access, uuid.New())

	require.ErrorIs(t, err, domain.ErrSprintClosed)
	assert.NotErrorIs(t, err, domain.ErrSprintNotActive)
	assert.NotErrorIs(t, err, domain.ErrInvalidSprintTransition)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Close", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// FR-040: un sprint active se cierra delegando en el repositorio con el sprint del acceso, el actor
// recibido como closedBy (el titular de la sesión) y el instante del reloj inyectado, que queda como
// momento real de cierre; la respuesta es el detalle del sprint.
func TestSprintService_Close_ActorEInstanteDelReloj(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	actorID := uuid.New()
	expectSuccessfulClose(d, access.SprintID, actorID)

	view, err := svc.Close(context.Background(), access, actorID)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.Detail)
	assert.Equal(t, access.SprintID, view.Detail.Sprint.ID)
	d.repo.AssertCalled(t, "Close", mock.Anything, access.SprintID, actorID, serviceFixedNow)
	d.repo.AssertExpectations(t)
}

// RN-07: cerrar un sprint no cambia el estado del proyecto. El servicio solo lee el sprint e invoca
// Close una vez, sin consultar el puerto de esfuerzo. Los dobles atados a t fallan ante cualquier
// llamada no configurada (m.Called), de modo que otra operación del repositorio haría fallar la
// prueba; y el servicio no recibe dependencia alguna del módulo de proyectos.
func TestSprintService_Close_NoTocaElProyecto(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	actorID := uuid.New()
	expectSuccessfulClose(d, access.SprintID, actorID)

	_, err := svc.Close(context.Background(), access, actorID)

	require.NoError(t, err)
	d.repo.AssertExpectations(t)
	d.repo.AssertNumberOfCalls(t, "Close", 1)
	d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
}

// Pruebas unitarias del listado y del detalle de sprints en el servicio (T073; FR-050 a FR-054,
// RN-23). El listado distingue el proyecto sin sprints del filtro sin coincidencias y solo consulta
// HasSprints cuando el resultado filtrado viene vacío (research.md sección 10). El detalle agrega
// los días restantes o vencidos solo a un sprint active, calculados con el reloj fijo y la zona
// inyectados. Igual que las anteriores, no llevan etiqueta de escenario: los TestUS6_* son las
// pruebas de integración de T074.

// listItemWithStatus arma un elemento del listado de un sprint en el estado indicado, como lo
// devuelve List del repositorio.
func listItemWithStatus(status domain.SprintStatus) domain.SprintListItem {
	item := domain.SprintListItem{}
	item.Sprint.ID = uuid.New()
	item.Sprint.Status = status
	return item
}

// FR-050, FR-054: con resultados, el listado devuelve los elementos del repositorio en su orden,
// con el filtro recibido, sin motivo de vacío y sin la consulta de existencia.
func TestSprintService_List_ConResultados(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID := uuid.New()
	statuses := []domain.SprintStatus{domain.SprintStatusClosed, domain.SprintStatusActive}
	items := []domain.SprintListItem{
		listItemWithStatus(domain.SprintStatusActive),
		listItemWithStatus(domain.SprintStatusClosed),
	}
	d.repo.On("List", mock.Anything, projectID, statuses).Return(items, nil).Once()

	view, err := svc.List(context.Background(), projectID, statuses)

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, items, view.Sprints)
	assert.Nil(t, view.EmptyReason)
	d.repo.AssertExpectations(t)
	d.repo.AssertNotCalled(t, "HasSprints", mock.Anything, mock.Anything)
}

// FR-054, US6-10: un proyecto sin sprints devuelve la lista vacía, nunca nil, con no_sprints, haya
// o no filtro de estado; la consulta de existencia se hace una sola vez porque el resultado vino
// vacío.
func TestSprintService_List_ProyectoSinSprints(t *testing.T) {
	filters := map[string][]domain.SprintStatus{
		"sin filtro":        {},
		"con filtro closed": {domain.SprintStatusClosed},
	}
	for name, statuses := range filters {
		t.Run(name, func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			projectID := uuid.New()
			d.repo.On("List", mock.Anything, projectID, statuses).Return([]domain.SprintListItem{}, nil).Once()
			d.repo.On("HasSprints", mock.Anything, projectID).Return(false, nil).Once()

			view, err := svc.List(context.Background(), projectID, statuses)

			require.NoError(t, err)
			require.NotNil(t, view)
			assert.NotNil(t, view.Sprints)
			assert.Empty(t, view.Sprints)
			require.NotNil(t, view.EmptyReason)
			assert.Equal(t, EmptyReasonNoSprints, *view.EmptyReason)
			d.repo.AssertExpectations(t)
			d.repo.AssertNumberOfCalls(t, "HasSprints", 1)
		})
	}
}

// FR-054: si el proyecto tiene sprints pero ninguno cumple el filtro, la lista vacía, nunca nil,
// lleva no_matches.
func TestSprintService_List_FiltroSinCoincidencias(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	projectID := uuid.New()
	statuses := []domain.SprintStatus{domain.SprintStatusClosed}
	d.repo.On("List", mock.Anything, projectID, statuses).Return([]domain.SprintListItem{}, nil).Once()
	d.repo.On("HasSprints", mock.Anything, projectID).Return(true, nil).Once()

	view, err := svc.List(context.Background(), projectID, statuses)

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.NotNil(t, view.Sprints)
	assert.Empty(t, view.Sprints)
	require.NotNil(t, view.EmptyReason)
	assert.Equal(t, EmptyReasonNoMatches, *view.EmptyReason)
	d.repo.AssertExpectations(t)
}

// scheduleDays devuelve un puntero al entero dado, para expresar los valores esperados no nulos.
func scheduleDays(v int) *int {
	return &v
}

// showScheduleDays representa un valor opcional de días como texto, "nil" si falta, para que un
// fallo muestre los números y no las direcciones de los punteros.
func showScheduleDays(p *int) string {
	if p == nil {
		return "nil"
	}
	return strconv.Itoa(*p)
}

// sprintDetailEnding arma el detalle de un sprint en el estado indicado con la fecha de fin dada.
func sprintDetailEnding(sprintID uuid.UUID, status domain.SprintStatus, endDate time.Time) *domain.SprintDetail {
	detail := sprintDetailWithStatus(sprintID, status)
	detail.Sprint.EndDate = endDate
	return detail
}

// FR-053, RN-23: el detalle de un sprint active informa los días restantes hasta la fecha de fin,
// con el día de fin incluido, o los días que lleva vencido, contados con el reloj fijo (2026-10-21);
// un sprint planned o closed no lleva schedule.
func TestSprintService_Detail_ScheduleSoloActivo(t *testing.T) {
	cases := []struct {
		name          string
		status        domain.SprintStatus
		endDate       time.Time
		wantSchedule  bool
		wantRemaining *int
		wantOverdue   *int
	}{
		{name: "activo con diez días restantes", status: domain.SprintStatusActive, endDate: serviceDay(time.October, 31), wantSchedule: true, wantRemaining: scheduleDays(10)},
		{name: "activo que termina hoy", status: domain.SprintStatusActive, endDate: serviceDay(time.October, 21), wantSchedule: true, wantRemaining: scheduleDays(0)},
		{name: "activo vencido hace tres días", status: domain.SprintStatusActive, endDate: serviceDay(time.October, 18), wantSchedule: true, wantOverdue: scheduleDays(3)},
		{name: "planificado sin schedule", status: domain.SprintStatusPlanned, endDate: serviceDay(time.October, 31)},
		{name: "cerrado sin schedule", status: domain.SprintStatusClosed, endDate: serviceDay(time.October, 18)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			access := sprintAccessFor(false)
			d.repo.On("FindDetail", mock.Anything, access.SprintID).
				Return(sprintDetailEnding(access.SprintID, tc.status, tc.endDate), nil).Once()

			view, err := svc.Detail(context.Background(), access)

			require.NoError(t, err)
			require.NotNil(t, view)
			if !tc.wantSchedule {
				assert.Nil(t, view.Schedule)
				return
			}
			require.NotNil(t, view.Schedule)
			assert.Equal(t, showScheduleDays(tc.wantRemaining), showScheduleDays(view.Schedule.RemainingDays), "remainingDays")
			assert.Equal(t, showScheduleDays(tc.wantOverdue), showScheduleDays(view.Schedule.OverdueDays), "overdueDays")
		})
	}
}

// RN-23: el día de calendario se toma en la zona inyectada y no en UTC. A las 01:00 UTC del
// 2026-10-22 todavía es 2026-10-21 en la zona del sistema (UTC−3), así que un sprint active que
// termina el 2026-10-21 tiene 0 días restantes y no está vencido.
func TestSprintService_Detail_ScheduleEnLaZonaDelSistema(t *testing.T) {
	repo := &mockSprintRepository{}
	repo.Test(t)
	effort := &mockSprintEffort{}
	effort.Test(t)
	zone := time.FixedZone("UTC-3", -3*60*60)
	now := time.Date(2026, time.October, 22, 1, 0, 0, 0, time.UTC)
	svc := NewSprintService(repo, effort, zone, func() time.Time { return now })
	access := sprintAccessFor(false)
	repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailEnding(access.SprintID, domain.SprintStatusActive, serviceDay(time.October, 21)), nil).Once()

	view, err := svc.Detail(context.Background(), access)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.Schedule)
	assert.Equal(t, "0", showScheduleDays(view.Schedule.RemainingDays), "remainingDays")
	assert.Equal(t, "nil", showScheduleDays(view.Schedule.OverdueDays), "overdueDays")
}

// Pruebas unitarias obligatorias de la modificación y la eliminación de un sprint en el servicio
// (T081; FR-012 a FR-016, FR-048, RN-18 a RN-20). Con el repositorio y el puerto de esfuerzo
// reemplazados por dobles se verifica el orden de las guardas: solo lectura del proyecto Finalizado,
// estado del sprint y, en la eliminación, esfuerzo registrado antes de llegar al repositorio. Igual
// que las anteriores, no llevan etiqueta de escenario: los TestUS7_* son las pruebas de integración
// de T082.

// assertNoOtherSprintWrites verifica que una modificación no disparó ninguna escritura que cambie el
// estado, las historias comprometidas o la instantánea del sprint (FR-015).
func assertNoOtherSprintWrites(t *testing.T, d serviceDoubles) {
	t.Helper()
	d.repo.AssertNotCalled(t, "Start", mock.Anything, mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Close", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "AssignStories", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "RemoveStories", mock.Anything, mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// FR-012, FR-015: un sprint planned se modifica delegando en el repositorio solo el sprint del acceso
// y los datos editables de la entrada; no se pasa estado, historias ni instantánea, no se dispara
// otra escritura y la respuesta es el detalle actualizado del sprint.
func TestSprintService_Update_SprintPlanificado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	input := serviceInput()
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil)
	d.repo.On("Update", mock.Anything, access.SprintID, input).Return(nil).Once()

	view, err := svc.Update(context.Background(), access, input)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.Detail)
	assert.Equal(t, access.SprintID, view.Detail.Sprint.ID)
	d.repo.AssertCalled(t, "Update", mock.Anything, access.SprintID, input)
	assertNoOtherSprintWrites(t, d)
	d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
	d.repo.AssertExpectations(t)
}

// RN-19: con el proyecto Finalizado la modificación se rechaza con ErrSprintsReadOnly sin leer el
// sprint ni escribir.
func TestSprintService_Update_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	view, err := svc.Update(context.Background(), access, serviceInput())

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

// FR-016: solo se modifica un sprint Planificado; uno active se rechaza con ErrSprintNotPlanned y no
// llega al repositorio.
func TestSprintService_Update_SprintActivo(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusActive), nil).Once()

	view, err := svc.Update(context.Background(), access, serviceInput())

	require.ErrorIs(t, err, domain.ErrSprintNotPlanned)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

// RN-18: un sprint closed es definitivo; la modificación se rechaza con ErrSprintClosed (no con
// ErrSprintNotPlanned) y no llega al repositorio.
func TestSprintService_Update_SprintCerrado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusClosed), nil).Once()

	view, err := svc.Update(context.Background(), access, serviceInput())

	require.ErrorIs(t, err, domain.ErrSprintClosed)
	assert.NotErrorIs(t, err, domain.ErrSprintNotPlanned)
	assert.Nil(t, view)
	d.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

// FR-048, RN-20: un sprint planned sin esfuerzo registrado se elimina consultando el puerto de
// esfuerzo y delegando después en el repositorio con el sprint del acceso.
func TestSprintService_Delete_PlanificadoSinEsfuerzo(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil).Once()
	d.effort.On("HasEffort", mock.Anything, access.SprintID).Return(false, nil).Once()
	d.repo.On("Delete", mock.Anything, access.SprintID).Return(nil).Once()

	err := svc.Delete(context.Background(), access)

	require.NoError(t, err)
	d.effort.AssertExpectations(t)
	d.repo.AssertExpectations(t)
}

// RN-19: con el proyecto Finalizado la eliminación se rechaza con ErrSprintsReadOnly sin leer el
// sprint, sin consultar el esfuerzo y sin borrar.
func TestSprintService_Delete_ProyectoFinalizado(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(true)

	err := svc.Delete(context.Background(), access)

	require.ErrorIs(t, err, domain.ErrSprintsReadOnly)
	d.repo.AssertNotCalled(t, "FindDetail", mock.Anything, mock.Anything)
	d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
	d.repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// RN-20: un sprint active o closed ya fue iniciado y no se elimina; se rechaza con
// ErrSprintAlreadyStarted antes de consultar el esfuerzo y sin llegar al repositorio.
func TestSprintService_Delete_SprintIniciado(t *testing.T) {
	for _, status := range []domain.SprintStatus{domain.SprintStatusActive, domain.SprintStatusClosed} {
		t.Run(string(status), func(t *testing.T) {
			svc, d := newSprintServiceWithDoubles(t)
			access := sprintAccessFor(false)
			d.repo.On("FindDetail", mock.Anything, access.SprintID).
				Return(sprintDetailWithStatus(access.SprintID, status), nil).Once()

			err := svc.Delete(context.Background(), access)

			require.ErrorIs(t, err, domain.ErrSprintAlreadyStarted)
			d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
			d.repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
		})
	}
}

// FR-048, RN-20: un sprint planned con esfuerzo registrado no se elimina; se rechaza con
// ErrSprintHasEffort sin llamar a Delete del repositorio.
func TestSprintService_Delete_ConEsfuerzo(t *testing.T) {
	svc, d := newSprintServiceWithDoubles(t)
	access := sprintAccessFor(false)
	d.repo.On("FindDetail", mock.Anything, access.SprintID).
		Return(sprintDetailWithStatus(access.SprintID, domain.SprintStatusPlanned), nil).Once()
	d.effort.On("HasEffort", mock.Anything, access.SprintID).Return(true, nil).Once()

	err := svc.Delete(context.Background(), access)

	require.ErrorIs(t, err, domain.ErrSprintHasEffort)
	d.effort.AssertExpectations(t)
	d.repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

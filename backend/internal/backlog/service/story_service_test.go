package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Pruebas unitarias obligatorias de la creación de historias en el servicio (T014): RN-01, RN-02,
// RN-09, RN-15 y FR-049. El repositorio y los cinco puertos se reemplazan por dobles de testify, de
// modo que cada prueba aísla la regla del servicio sin tocar la base de datos. Los nombres llevan
// el prefijo "Servicio" para no chocar con las pruebas de integración de handler_test.go (T015).

// mockStoryRepository es el doble de testify de StoryRepository.
type mockStoryRepository struct {
	mock.Mock
}

func (m *mockStoryRepository) FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*domain.StoryAccess, error) {
	args := m.Called(ctx, storyID, userID)
	access, _ := args.Get(0).(*domain.StoryAccess)
	return access, args.Error(1)
}

func (m *mockStoryRepository) Create(ctx context.Context, story *domain.UserStory, criteria []string) error {
	args := m.Called(ctx, story, criteria)
	return args.Error(0)
}

func (m *mockStoryRepository) FindDetail(ctx context.Context, storyID uuid.UUID) (*domain.StoryDetail, error) {
	args := m.Called(ctx, storyID)
	detail, _ := args.Get(0).(*domain.StoryDetail)
	return detail, args.Error(1)
}

func (m *mockStoryRepository) List(ctx context.Context, projectID uuid.UUID, filter domain.StoryFilter) ([]domain.StoryListItem, error) {
	args := m.Called(ctx, projectID, filter)
	items, _ := args.Get(0).([]domain.StoryListItem)
	return items, args.Error(1)
}

func (m *mockStoryRepository) HasStories(ctx context.Context, projectID uuid.UUID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func (m *mockStoryRepository) Update(ctx context.Context, storyID uuid.UUID, changes domain.StoryChanges) error {
	args := m.Called(ctx, storyID, changes)
	return args.Error(0)
}

func (m *mockStoryRepository) UpdateStoryPoints(ctx context.Context, storyID uuid.UUID, storyPoints *int) error {
	args := m.Called(ctx, storyID, storyPoints)
	return args.Error(0)
}

func (m *mockStoryRepository) Delete(ctx context.Context, storyID uuid.UUID) error {
	args := m.Called(ctx, storyID)
	return args.Error(0)
}

// mockSprintAssignments es el doble de testify del puerto SprintAssignments.
type mockSprintAssignments struct {
	mock.Mock
}

func (m *mockSprintAssignments) Find(ctx context.Context, storyID uuid.UUID) (domain.SprintAssignment, error) {
	args := m.Called(ctx, storyID)
	return args.Get(0).(domain.SprintAssignment), args.Error(1)
}

// mockEffortRecords es el doble de testify del puerto EffortRecords.
type mockEffortRecords struct {
	mock.Mock
}

func (m *mockEffortRecords) HasEffort(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// mockDefectRecords es el doble de testify del puerto DefectRecords.
type mockDefectRecords struct {
	mock.Mock
}

func (m *mockDefectRecords) HasDefects(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// mockEstimationSessions es el doble de testify del puerto EstimationSessions.
type mockEstimationSessions struct {
	mock.Mock
}

func (m *mockEstimationSessions) HasSessions(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// mockProjectDirectory es el doble de testify del puerto ProjectDirectory.
type mockProjectDirectory struct {
	mock.Mock
}

func (m *mockProjectDirectory) IsFinished(ctx context.Context, projectID uuid.UUID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

// storyServiceDoubles agrupa los dobles con los que se construye el servicio, para configurar sus
// expectativas y verificarlas en cada prueba.
type storyServiceDoubles struct {
	stories  *mockStoryRepository
	sprints  *mockSprintAssignments
	effort   *mockEffortRecords
	defects  *mockDefectRecords
	sessions *mockEstimationSessions
	projects *mockProjectDirectory
}

// newStoryServiceWithDoubles construye el servicio con un doble nuevo para el repositorio y para
// cada uno de los cinco puertos.
func newStoryServiceWithDoubles() (*StoryService, storyServiceDoubles) {
	d := storyServiceDoubles{
		stories:  new(mockStoryRepository),
		sprints:  new(mockSprintAssignments),
		effort:   new(mockEffortRecords),
		defects:  new(mockDefectRecords),
		sessions: new(mockEstimationSessions),
		projects: new(mockProjectDirectory),
	}
	svc := NewStoryService(d.stories, d.sprints, d.effort, d.defects, d.sessions, d.projects)
	return svc, d
}

// expectSuccessfulCreate prepara una creación que llega a persistirse en un proyecto no finalizado:
// guarda en *captured la historia y en *criteria los criterios que el servicio entrega a Create, y
// deja que la lectura del detalle y del puerto de sprints respondan sin errores. Cada llamada a Create
// sobrescribe lo capturado, y las historias de todas las llamadas se acumulan en *all si no es nil.
func expectSuccessfulCreate(d storyServiceDoubles, projectID uuid.UUID, captured **domain.UserStory, criteria *[]string, all *[]domain.UserStory) {
	d.projects.On("IsFinished", mock.Anything, projectID).Return(false, nil)
	d.stories.On("Create", mock.Anything, mock.AnythingOfType("*domain.UserStory"), mock.Anything).
		Run(func(args mock.Arguments) {
			story := args.Get(1).(*domain.UserStory)
			*captured = story
			if criteria != nil {
				*criteria, _ = args.Get(2).([]string)
			}
			if all != nil {
				*all = append(*all, *story)
			}
		}).
		Return(nil)
	d.stories.On("FindDetail", mock.Anything, mock.Anything).Return(&domain.StoryDetail{}, nil).Maybe()
	d.sprints.On("Find", mock.Anything, mock.Anything).Return(domain.SprintAssignment{}, nil).Maybe()
}

// intPtr y strPtr devuelven punteros a los valores, para expresar los campos opcionales de la entrada.
func intPtr(value int) *int {
	return &value
}

func strPtr(value string) *string {
	return &value
}

// Escenario: Creación completa (003/US1)
// RN-09: la historia nace pending, con un UUID nuevo, en el proyecto de la ruta y con los datos de
// la entrada; los criterios llegan al repositorio en el mismo orden en que se recibieron.
func TestUS1_ServicioCreacionCompleta(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID, actorID := uuid.New(), uuid.New()
	var story *domain.UserStory
	var criteria []string
	expectSuccessfulCreate(d, projectID, &story, &criteria, nil)

	input := CreateStoryInput{
		Title:              "Exportar el reporte de velocidad a PDF",
		Description:        strPtr("El equipo necesita compartir la velocidad con la gerencia."),
		Priority:           domain.StoryPriorityHigh,
		StoryPoints:        intPtr(5),
		AcceptanceCriteria: []string{"El PDF incluye la velocidad.", "El archivo lleva el nombre del proyecto."},
	}
	view, err := svc.Create(context.Background(), projectID, actorID, input)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, story, "el servicio no entregó la historia al repositorio")
	is := assert.New(t)
	is.Equal(domain.StoryStatusPending, story.Status)
	is.NotEqual(uuid.Nil, story.ID)
	is.Equal(projectID, story.ProjectID)
	is.Equal(input.Title, story.Title)
	is.Equal(input.Description, story.Description)
	is.Equal(input.Priority, story.Priority)
	is.Equal(input.StoryPoints, story.StoryPoints)
	is.Equal(input.AcceptanceCriteria, criteria)
	d.projects.AssertExpectations(t)
	d.stories.AssertExpectations(t)
}

// Escenario: Creación completa (003/US1)
// FR-049: el autor de la historia es el actor que recibe el servicio (el titular de la sesión), no
// un dato de la entrada, que no tiene campo de autor.
func TestUS1_ServicioAutorEsElActor(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID, actorID := uuid.New(), uuid.New()
	var story *domain.UserStory
	expectSuccessfulCreate(d, projectID, &story, nil, nil)

	_, err := svc.Create(context.Background(), projectID, actorID, CreateStoryInput{
		Title:    "Registrar el esfuerzo diario",
		Priority: domain.StoryPriorityMedium,
	})

	require.NoError(t, err)
	require.NotNil(t, story, "el servicio no entregó la historia al repositorio")
	assert.Equal(t, actorID, story.CreatedByID)
}

// Escenario: Creación completa (003/US1)
// RN-01 y RN-02: cualquier integrante crea historias, sea o no el propietario del proyecto. El
// servicio no consulta la propiedad: dos actores distintos crean en el mismo proyecto y cada
// historia queda atribuida a quien la creó.
func TestUS1_ServicioIntegranteNoPropietario(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID := uuid.New()
	ownerID, memberID := uuid.New(), uuid.New()
	var story *domain.UserStory
	var created []domain.UserStory
	expectSuccessfulCreate(d, projectID, &story, nil, &created)

	// Primero crea el propietario y después un integrante que no lo es.
	for _, actorID := range []uuid.UUID{ownerID, memberID} {
		_, err := svc.Create(context.Background(), projectID, actorID, CreateStoryInput{
			Title:    "Ver el backlog filtrado",
			Priority: domain.StoryPriorityLow,
		})
		require.NoError(t, err)
	}

	require.Len(t, created, 2)
	assert.Equal(t, ownerID, created[0].CreatedByID)
	assert.Equal(t, memberID, created[1].CreatedByID)
	d.stories.AssertNumberOfCalls(t, "Create", 2)
}

// Escenario: Título repetido (003/US1)
// RN-09: cada creación genera un UUID nuevo, también cuando dos historias tienen el mismo título.
func TestUS1_ServicioTituloRepetido(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID, actorID := uuid.New(), uuid.New()
	var story *domain.UserStory
	var created []domain.UserStory
	expectSuccessfulCreate(d, projectID, &story, nil, &created)

	// Se crean dos historias idénticas en el mismo proyecto.
	input := CreateStoryInput{Title: "Exportar el reporte", Priority: domain.StoryPriorityHigh}
	for range 2 {
		_, err := svc.Create(context.Background(), projectID, actorID, input)
		require.NoError(t, err)
	}

	require.Len(t, created, 2)
	assert.NotEqual(t, uuid.Nil, created[0].ID)
	assert.NotEqual(t, uuid.Nil, created[1].ID)
	assert.NotEqual(t, created[0].ID, created[1].ID)
}

// Escenario: Proyecto finalizado (003/US1)
// RN-15: con el proyecto Finalizado el servicio devuelve ErrBacklogReadOnly y no llama a Create.
func TestUS1_ServicioProyectoFinalizado(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID, actorID := uuid.New(), uuid.New()
	d.projects.On("IsFinished", mock.Anything, projectID).Return(true, nil)

	view, err := svc.Create(context.Background(), projectID, actorID, CreateStoryInput{
		Title:    "Exportar el reporte",
		Priority: domain.StoryPriorityHigh,
	})

	require.ErrorIs(t, err, domain.ErrBacklogReadOnly)
	assert.Nil(t, view)
	d.projects.AssertExpectations(t)
	d.stories.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

// Pruebas unitarias del listado del backlog en el servicio (T089): FR-033, FR-035 y FR-036. El
// repositorio es el doble de testify, de modo que cada prueba fija lo que devuelven List y
// HasStories y verifica cómo el servicio arma readyToPlan y elige el motivo del vacío.

// Con resultados, cada elemento conserva el orden y los datos del repositorio, readyToPlan sale de
// Readiness (lista solo con Story Points, el 0 incluido, y al menos un criterio), emptyReason es nil y
// HasStories no se consulta (FR-033).
func TestUS2_ServicioListadoConResultados(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID := uuid.New()
	filter := domain.StoryFilter{Statuses: []domain.StoryStatus{domain.StoryStatusPending}}
	items := []domain.StoryListItem{
		{ID: uuid.New(), Title: "Estimada y con criterios", StoryPoints: intPtr(5), CriteriaCount: 2},
		{ID: uuid.New(), Title: "Estimada en cero y con criterio", StoryPoints: intPtr(0), CriteriaCount: 1},
		{ID: uuid.New(), Title: "Sin estimar", StoryPoints: nil, CriteriaCount: 3},
		{ID: uuid.New(), Title: "Sin criterios", StoryPoints: intPtr(3), CriteriaCount: 0},
	}
	d.stories.On("List", mock.Anything, projectID, filter).Return(items, nil).Once()

	view, err := svc.List(context.Background(), projectID, filter)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.Len(t, view.Stories, len(items))
	is := assert.New(t)
	is.Nil(view.EmptyReason)
	for i, want := range []bool{true, true, false, false} {
		is.Equal(items[i], view.Stories[i].Item, "elemento %d", i)
		is.Equal(want, view.Stories[i].ReadyToPlan, "readyToPlan de %q", items[i].Title)
	}
	d.stories.AssertExpectations(t)
	d.stories.AssertNotCalled(t, "HasStories", mock.Anything, mock.Anything)
}

// Escenario: Backlog vacío (003/US2)
// Con el resultado vacío y HasStories en false, el motivo es backlog_empty con o sin filtros, la
// lista es vacía y no nil, y no hay error (FR-035).
func TestUS2_ServicioBacklogVacio(t *testing.T) {
	cases := []struct {
		name   string
		filter domain.StoryFilter
	}{
		{"sin filtros", domain.StoryFilter{}},
		{"con filtros", domain.StoryFilter{
			Statuses:   []domain.StoryStatus{domain.StoryStatusPending},
			Priorities: []domain.StoryPriority{domain.StoryPriorityHigh},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			projectID := uuid.New()
			d.stories.On("List", mock.Anything, projectID, tc.filter).Return([]domain.StoryListItem{}, nil).Once()
			d.stories.On("HasStories", mock.Anything, projectID).Return(false, nil).Once()

			view, err := svc.List(context.Background(), projectID, tc.filter)

			require.NoError(t, err)
			require.NotNil(t, view)
			is := assert.New(t)
			is.NotNil(view.Stories)
			is.Empty(view.Stories)
			require.NotNil(t, view.EmptyReason)
			is.Equal(EmptyReasonBacklogEmpty, *view.EmptyReason)
			d.stories.AssertExpectations(t)
		})
	}
}

// Escenario: Filtro sin coincidencias (003/US2)
// Con el resultado vacío y HasStories en true, el motivo es no_matches, distinguible del backlog
// vacío, y no hay error (FR-036).
func TestUS2_ServicioFiltroSinCoincidencias(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	projectID := uuid.New()
	filter := domain.StoryFilter{Priorities: []domain.StoryPriority{domain.StoryPriorityHigh}}
	d.stories.On("List", mock.Anything, projectID, filter).Return([]domain.StoryListItem{}, nil).Once()
	d.stories.On("HasStories", mock.Anything, projectID).Return(true, nil).Once()

	view, err := svc.List(context.Background(), projectID, filter)

	require.NoError(t, err)
	require.NotNil(t, view)
	is := assert.New(t)
	is.NotNil(view.Stories)
	is.Empty(view.Stories)
	require.NotNil(t, view.EmptyReason)
	is.Equal(EmptyReasonNoMatches, *view.EmptyReason)
	d.stories.AssertExpectations(t)
}

// Los errores del repositorio se propagan envueltos y sin vista: si falla List no se consulta
// HasStories, y si falla HasStories con el resultado vacío no se inventa un motivo.
func TestUS2_ServicioListadoPropagaErrores(t *testing.T) {
	errList := errors.New("fallo de List")
	errHas := errors.New("fallo de HasStories")

	t.Run("falla List", func(t *testing.T) {
		svc, d := newStoryServiceWithDoubles()
		projectID := uuid.New()
		d.stories.On("List", mock.Anything, projectID, domain.StoryFilter{}).Return(nil, errList).Once()

		view, err := svc.List(context.Background(), projectID, domain.StoryFilter{})

		require.ErrorIs(t, err, errList)
		assert.Nil(t, view)
		d.stories.AssertExpectations(t)
		d.stories.AssertNotCalled(t, "HasStories", mock.Anything, mock.Anything)
	})

	t.Run("falla HasStories", func(t *testing.T) {
		svc, d := newStoryServiceWithDoubles()
		projectID := uuid.New()
		d.stories.On("List", mock.Anything, projectID, domain.StoryFilter{}).Return([]domain.StoryListItem{}, nil).Once()
		d.stories.On("HasStories", mock.Anything, projectID).Return(false, errHas).Once()

		view, err := svc.List(context.Background(), projectID, domain.StoryFilter{})

		require.ErrorIs(t, err, errHas)
		assert.Nil(t, view)
		d.stories.AssertExpectations(t)
	})
}

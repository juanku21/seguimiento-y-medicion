package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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

// Pruebas unitarias obligatorias de la estimación en el servicio (T047): RN-14, RN-15 y RN-18. El
// acceso llega ya resuelto por RequireStoryAccess; el estado de la historia lo informa el doble del
// repositorio y el compromiso en un sprint abierto, el doble de SprintAssignments. En todos los
// rechazos se verifica que UpdateStoryPoints no se llama.

// expectStoryState prepara la lectura de una historia con el estado y los Story Points indicados y
// la respuesta del puerto de sprints sobre su compromiso en un sprint abierto. Ambas lecturas son
// opcionales, para no fijar en la prueba cuántas veces ni en qué orden las hace el servicio.
func expectStoryState(d storyServiceDoubles, access *domain.StoryAccess, status domain.StoryStatus, storyPoints *int, committed bool) {
	detail := &domain.StoryDetail{
		Story: domain.UserStory{
			ID:          access.StoryID,
			ProjectID:   access.ProjectID,
			Title:       "Exportar el reporte de velocidad",
			Priority:    domain.StoryPriorityHigh,
			Status:      status,
			StoryPoints: storyPoints,
		},
		AcceptanceCriteria: []string{"El PDF incluye la velocidad."},
	}
	d.stories.On("FindDetail", mock.Anything, access.StoryID).Return(detail, nil).Maybe()
	d.sprints.On("Find", mock.Anything, access.StoryID).
		Return(domain.SprintAssignment{CommittedToOpenSprint: committed, EverAssigned: committed}, nil).Maybe()
}

// newStoryAccess arma el acceso que RequireStoryAccess deja en el contexto para una historia nueva.
func newStoryAccess(projectFinished bool) *domain.StoryAccess {
	return &domain.StoryAccess{StoryID: uuid.New(), ProjectID: uuid.New(), ProjectFinished: projectFinished}
}

// Escenario: Historia completada (003/US4)
// RN-14: una historia completed no se reestima, ni con otro valor de la escala ni devolviéndola a
// "sin estimar"; el servicio devuelve ErrStoryCompleted y no llama a UpdateStoryPoints.
func TestUS4_ServicioHistoriaCompletada(t *testing.T) {
	cases := []struct {
		name        string
		storyPoints *int
	}{
		{"con otro valor de la escala", intPtr(8)},
		{"con nil", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryState(d, access, domain.StoryStatusCompleted, intPtr(5), false)

			view, err := svc.Estimate(context.Background(), access, tc.storyPoints)

			require.ErrorIs(t, err, domain.ErrStoryCompleted)
			assert.Nil(t, view)
			d.stories.AssertNotCalled(t, "UpdateStoryPoints", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// Escenario: Proyecto finalizado (003/US4)
// RN-15: con el proyecto Finalizado el servicio devuelve ErrBacklogReadOnly y no llama a
// UpdateStoryPoints. La regla se evalúa antes que las demás: aunque la historia esté completed y
// comprometida en un sprint abierto, y se pida devolverla a "sin estimar", el rechazo es el de solo
// lectura.
func TestUS4_ServicioProyectoFinalizado(t *testing.T) {
	cases := []struct {
		name        string
		status      domain.StoryStatus
		committed   bool
		storyPoints *int
	}{
		{"historia pendiente con un valor de la escala", domain.StoryStatusPending, false, intPtr(13)},
		{"historia completada y comprometida con nil", domain.StoryStatusCompleted, true, nil},
		{"historia en progreso y comprometida con nil", domain.StoryStatusInProgress, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(true)
			expectStoryState(d, access, tc.status, intPtr(5), tc.committed)

			view, err := svc.Estimate(context.Background(), access, tc.storyPoints)

			require.ErrorIs(t, err, domain.ErrBacklogReadOnly)
			assert.Nil(t, view)
			d.stories.AssertNotCalled(t, "UpdateStoryPoints", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// Escenario: Comprometida no vuelve a sin estimar (003/US4)
// RN-18: una historia comprometida en un sprint abierto no puede quedar sin Story Points; con nil el
// servicio devuelve ErrCommittedStoryMustStayReady y no llama a UpdateStoryPoints.
func TestUS4_ServicioComprometidaNoVuelveASinEstimar(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	expectStoryState(d, access, domain.StoryStatusInProgress, intPtr(5), true)

	view, err := svc.Estimate(context.Background(), access, nil)

	require.ErrorIs(t, err, domain.ErrCommittedStoryMustStayReady)
	assert.Nil(t, view)
	d.stories.AssertNotCalled(t, "UpdateStoryPoints", mock.Anything, mock.Anything, mock.Anything)
}

// RN-18: una historia comprometida en un sprint abierto sí admite cambiar un valor de la escala por
// otro, el 0 incluido, porque sigue lista para planificar; el servicio guarda el valor pedido con
// UpdateStoryPoints y devuelve el detalle.
func TestUS4_ServicioComprometidaAceptaOtroValor(t *testing.T) {
	for _, value := range []int{8, 0} {
		t.Run(fmt.Sprintf("estimada en %d", value), func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryState(d, access, domain.StoryStatusInProgress, intPtr(5), true)
			d.stories.On("UpdateStoryPoints", mock.Anything, access.StoryID, mock.MatchedBy(func(points *int) bool {
				return points != nil && *points == value
			})).Return(nil).Once()

			view, err := svc.Estimate(context.Background(), access, intPtr(value))

			require.NoError(t, err)
			require.NotNil(t, view)
			d.stories.AssertExpectations(t)
		})
	}
}

// Pruebas unitarias obligatorias de la modificación en el servicio (T057): RN-10, RN-11, RN-14,
// RN-15, RN-18 y FR-024. El acceso llega ya resuelto por RequireStoryAccess y los cambios ya
// validados por delivery; el estado de la historia lo informa el doble del repositorio y el
// compromiso en un sprint abierto, el doble de SprintAssignments. En todos los rechazos se verifica
// que Update del repositorio no se llama.

// criteriaPtr devuelve un puntero a la lista, para expresar acceptanceCriteria presente en el PATCH.
func criteriaPtr(criteria ...string) *[]string {
	list := append([]string{}, criteria...)
	return &list
}

// priorityPtr devuelve un puntero a la prioridad, para expresar priority presente en el PATCH.
func priorityPtr(priority domain.StoryPriority) *domain.StoryPriority {
	return &priority
}

// patchCases son cambios válidos de distinto tipo, para comprobar que una regla de rechazo no
// depende de qué campos trae el PATCH.
var patchCases = []struct {
	name    string
	changes domain.StoryChanges
}{
	{"título", domain.StoryChanges{Title: strPtr("Exportar el reporte de velocidad a PDF")}},
	{"descripción borrada", domain.StoryChanges{DescriptionSet: true}},
	{"prioridad", domain.StoryChanges{Priority: priorityPtr(domain.StoryPriorityLow)}},
	{"criterios vacíos", domain.StoryChanges{AcceptanceCriteria: criteriaPtr()}},
	{"criterios nuevos", domain.StoryChanges{AcceptanceCriteria: criteriaPtr("El PDF incluye la velocidad.")}},
}

// expectSuccessfulUpdate prepara una modificación que llega a persistirse: Update del repositorio
// recibe exactamente los cambios indicados y responde sin error.
func expectSuccessfulUpdate(d storyServiceDoubles, access *domain.StoryAccess, changes domain.StoryChanges) {
	d.stories.On("Update", mock.Anything, access.StoryID, changes).Return(nil).Once()
}

// Escenario: Historia completada (003/US5)
// RN-14: una historia completed no se modifica, cualquiera sea el campo; el servicio devuelve
// ErrStoryCompleted y no llama a Update del repositorio.
func TestUS5_ServicioHistoriaCompletada(t *testing.T) {
	for _, tc := range patchCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryState(d, access, domain.StoryStatusCompleted, intPtr(5), false)

			view, err := svc.Update(context.Background(), access, tc.changes)

			require.ErrorIs(t, err, domain.ErrStoryCompleted)
			assert.Nil(t, view)
			d.stories.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// Escenario: Proyecto finalizado (003/US5)
// RN-15: con el proyecto Finalizado el servicio devuelve ErrBacklogReadOnly y no llama a Update del
// repositorio. La regla se evalúa antes que las demás: aunque la historia esté completed, o
// comprometida en un sprint abierto y se pida vaciar sus criterios, el rechazo es el de solo lectura.
func TestUS5_ServicioProyectoFinalizado(t *testing.T) {
	cases := []struct {
		name      string
		status    domain.StoryStatus
		committed bool
		changes   domain.StoryChanges
	}{
		{"historia pendiente con un título nuevo", domain.StoryStatusPending, false,
			domain.StoryChanges{Title: strPtr("Ver el backlog filtrado")}},
		{"historia completada", domain.StoryStatusCompleted, false,
			domain.StoryChanges{Priority: priorityPtr(domain.StoryPriorityHigh)}},
		{"historia comprometida con criterios vacíos", domain.StoryStatusInProgress, true,
			domain.StoryChanges{AcceptanceCriteria: criteriaPtr()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(true)
			expectStoryState(d, access, tc.status, intPtr(5), tc.committed)

			view, err := svc.Update(context.Background(), access, tc.changes)

			require.ErrorIs(t, err, domain.ErrBacklogReadOnly)
			assert.Nil(t, view)
			d.stories.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// Escenario: Comprometida no queda sin criterios (003/US5)
// RN-18: una historia comprometida en un sprint abierto no puede quedar sin criterios de aceptación;
// con acceptanceCriteria vacío el servicio devuelve ErrCommittedStoryMustStayReady y no llama a
// Update del repositorio, aunque el PATCH traiga además otros cambios válidos.
func TestUS5_ServicioComprometidaNoQuedaSinCriterios(t *testing.T) {
	cases := []struct {
		name    string
		changes domain.StoryChanges
	}{
		{"solo criterios vacíos", domain.StoryChanges{AcceptanceCriteria: criteriaPtr()}},
		{"criterios vacíos junto a un título nuevo", domain.StoryChanges{
			Title:              strPtr("Registrar el esfuerzo diario"),
			AcceptanceCriteria: criteriaPtr(),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryState(d, access, domain.StoryStatusInProgress, intPtr(5), true)

			view, err := svc.Update(context.Background(), access, tc.changes)

			require.ErrorIs(t, err, domain.ErrCommittedStoryMustStayReady)
			assert.Nil(t, view)
			d.stories.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// RN-18: una historia comprometida en un sprint abierto sí admite reemplazar sus criterios por una
// lista no vacía, o cambiar otros campos sin tocar los criterios, porque sigue lista para planificar;
// el servicio entrega los cambios a Update del repositorio y devuelve el detalle.
func TestUS5_ServicioComprometidaAceptaCambios(t *testing.T) {
	cases := []struct {
		name    string
		changes domain.StoryChanges
	}{
		{"criterios no vacíos", domain.StoryChanges{
			AcceptanceCriteria: criteriaPtr("El PDF incluye la velocidad.", "El archivo lleva el nombre del proyecto."),
		}},
		{"descripción sin tocar los criterios", domain.StoryChanges{
			DescriptionSet: true,
			Description:    strPtr("Se comparte con la gerencia."),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryState(d, access, domain.StoryStatusInProgress, intPtr(5), true)
			expectSuccessfulUpdate(d, access, tc.changes)

			view, err := svc.Update(context.Background(), access, tc.changes)

			require.NoError(t, err)
			require.NotNil(t, view)
			d.stories.AssertExpectations(t)
		})
	}
}

// Escenario: Vaciar criterios (003/US5)
// Una historia que no está comprometida en ningún sprint sí puede quedar sin criterios: el servicio
// entrega la lista vacía a Update del repositorio y devuelve el detalle.
func TestUS5_ServicioVaciarCriterios(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	changes := domain.StoryChanges{AcceptanceCriteria: criteriaPtr()}
	expectStoryState(d, access, domain.StoryStatusPending, intPtr(5), false)
	expectSuccessfulUpdate(d, access, changes)

	view, err := svc.Update(context.Background(), access, changes)

	require.NoError(t, err)
	require.NotNil(t, view)
	d.stories.AssertExpectations(t)
}

// Escenario: Historia en progreso (003/US5)
// RN-14: solo el estado completed cierra la historia a modificaciones; una historia in_progress
// dentro de un sprint acepta la corrección de su descripción.
func TestUS5_ServicioHistoriaEnProgreso(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	changes := domain.StoryChanges{DescriptionSet: true, Description: strPtr("Descripción corregida.")}
	expectStoryState(d, access, domain.StoryStatusInProgress, intPtr(5), true)
	expectSuccessfulUpdate(d, access, changes)

	view, err := svc.Update(context.Background(), access, changes)

	require.NoError(t, err)
	require.NotNil(t, view)
	d.stories.AssertExpectations(t)
}

// Escenario: Cambio de estado no disponible (003/US5)
// FR-024, RN-10 y RN-11: StoryChanges solo tiene los campos editables, de modo que el estado, el
// proyecto, el autor y el momento de creación no pueden viajar al repositorio por construcción; y el
// servicio entrega a Update exactamente los cambios recibidos, sin agregar ninguno ni reestimar.
func TestUS5_ServicioCambioDeEstadoNoDisponible(t *testing.T) {
	t.Run("StoryChanges no incluye campos de solo lectura", func(t *testing.T) {
		fields := reflect.TypeOf(domain.StoryChanges{})
		names := make([]string, 0, fields.NumField())
		for i := range fields.NumField() {
			names = append(names, fields.Field(i).Name)
		}
		assert.ElementsMatch(t,
			[]string{"Title", "DescriptionSet", "Description", "Priority", "AcceptanceCriteria"}, names)
	})

	t.Run("el servicio entrega solo los cambios recibidos", func(t *testing.T) {
		svc, d := newStoryServiceWithDoubles()
		access := newStoryAccess(false)
		changes := domain.StoryChanges{Priority: priorityPtr(domain.StoryPriorityLow)}
		expectStoryState(d, access, domain.StoryStatusPending, intPtr(5), false)
		expectSuccessfulUpdate(d, access, changes)

		view, err := svc.Update(context.Background(), access, changes)

		require.NoError(t, err)
		require.NotNil(t, view)
		d.stories.AssertExpectations(t)
		d.stories.AssertNotCalled(t, "UpdateStoryPoints", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Pruebas unitarias obligatorias de la eliminación en el servicio (T066): RN-15, RN-16 y FR-041 a
// FR-043. El acceso llega ya resuelto por RequireStoryAccess; el historial de la historia lo
// informan los dobles de los cuatro puertos (sprints, esfuerzo, defectos y sesiones de estimación).
// En todos los rechazos se verifica que Delete del repositorio no se llama.

// storyHistory describe qué historial informa cada uno de los cuatro puertos sobre la historia.
type storyHistory struct {
	everAssigned bool
	effort       bool
	defects      bool
	sessions     bool
}

// expectStoryHistory prepara la respuesta de los cuatro puertos para la historia del acceso. La
// historia quitada de un sprint figura como asignada alguna vez pero no comprometida en un sprint
// abierto, porque la condición de RN-16 es no haber estado nunca en un sprint.
func expectStoryHistory(d storyServiceDoubles, access *domain.StoryAccess, history storyHistory) {
	d.sprints.On("Find", mock.Anything, access.StoryID).
		Return(domain.SprintAssignment{EverAssigned: history.everAssigned}, nil)
	d.effort.On("HasEffort", mock.Anything, access.StoryID).Return(history.effort, nil)
	d.defects.On("HasDefects", mock.Anything, access.StoryID).Return(history.defects, nil)
	d.sessions.On("HasSessions", mock.Anything, access.StoryID).Return(history.sessions, nil)
}

// assertDeletionBlocked comprueba que el error es DeletionBlockedError con exactamente los motivos
// esperados, en el orden de T005, y que Delete del repositorio no se llamó.
func assertDeletionBlocked(t *testing.T, d storyServiceDoubles, err error, expected ...domain.DeletionBlocker) {
	t.Helper()
	var blocked *domain.DeletionBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, expected, blocked.Reasons)
	d.stories.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// Escenario: Eliminación exitosa (003/US6)
// RN-16: una historia que nunca estuvo en un sprint, sin esfuerzo, defectos ni sesiones de
// estimación se elimina: el servicio consulta los cuatro puertos y llama a Delete del repositorio
// con su identificador.
func TestUS6_ServicioEliminacionExitosa(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	expectStoryHistory(d, access, storyHistory{})
	d.stories.On("Delete", mock.Anything, access.StoryID).Return(nil).Once()

	err := svc.Delete(context.Background(), access)

	require.NoError(t, err)
	d.stories.AssertExpectations(t)
	d.sprints.AssertExpectations(t)
	d.effort.AssertExpectations(t)
	d.defects.AssertExpectations(t)
	d.sessions.AssertExpectations(t)
}

// Escenario: Quitada de un sprint (003/US6)
// RN-16: una historia que estuvo en un sprint y ya no está comprometida en ninguno abierto se rechaza
// con el motivo sprint_history.
func TestUS6_ServicioQuitadaDeUnSprint(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	expectStoryHistory(d, access, storyHistory{everAssigned: true})

	err := svc.Delete(context.Background(), access)

	assertDeletionBlocked(t, d, err, domain.DeletionBlockerSprintHistory)
}

// Escenario: Esfuerzo o defectos (003/US6)
// RN-16 y FR-043: una historia con esfuerzo registrado o con defectos asociados se rechaza indicando
// cuál de los dos historiales lo impide, y con ambos indica los dos en el orden de T005.
func TestUS6_ServicioEsfuerzoODefectos(t *testing.T) {
	cases := []struct {
		name     string
		history  storyHistory
		expected []domain.DeletionBlocker
	}{
		{"con esfuerzo", storyHistory{effort: true},
			[]domain.DeletionBlocker{domain.DeletionBlockerEffort}},
		{"con defectos", storyHistory{defects: true},
			[]domain.DeletionBlocker{domain.DeletionBlockerDefects}},
		{"con esfuerzo y defectos", storyHistory{effort: true, defects: true},
			[]domain.DeletionBlocker{domain.DeletionBlockerEffort, domain.DeletionBlockerDefects}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryHistory(d, access, tc.history)

			err := svc.Delete(context.Background(), access)

			assertDeletionBlocked(t, d, err, tc.expected...)
		})
	}
}

// Escenario: Sesiones de estimación (003/US6)
// RN-16 y RC-10: una historia que nunca estuvo en un sprint pero tiene una sesión de Planning Poker,
// Finalizada o Cancelada, se rechaza con el motivo estimation_sessions.
func TestUS6_ServicioSesionesDeEstimacion(t *testing.T) {
	svc, d := newStoryServiceWithDoubles()
	access := newStoryAccess(false)
	expectStoryHistory(d, access, storyHistory{sessions: true})

	err := svc.Delete(context.Background(), access)

	assertDeletionBlocked(t, d, err, domain.DeletionBlockerEstimationSessions)
}

// FR-043: con varios historiales a la vez el rechazo lleva todos los motivos, siempre en el orden de
// T005 (sprint_history, effort, defects, estimation_sessions), sin importar cuáles se combinen.
func TestUS6_ServicioVariosMotivos(t *testing.T) {
	cases := []struct {
		name     string
		history  storyHistory
		expected []domain.DeletionBlocker
	}{
		{"sprint y sesiones", storyHistory{everAssigned: true, sessions: true},
			[]domain.DeletionBlocker{domain.DeletionBlockerSprintHistory, domain.DeletionBlockerEstimationSessions}},
		{"defectos y sesiones", storyHistory{defects: true, sessions: true},
			[]domain.DeletionBlocker{domain.DeletionBlockerDefects, domain.DeletionBlockerEstimationSessions}},
		{"los cuatro motivos", storyHistory{everAssigned: true, effort: true, defects: true, sessions: true},
			[]domain.DeletionBlocker{
				domain.DeletionBlockerSprintHistory,
				domain.DeletionBlockerEffort,
				domain.DeletionBlockerDefects,
				domain.DeletionBlockerEstimationSessions,
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(false)
			expectStoryHistory(d, access, tc.history)

			err := svc.Delete(context.Background(), access)

			assertDeletionBlocked(t, d, err, tc.expected...)
		})
	}
}

// Escenario: Proyecto finalizado (003/US6)
// RN-15: con el proyecto Finalizado el servicio devuelve ErrBacklogReadOnly antes de consultar los
// cuatro puertos y no llama a Delete del repositorio, tenga o no historial la historia.
func TestUS6_ServicioProyectoFinalizado(t *testing.T) {
	cases := []struct {
		name    string
		history storyHistory
	}{
		{"historia sin historial", storyHistory{}},
		{"historia con los cuatro historiales", storyHistory{everAssigned: true, effort: true, defects: true, sessions: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, d := newStoryServiceWithDoubles()
			access := newStoryAccess(true)
			expectStoryHistory(d, access, tc.history)

			err := svc.Delete(context.Background(), access)

			require.ErrorIs(t, err, domain.ErrBacklogReadOnly)
			d.sprints.AssertNotCalled(t, "Find", mock.Anything, mock.Anything)
			d.effort.AssertNotCalled(t, "HasEffort", mock.Anything, mock.Anything)
			d.defects.AssertNotCalled(t, "HasDefects", mock.Anything, mock.Anything)
			d.sessions.AssertNotCalled(t, "HasSessions", mock.Anything, mock.Anything)
			d.stories.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
		})
	}
}

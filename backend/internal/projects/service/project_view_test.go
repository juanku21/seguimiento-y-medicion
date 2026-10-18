package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas unitarias del armado de la vista de estado (T055): FR-042 a FR-045 y RN-08. Los puertos
// de backlog y de sprints se reemplazan por dobles de testify, de modo que el escenario de 16 Story
// Points y 104 horas no depende de las specs 003 ni 004. Los repositorios de proyectos y de
// integrantes son dobles en memoria mínimos, porque la vista solo lee de ellos.

// viewMockBacklog es el doble de testify del puerto BacklogSummaryProvider.
type viewMockBacklog struct {
	mock.Mock
}

func (m *viewMockBacklog) Summarize(ctx context.Context, projectID uuid.UUID) (domain.BacklogSummary, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(domain.BacklogSummary), args.Error(1)
}

// viewMockSprints es el doble de testify del puerto SprintDirectory; devuelve (nil, nil) cuando el
// proyecto no tiene sprint activo, como exige el contrato del puerto (FR-043).
type viewMockSprints struct {
	mock.Mock
}

func (m *viewMockSprints) FindActiveSprint(ctx context.Context, projectID uuid.UUID) (*domain.ActiveSprint, error) {
	args := m.Called(ctx, projectID)
	sprint, _ := args.Get(0).(*domain.ActiveSprint)
	return sprint, args.Error(1)
}

// viewFakeProjects devuelve siempre el mismo proyecto en FindByID; las demás operaciones no las usa
// la vista de estado.
type viewFakeProjects struct {
	project domain.Project
}

func (r *viewFakeProjects) Create(context.Context, *domain.Project, *domain.ProjectMember) error {
	return errMemberOpNotUsed
}

func (r *viewFakeProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	if id != r.project.ID {
		return nil, domain.ErrProjectNotFound
	}
	project := r.project
	return &project, nil
}

func (r *viewFakeProjects) Update(context.Context, *domain.Project) error {
	return errMemberOpNotUsed
}

func (r *viewFakeProjects) ListForMember(context.Context, uuid.UUID) ([]domain.ProjectListItem, error) {
	return nil, errMemberOpNotUsed
}

// viewFakeMembers devuelve una lista fija de integrantes en ListMembers.
type viewFakeMembers struct {
	members []domain.MemberView
}

func (r *viewFakeMembers) FindByProjectAndUser(context.Context, uuid.UUID, uuid.UUID) (*domain.ProjectMember, error) {
	return nil, errMemberOpNotUsed
}

func (r *viewFakeMembers) Create(context.Context, *domain.ProjectMember) error {
	return errMemberOpNotUsed
}

func (r *viewFakeMembers) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	return errMemberOpNotUsed
}

func (r *viewFakeMembers) ListMembers(context.Context, uuid.UUID) ([]domain.MemberView, error) {
	return r.members, nil
}

// newViewProject arma un proyecto en curso con el factor indicado en centésimas de hora.
func newViewProject(hoursPerStoryPointHundredths int) domain.Project {
	ownerID := uuid.New()
	return domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         "Proyecto de prueba",
		NameNormalized:               "proyecto de prueba",
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: hoursPerStoryPointHundredths,
		Status:                       domain.ProjectStatusInProgress,
	}
}

// newViewService compone el servicio con el proyecto dado, un integrante y los dos dobles de puertos.
func newViewService(project domain.Project, backlog *viewMockBacklog, sprints *viewMockSprints) *ProjectService {
	members := &viewFakeMembers{members: []domain.MemberView{{
		UserID:   project.OwnerID,
		FullName: "Ana Propietaria",
		Email:    "ana@example.com",
		IsOwner:  true,
		JoinedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	}}}
	return NewProjectService(&viewFakeProjects{project: project}, members, backlog, sprints)
}

// Escenario: Proyecto con factor 6,5 y backlog estimado en 3, 5 y 8 Story Points más una historia sin estimar (002/US4)
func TestUS4_HorasEstimadas(t *testing.T) {
	ctx := context.Background()
	project := newViewProject(650)

	// El puerto informa 16 Story Points (3 + 5 + 8) y cuatro historias por estado: la sin estimar
	// cuenta en su estado pero no suma Story Points.
	backlog := &viewMockBacklog{}
	backlog.On("Summarize", mock.Anything, project.ID).Return(domain.BacklogSummary{
		CountsByState:        map[string]int{"to_do": 2, "in_progress": 1, "done": 1},
		EstimatedStoryPoints: 16,
	}, nil).Once()
	sprints := &viewMockSprints{}
	sprints.On("FindActiveSprint", mock.Anything, project.ID).Return(nil, nil).Once()

	view, err := newViewService(project, backlog, sprints).Get(ctx, project.ID)

	// 16 Story Points por 6,50 horas son 104 horas, es decir 10400 centésimas.
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, 16, view.Backlog.EstimatedStoryPoints)
	assert.Equal(t, 10400, view.EstimatedHoursHundredths)
	backlog.AssertExpectations(t)
	sprints.AssertExpectations(t)
}

// Escenario: Proyecto con sprint activo o sin sprint activo (002/US4)
func TestUS4_SprintActivoSegunElPuerto(t *testing.T) {
	ctx := context.Background()

	// Con sprint activo, la vista trae exactamente el sprint que informa el puerto.
	t.Run("con sprint activo", func(t *testing.T) {
		project := newViewProject(650)
		active := &domain.ActiveSprint{ID: uuid.New(), Name: "Sprint 2"}
		backlog := &viewMockBacklog{}
		backlog.On("Summarize", mock.Anything, project.ID).
			Return(domain.BacklogSummary{CountsByState: map[string]int{}}, nil).Once()
		sprints := &viewMockSprints{}
		sprints.On("FindActiveSprint", mock.Anything, project.ID).Return(active, nil).Once()

		view, err := newViewService(project, backlog, sprints).Get(ctx, project.ID)

		require.NoError(t, err)
		require.NotNil(t, view)
		require.NotNil(t, view.ActiveSprint)
		assert.Equal(t, *active, *view.ActiveSprint)
		sprints.AssertExpectations(t)
	})

	// Sin sprint activo, la vista no trae sprint y la ausencia no es un error (FR-043).
	t.Run("sin sprint activo", func(t *testing.T) {
		project := newViewProject(650)
		backlog := &viewMockBacklog{}
		backlog.On("Summarize", mock.Anything, project.ID).
			Return(domain.BacklogSummary{CountsByState: map[string]int{}}, nil).Once()
		sprints := &viewMockSprints{}
		sprints.On("FindActiveSprint", mock.Anything, project.ID).Return(nil, nil).Once()

		view, err := newViewService(project, backlog, sprints).Get(ctx, project.ID)

		require.NoError(t, err)
		require.NotNil(t, view)
		assert.Nil(t, view.ActiveSprint)
		sprints.AssertExpectations(t)
	})
}

// Escenario: Backlog con historias en distintos estados (002/US4)
func TestUS4_ConteoPorEstadoTalComoLlega(t *testing.T) {
	ctx := context.Background()
	project := newViewProject(650)

	// Los estados son nombres arbitrarios que la vista no conoce: el conjunto lo define la spec 003,
	// así que el mapa debe trasladarse sin agregar, quitar ni renombrar claves.
	counts := map[string]int{"estado_a": 3, "estado_b": 0, "otro_estado_desconocido": 7}
	backlog := &viewMockBacklog{}
	backlog.On("Summarize", mock.Anything, project.ID).Return(domain.BacklogSummary{
		CountsByState:        counts,
		EstimatedStoryPoints: 0,
	}, nil).Once()
	sprints := &viewMockSprints{}
	sprints.On("FindActiveSprint", mock.Anything, project.ID).Return(nil, nil).Once()

	view, err := newViewService(project, backlog, sprints).Get(ctx, project.ID)

	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, map[string]int{"estado_a": 3, "estado_b": 0, "otro_estado_desconocido": 7}, view.Backlog.CountsByState)
	backlog.AssertExpectations(t)
}

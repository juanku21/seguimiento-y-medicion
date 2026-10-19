package service

import (
	"context"
	"errors"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas unitarias obligatorias de la finalización del proyecto (T071): RN-10, RN-11, RN-16 y
// SC-010. El repositorio de proyectos es un doble en memoria que registra cada escritura, el puerto
// de sprints es el doble de testify viewMockSprints de project_view_test.go y el reloj se inyecta
// para fijar el instante del cambio. Los nombres llevan el prefijo "Regla" para no chocar con las
// pruebas de integración de los mismos escenarios (T072).

// errStatusOpNotUsed señala una operación que el cambio de estado no debería invocar.
var errStatusOpNotUsed = errors.New("operación no usada por el cambio de estado")

// statusFakeProjects guarda proyectos en memoria y registra cada proyecto que recibe Update, para
// verificar qué intentó escribir el servicio y qué quedó guardado.
type statusFakeProjects struct {
	byID     map[uuid.UUID]domain.Project
	received []domain.Project
}

func (r *statusFakeProjects) Create(context.Context, *domain.Project, *domain.ProjectMember) error {
	return errStatusOpNotUsed
}

func (r *statusFakeProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	project, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return &project, nil
}

func (r *statusFakeProjects) Update(_ context.Context, project *domain.Project) error {
	r.received = append(r.received, *project)
	r.byID[project.ID] = *project
	return nil
}

func (r *statusFakeProjects) ListForMember(context.Context, uuid.UUID) ([]domain.ProjectListItem, error) {
	return nil, errStatusOpNotUsed
}

// statusFixture reúne el servicio de estado, el proyecto de partida y los dobles para verificar qué
// quedó guardado y qué se consultó.
type statusFixture struct {
	service  *StatusService
	project  domain.Project
	projects *statusFakeProjects
	sprints  *viewMockSprints
}

// newStatusFixture arma un proyecto en el estado indicado, con fecha real solo si está finalizado, y
// el servicio de estado con el sprint activo que informe el doble, la zona y el instante recibidos.
func newStatusFixture(t *testing.T, status domain.ProjectStatus, activeSprint *domain.ActiveSprint, location *time.Location, now time.Time) statusFixture {
	t.Helper()
	createdAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	project := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      uuid.New(),
		Name:                         "Portal de turnos",
		NameNormalized:               "portal de turnos",
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 600,
		Status:                       status,
		CreatedAt:                    createdAt,
		UpdatedAt:                    createdAt,
	}
	if status == domain.ProjectStatusFinished {
		actualEnd := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
		project.ActualEndDate = &actualEnd
	}

	projects := &statusFakeProjects{byID: map[uuid.UUID]domain.Project{project.ID: project}}
	sprints := &viewMockSprints{}
	sprints.On("FindActiveSprint", mock.Anything, project.ID).Return(activeSprint, nil).Maybe()

	return statusFixture{
		service:  NewStatusService(projects, sprints, location, func() time.Time { return now }),
		project:  project,
		projects: projects,
		sprints:  sprints,
	}
}

// loadLocation carga una zona horaria de la base embebida por time/tzdata.
func loadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	require.NoError(t, err)
	return location
}

// Escenario: Recorrido completo (002/US6)
// RN-10 y FR-039: al finalizar, la fecha real es la fecha de calendario del instante del cambio en
// la zona del sistema y no en UTC. El reloj devuelve el instante en UTC para que el servicio tenga
// que convertirlo con la zona recibida: a las 23:30 en Buenos Aires ya es el día siguiente en UTC, y
// a las 08:00 en Tokio todavía es el día anterior en UTC.
func TestUS6_ReglaFechaRealEnZonaDelSistema(t *testing.T) {
	cases := []struct {
		name     string
		location string
		now      time.Time
		expected string
	}{
		{
			name:     "23:30 en Buenos Aires con UTC ya en el día siguiente",
			location: "America/Argentina/Buenos_Aires",
			now:      time.Date(2026, 12, 11, 2, 30, 0, 0, time.UTC),
			expected: "2026-12-10",
		},
		{
			name:     "08:00 en Tokio con UTC todavía en el día anterior",
			location: "Asia/Tokyo",
			now:      time.Date(2026, 12, 9, 23, 0, 0, 0, time.UTC),
			expected: "2026-12-10",
		},
	}

	// Cada zona se prueba en un subtest propio para identificar cuál falla.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newStatusFixture(t, domain.ProjectStatusInProgress, nil, loadLocation(t, tc.location), tc.now)

			project, err := fx.service.ChangeStatus(context.Background(), fx.project.ID, domain.ProjectStatusFinished)

			require.NoError(t, err)
			require.NotNil(t, project)
			assert.Equal(t, domain.ProjectStatusFinished, project.Status)
			require.NotNil(t, project.ActualEndDate)
			assert.Equal(t, tc.expected, project.ActualEndDate.Format("2006-01-02"))

			// SC-010: lo guardado queda finalizado y con la misma fecha real.
			stored := fx.projects.byID[fx.project.ID]
			assert.Equal(t, domain.ProjectStatusFinished, stored.Status)
			require.NotNil(t, stored.ActualEndDate)
			assert.Equal(t, tc.expected, stored.ActualEndDate.Format("2006-01-02"))
		})
	}
}

// Escenario: Finalizar con sprint activo (002/US6)
// RN-16 y FR-041: con un sprint activo la finalización se rechaza con ErrActiveSprintBlocksFinish,
// no se escribe nada y el proyecto sigue in_progress sin fecha real.
func TestUS6_ReglaFinalizarConSprintActivo(t *testing.T) {
	location := loadLocation(t, "America/Argentina/Buenos_Aires")
	active := &domain.ActiveSprint{ID: uuid.New(), Name: "Sprint 3"}
	fx := newStatusFixture(t, domain.ProjectStatusInProgress, active, location, time.Date(2026, 12, 10, 15, 0, 0, 0, time.UTC))

	project, err := fx.service.ChangeStatus(context.Background(), fx.project.ID, domain.ProjectStatusFinished)

	assert.ErrorIs(t, err, domain.ErrActiveSprintBlocksFinish)
	assert.Nil(t, project)
	assert.Empty(t, fx.projects.received, "no debe escribirse el proyecto")
	fx.sprints.AssertCalled(t, "FindActiveSprint", mock.Anything, fx.project.ID)

	// El proyecto guardado sigue en curso y sin fecha de finalización real.
	stored := fx.projects.byID[fx.project.ID]
	assert.Equal(t, domain.ProjectStatusInProgress, stored.Status)
	assert.Nil(t, stored.ActualEndDate)
}

// Escenario: Arranque sin condiciones (002/US6)
// RN-16 solo condiciona la finalización: planned -> in_progress se acepta aunque el puerto informe un
// sprint activo, y el proyecto en curso no tiene fecha real.
func TestUS6_ReglaArranqueSinCondiciones(t *testing.T) {
	location := loadLocation(t, "America/Argentina/Buenos_Aires")
	active := &domain.ActiveSprint{ID: uuid.New(), Name: "Sprint 1"}
	fx := newStatusFixture(t, domain.ProjectStatusPlanned, active, location, time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC))

	project, err := fx.service.ChangeStatus(context.Background(), fx.project.ID, domain.ProjectStatusInProgress)

	require.NoError(t, err)
	require.NotNil(t, project)
	assert.Equal(t, domain.ProjectStatusInProgress, project.Status)
	assert.Nil(t, project.ActualEndDate)

	// Lo guardado queda en curso y sin fecha real.
	stored := fx.projects.byID[fx.project.ID]
	assert.Equal(t, domain.ProjectStatusInProgress, stored.Status)
	assert.Nil(t, stored.ActualEndDate)
}

// Escenario: Finalizado es solo lectura (002/US6)
// RN-10, RN-11 y SC-010: un proyecto finished rechaza todo cambio de estado, incluido el de pasar a
// finished otra vez, con ErrProjectReadOnly; no se escribe nada y la fecha real ya registrada no
// cambia aunque el reloj marque otro día.
func TestUS6_ReglaFinalizadoEsSoloLectura(t *testing.T) {
	location := loadLocation(t, "America/Argentina/Buenos_Aires")
	targets := []domain.ProjectStatus{
		domain.ProjectStatusPlanned,
		domain.ProjectStatusInProgress,
		domain.ProjectStatusFinished,
	}

	// Cada estado destino se prueba en un subtest propio para identificar cuál falla.
	for _, to := range targets {
		t.Run("finished a "+string(to), func(t *testing.T) {
			fx := newStatusFixture(t, domain.ProjectStatusFinished, nil, location, time.Date(2026, 12, 20, 15, 0, 0, 0, time.UTC))
			registered := *fx.project.ActualEndDate

			project, err := fx.service.ChangeStatus(context.Background(), fx.project.ID, to)

			assert.ErrorIs(t, err, domain.ErrProjectReadOnly)
			assert.Nil(t, project)
			assert.Empty(t, fx.projects.received, "no debe escribirse el proyecto")

			// El proyecto guardado sigue finalizado y con la misma fecha real.
			stored := fx.projects.byID[fx.project.ID]
			assert.Equal(t, domain.ProjectStatusFinished, stored.Status)
			require.NotNil(t, stored.ActualEndDate)
			assert.True(t, registered.Equal(*stored.ActualEndDate), "la fecha real no debe cambiar")
		})
	}
}

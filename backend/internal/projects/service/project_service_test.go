package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas unitarias obligatorias de las reglas de la modificación del proyecto (T062): FR-014,
// FR-015, FR-016, FR-018 y RN-15. El repositorio de proyectos es un doble en memoria que reproduce
// el índice único (owner_id, name_normalized) y la atomicidad del UPDATE; el de integrantes cuenta
// sus escrituras para verificar que la modificación no toca otra tabla. Los puertos de backlog y de
// sprints son los dobles de testify de project_view_test.go, solo de lectura.

// errUpdateOpNotUsed señala una operación que la modificación del proyecto no debería invocar.
var errUpdateOpNotUsed = errors.New("operación no usada por la modificación del proyecto")

// updateFakeProjects guarda proyectos en memoria. Update reproduce el índice único por propietario:
// rechaza con ErrProjectNameTaken solo si OTRO proyecto del mismo propietario usa el nombre
// normalizado, así que el propio proyecto queda excluido como en un UPDATE real (FR-014). Un rechazo
// no guarda nada (FR-015). Registra cada proyecto recibido para verificar qué intentó escribir el
// servicio.
type updateFakeProjects struct {
	byID     map[uuid.UUID]domain.Project
	received []domain.Project
}

func (r *updateFakeProjects) Create(context.Context, *domain.Project, *domain.ProjectMember) error {
	return errUpdateOpNotUsed
}

func (r *updateFakeProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	project, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return &project, nil
}

func (r *updateFakeProjects) Update(_ context.Context, project *domain.Project) error {
	r.received = append(r.received, *project)
	for id, other := range r.byID {
		if id != project.ID && other.OwnerID == project.OwnerID && other.NameNormalized == project.NameNormalized {
			return domain.ErrProjectNameTaken
		}
	}
	r.byID[project.ID] = *project
	return nil
}

func (r *updateFakeProjects) UpdateStatus(context.Context, *domain.Project) error {
	return errUpdateOpNotUsed
}

func (r *updateFakeProjects) ListForMember(context.Context, uuid.UUID) ([]domain.ProjectListItem, error) {
	return nil, errUpdateOpNotUsed
}

// updateFakeMembers lista al propietario para la vista y cuenta las escrituras sobre
// project_members, que la modificación nunca debe hacer.
type updateFakeMembers struct {
	writes int
}

func (r *updateFakeMembers) FindByProjectAndUser(context.Context, uuid.UUID, uuid.UUID) (*domain.ProjectMember, error) {
	return nil, errUpdateOpNotUsed
}

func (r *updateFakeMembers) Create(context.Context, *domain.ProjectMember) error {
	r.writes++
	return errUpdateOpNotUsed
}

func (r *updateFakeMembers) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	r.writes++
	return errUpdateOpNotUsed
}

func (r *updateFakeMembers) ListMembers(context.Context, uuid.UUID) ([]domain.MemberView, error) {
	return []domain.MemberView{}, nil
}

// updateFixture arma un proyecto de Ana llamado "Portal de turnos" con factor 6, en el estado
// indicado, y otro proyecto propio llamado "Portal de Clientes".
type updateFixture struct {
	service  *ProjectService
	project  domain.Project
	other    domain.Project
	projects *updateFakeProjects
	members  *updateFakeMembers
}

func newUpdateFixture(t *testing.T, status domain.ProjectStatus) updateFixture {
	t.Helper()
	ownerID := uuid.New()
	createdAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	project := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
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
		actualEnd := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
		project.ActualEndDate = &actualEnd
	}
	other := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         "Portal de Clientes",
		NameNormalized:               "portal de clientes",
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 500,
		Status:                       domain.ProjectStatusPlanned,
		CreatedAt:                    createdAt,
		UpdatedAt:                    createdAt,
	}

	projects := &updateFakeProjects{byID: map[uuid.UUID]domain.Project{project.ID: project, other.ID: other}}
	members := &updateFakeMembers{}
	backlog := &viewMockBacklog{}
	backlog.On("Summarize", mock.Anything, mock.Anything).Return(domain.BacklogSummary{CountsByState: map[string]int{}}, nil).Maybe()
	sprints := &viewMockSprints{}
	sprints.On("FindActiveSprint", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	return updateFixture{
		service:  NewProjectService(projects, members, backlog, sprints),
		project:  project,
		other:    other,
		projects: projects,
		members:  members,
	}
}

// inputFrom devuelve los cinco datos editables tal como están en el proyecto, para que cada prueba
// cambie solo lo que necesita, igual que el formulario de edición precargado.
func inputFrom(project domain.Project) UpdateProjectInput {
	return UpdateProjectInput{
		Name:                         project.Name,
		NameNormalized:               project.NameNormalized,
		Description:                  project.Description,
		StartDate:                    project.StartDate,
		PlannedEndDate:               project.PlannedEndDate,
		HoursPerStoryPointHundredths: project.HoursPerStoryPointHundredths,
	}
}

// Escenario: Renombrar con el mismo nombre que ya tenía (002/US5)
// FR-014: la unicidad del nombre excluye al propio proyecto, así que repetir su nombre, también con
// otra capitalización que normaliza igual, se acepta y queda guardado.
func TestUS5_ReglaMismoNombreQueYaTenia(t *testing.T) {
	variants := []struct {
		name           string
		newName        string
		nameNormalized string
	}{
		{name: "nombre idéntico", newName: "Portal de turnos", nameNormalized: "portal de turnos"},
		{name: "otra capitalización", newName: "PORTAL de Turnos", nameNormalized: "portal de turnos"},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := newUpdateFixture(t, domain.ProjectStatusPlanned)
			input := inputFrom(f.project)
			input.Name = v.newName
			input.NameNormalized = v.nameNormalized

			view, err := f.service.Update(context.Background(), f.project.ID, input)

			require.NoError(t, err)
			require.NotNil(t, view)
			assert.Equal(t, v.newName, view.Project.Name)
			assert.Equal(t, v.newName, f.projects.byID[f.project.ID].Name)
		})
	}
}

// Escenario: Nombre de otro proyecto propio (002/US5)
// FR-015: el rechazo por nombre repetido se propaga como ErrProjectNameTaken y no aplica ningún
// cambio parcial, ni siquiera de los demás campos válidos que viajaban junto al nombre.
func TestUS5_ReglaNombreDeOtroProyectoPropio(t *testing.T) {
	f := newUpdateFixture(t, domain.ProjectStatusPlanned)
	input := inputFrom(f.project)
	input.Name = "Portal de Clientes"
	input.NameNormalized = "portal de clientes"
	input.PlannedEndDate = time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	input.HoursPerStoryPointHundredths = 800

	view, err := f.service.Update(context.Background(), f.project.ID, input)

	require.ErrorIs(t, err, domain.ErrProjectNameTaken)
	assert.Nil(t, view)
	assert.Equal(t, f.project, f.projects.byID[f.project.ID], "el proyecto debe quedar como estaba")
	assert.Equal(t, f.other, f.projects.byID[f.other.ID], "el otro proyecto no debe cambiar")
	assert.Zero(t, f.members.writes)
}

// Escenario: Proyecto finalizado (002/US5)
// FR-018, RN-11: un proyecto finalizado es de solo lectura; el rechazo ocurre antes de escribir, así
// que el repositorio no recibe ningún UPDATE y el proyecto queda intacto.
func TestUS5_ReglaProyectoFinalizado(t *testing.T) {
	f := newUpdateFixture(t, domain.ProjectStatusFinished)
	input := inputFrom(f.project)
	input.PlannedEndDate = time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)

	view, err := f.service.Update(context.Background(), f.project.ID, input)

	require.ErrorIs(t, err, domain.ErrProjectReadOnly)
	assert.Nil(t, view)
	assert.Empty(t, f.projects.received, "no debe intentarse ninguna escritura")
	assert.Equal(t, f.project, f.projects.byID[f.project.ID])
}

// Escenario: Modificación exitosa (002/US5)
// FR-012, FR-016: se guardan los cinco datos editables y la modificación no altera el estado ni la
// fecha de finalización real, ni la identidad, el propietario o la fecha de creación. Las variantes
// cubren el proyecto planificado con todos los campos cambiados y el proyecto en curso que solo
// corre la fecha prevista (escenario US5-2: solo finished bloquea).
func TestUS5_ReglaNoAlteraEstadoNiFechaReal(t *testing.T) {
	description := "Turnos online para la clínica"
	variants := []struct {
		name   string
		status domain.ProjectStatus
		change func(input *UpdateProjectInput)
	}{
		{
			name:   "planificado con todos los campos",
			status: domain.ProjectStatusPlanned,
			change: func(input *UpdateProjectInput) {
				input.Name = "Portal de turnos v2"
				input.NameNormalized = "portal de turnos v2"
				input.Description = &description
				input.StartDate = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
				input.PlannedEndDate = time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
				input.HoursPerStoryPointHundredths = 750
			},
		},
		{
			name:   "en curso con la fecha prevista corrida",
			status: domain.ProjectStatusInProgress,
			change: func(input *UpdateProjectInput) {
				input.PlannedEndDate = time.Date(2027, 2, 28, 0, 0, 0, 0, time.UTC)
			},
		},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := newUpdateFixture(t, v.status)
			input := inputFrom(f.project)
			v.change(&input)

			view, err := f.service.Update(context.Background(), f.project.ID, input)

			require.NoError(t, err)
			require.NotNil(t, view)
			require.Len(t, f.projects.received, 1, "la modificación es un único UPDATE")
			saved := f.projects.byID[f.project.ID]
			assert.Equal(t, input.Name, saved.Name)
			assert.Equal(t, input.NameNormalized, saved.NameNormalized)
			assert.Equal(t, input.Description, saved.Description)
			assert.Equal(t, input.StartDate, saved.StartDate)
			assert.Equal(t, input.PlannedEndDate, saved.PlannedEndDate)
			assert.Equal(t, input.HoursPerStoryPointHundredths, saved.HoursPerStoryPointHundredths)
			assert.Equal(t, v.status, saved.Status, "el estado no cambia")
			assert.Nil(t, saved.ActualEndDate, "la fecha de finalización real no cambia")
			assert.Equal(t, f.project.ID, saved.ID)
			assert.Equal(t, f.project.OwnerID, saved.OwnerID)
			assert.Equal(t, f.project.CreatedAt, saved.CreatedAt)
			assert.Equal(t, v.status, view.Project.Status)
		})
	}
}

// Escenario: Cambio de factor con sprints cerrados (002/US5)
// FR-017, RN-15: cambiar el factor de 6 a 8 solo escribe el proyecto. No hay escrituras sobre
// project_members ni sobre ninguna otra tabla, así que los sprints cerrados conservan el factor que
// tenían; los puertos de backlog y de sprints solo se leen para armar la vista.
func TestUS5_ReglaCambioDeFactorConSprintsCerrados(t *testing.T) {
	f := newUpdateFixture(t, domain.ProjectStatusInProgress)
	input := inputFrom(f.project)
	input.HoursPerStoryPointHundredths = 800

	view, err := f.service.Update(context.Background(), f.project.ID, input)

	require.NoError(t, err)
	require.NotNil(t, view)
	require.Len(t, f.projects.received, 1, "solo se escribe el proyecto, una vez")
	assert.Equal(t, 800, f.projects.byID[f.project.ID].HoursPerStoryPointHundredths)
	assert.Equal(t, f.other, f.projects.byID[f.other.ID], "ningún otro proyecto cambia")
	assert.Zero(t, f.members.writes, "la modificación no escribe en project_members")
}

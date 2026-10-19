package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas unitarias obligatorias de las reglas de negocio del alta y la baja de integrantes (T043):
// RN-02, RN-05, RN-11, RN-12 y RN-13. Los repositorios y el directorio de usuarios se reemplazan por
// dobles en memoria que respetan los contratos del dominio, de modo que cada prueba aísla la regla
// del servicio sin tocar la base de datos.

// errMemberOpNotUsed señala una operación que el alta y la baja de integrantes no deberían invocar.
var errMemberOpNotUsed = errors.New("operación no usada por el alta y la baja de integrantes")

// memberFakeProjects guarda proyectos en memoria y respeta el contrato de FindByID: un identificador
// inexistente devuelve ErrProjectNotFound, nunca (nil, nil).
type memberFakeProjects struct {
	byID map[uuid.UUID]domain.Project
}

func (r *memberFakeProjects) Create(context.Context, *domain.Project, *domain.ProjectMember) error {
	return errMemberOpNotUsed
}

func (r *memberFakeProjects) FindByID(_ context.Context, id uuid.UUID) (*domain.Project, error) {
	project, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return &project, nil
}

func (r *memberFakeProjects) Update(context.Context, *domain.Project) error {
	return errMemberOpNotUsed
}

func (r *memberFakeProjects) UpdateStatus(context.Context, *domain.Project) error {
	return errMemberOpNotUsed
}

func (r *memberFakeProjects) ListForMember(context.Context, uuid.UUID) ([]domain.ProjectListItem, error) {
	return nil, errMemberOpNotUsed
}

// memberKey identifica una membresía por el par (project_id, user_id) del índice único.
type memberKey struct {
	projectID uuid.UUID
	userID    uuid.UUID
}

// memberFakeMembers simula project_members en memoria. Create reproduce el índice único
// (project_id, user_id) devolviendo ErrAlreadyMember ante un par repetido, igual que la traducción
// de gorm.ErrDuplicatedKey del repositorio real, y completa CreatedAt como lo haría GORM. Delete
// devuelve ErrNotMember si no hay fila. Cuenta las llamadas para verificar que una acción rechazada
// no escribió nada.
type memberFakeMembers struct {
	rows        map[memberKey]domain.ProjectMember
	createCalls int
	deleteCalls int
}

func (r *memberFakeMembers) FindByProjectAndUser(context.Context, uuid.UUID, uuid.UUID) (*domain.ProjectMember, error) {
	return nil, errMemberOpNotUsed
}

func (r *memberFakeMembers) Create(_ context.Context, member *domain.ProjectMember) error {
	r.createCalls++
	key := memberKey{projectID: member.ProjectID, userID: member.UserID}
	if _, exists := r.rows[key]; exists {
		return domain.ErrAlreadyMember
	}
	member.CreatedAt = time.Date(2026, 10, 3, 9, 15, 0, 0, time.UTC)
	member.UpdatedAt = member.CreatedAt
	r.rows[key] = *member
	return nil
}

func (r *memberFakeMembers) Delete(_ context.Context, projectID, userID uuid.UUID) error {
	r.deleteCalls++
	key := memberKey{projectID: projectID, userID: userID}
	if _, exists := r.rows[key]; !exists {
		return domain.ErrNotMember
	}
	delete(r.rows, key)
	return nil
}

func (r *memberFakeMembers) ListMembers(context.Context, uuid.UUID) ([]domain.MemberView, error) {
	return nil, errMemberOpNotUsed
}

// has indica si el usuario tiene membresía en el proyecto.
func (r *memberFakeMembers) has(projectID, userID uuid.UUID) bool {
	_, exists := r.rows[memberKey{projectID: projectID, userID: userID}]
	return exists
}

// memberFakeUsers es el directorio de usuarios en memoria, indexado por correo ya normalizado.
// Registra cada correo recibido para verificar que el servicio normaliza antes de buscar; un correo
// desconocido devuelve ErrUserNotRegistered, como exige el contrato de UserDirectory.
type memberFakeUsers struct {
	byEmail  map[string]domain.UserRef
	lookedUp []string
}

func (d *memberFakeUsers) FindByEmail(_ context.Context, normalizedEmail string) (*domain.UserRef, error) {
	d.lookedUp = append(d.lookedUp, normalizedEmail)
	user, ok := d.byEmail[normalizedEmail]
	if !ok {
		return nil, domain.ErrUserNotRegistered
	}
	return &user, nil
}

// memberFixture arma un proyecto de Ana (propietaria) con Carla como integrante no propietaria, y a
// Bruno registrado como cuenta pero todavía fuera del proyecto, en el estado indicado.
type memberFixture struct {
	service   *MemberService
	projectID uuid.UUID
	owner     domain.UserRef
	carla     domain.UserRef
	bruno     domain.UserRef
	members   *memberFakeMembers
	users     *memberFakeUsers
}

func newMemberFixture(t *testing.T, status domain.ProjectStatus) memberFixture {
	t.Helper()
	owner := domain.UserRef{ID: uuid.New(), FullName: "Ana Pérez", Email: "ana@example.com"}
	carla := domain.UserRef{ID: uuid.New(), FullName: "Carla Díaz", Email: "carla@example.com"}
	bruno := domain.UserRef{ID: uuid.New(), FullName: "Bruno Gómez", Email: "bruno@example.com"}
	projectID := uuid.New()

	projects := &memberFakeProjects{byID: map[uuid.UUID]domain.Project{
		projectID: {
			ID:                           projectID,
			OwnerID:                      owner.ID,
			Name:                         "Portal de turnos",
			NameNormalized:               "portal de turnos",
			StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			PlannedEndDate:               time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
			HoursPerStoryPointHundredths: 650,
			Status:                       status,
		},
	}}
	members := &memberFakeMembers{rows: map[memberKey]domain.ProjectMember{}}
	for _, userID := range []uuid.UUID{owner.ID, carla.ID} {
		members.rows[memberKey{projectID: projectID, userID: userID}] = domain.ProjectMember{
			ID: uuid.New(), ProjectID: projectID, UserID: userID,
		}
	}
	users := &memberFakeUsers{byEmail: map[string]domain.UserRef{
		owner.Email: owner,
		carla.Email: carla,
		bruno.Email: bruno,
	}}

	return memberFixture{
		service:   NewMemberService(projects, members, users),
		projectID: projectID,
		owner:     owner,
		carla:     carla,
		bruno:     bruno,
		members:   members,
		users:     users,
	}
}

// Escenario: Correo con espacios y mayúsculas (002/US3)
// RN-05: antes de buscar la cuenta el correo se recorta con TrimSpace y se pasa a minúsculas con
// ToLower, así que cualquier variante de bruno@example.com encuentra la misma cuenta y la deja como
// integrante no propietario, con la fecha de incorporación de su membresía.
func TestUS3_CorreoConEspaciosYMayusculas(t *testing.T) {
	variants := []struct {
		name  string
		email string
	}{
		{name: "espacios extremos y mayúsculas mezcladas", email: "  Bruno@Example.COM  "},
		{name: "todo en mayúsculas", email: "BRUNO@EXAMPLE.COM"},
		{name: "tabulador y salto de línea extremos", email: "\tbruno@example.com\n"},
		{name: "ya normalizado", email: "bruno@example.com"},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := newMemberFixture(t, domain.ProjectStatusPlanned)

			view, err := f.service.AddMember(context.Background(), f.projectID, f.owner.ID, v.email)

			require.NoError(t, err)
			assert.Equal(t, []string{"bruno@example.com"}, f.users.lookedUp)
			assert.True(t, f.members.has(f.projectID, f.bruno.ID))
			require.NotNil(t, view)
			assert.Equal(t, f.bruno.ID, view.UserID)
			assert.Equal(t, f.bruno.FullName, view.FullName)
			assert.Equal(t, f.bruno.Email, view.Email)
			assert.False(t, view.IsOwner)
			assert.False(t, view.JoinedAt.IsZero())
		})
	}
}

// Escenario: No es propietario (002/US3)
// RN-02: solo el propietario agrega y quita integrantes. Carla es integrante pero no propietaria, y
// tanto el alta como la baja que pide se rechazan con ErrNotProjectOwner sin escribir nada.
func TestUS3_NoEsPropietario(t *testing.T) {
	t.Run("alta pedida por un integrante no propietario", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusInProgress)

		view, err := f.service.AddMember(context.Background(), f.projectID, f.carla.ID, f.bruno.Email)

		require.ErrorIs(t, err, domain.ErrNotProjectOwner)
		assert.Nil(t, view)
		assert.Zero(t, f.members.createCalls)
		assert.False(t, f.members.has(f.projectID, f.bruno.ID))
	})

	t.Run("baja pedida por un integrante no propietario", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusInProgress)

		err := f.service.RemoveMember(context.Background(), f.projectID, f.carla.ID, f.carla.ID)

		require.ErrorIs(t, err, domain.ErrNotProjectOwner)
		assert.Zero(t, f.members.deleteCalls)
		assert.True(t, f.members.has(f.projectID, f.carla.ID))
	})
}

// Escenario: Proyecto finalizado (002/US3)
// RN-11: un proyecto finished es de solo lectura, así que el propietario no puede agregar ni quitar
// integrantes: las dos acciones devuelven ErrProjectReadOnly y la lista no cambia. Como control, en
// planned e in_progress el propietario sí agrega y quita.
func TestUS3_ProyectoFinalizado(t *testing.T) {
	t.Run("alta sobre un proyecto finalizado", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusFinished)

		view, err := f.service.AddMember(context.Background(), f.projectID, f.owner.ID, f.bruno.Email)

		require.ErrorIs(t, err, domain.ErrProjectReadOnly)
		assert.Nil(t, view)
		assert.Zero(t, f.members.createCalls)
		assert.False(t, f.members.has(f.projectID, f.bruno.ID))
	})

	t.Run("baja sobre un proyecto finalizado", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusFinished)

		err := f.service.RemoveMember(context.Background(), f.projectID, f.owner.ID, f.carla.ID)

		require.ErrorIs(t, err, domain.ErrProjectReadOnly)
		assert.Zero(t, f.members.deleteCalls)
		assert.True(t, f.members.has(f.projectID, f.carla.ID))
	})

	for _, status := range []domain.ProjectStatus{domain.ProjectStatusPlanned, domain.ProjectStatusInProgress} {
		t.Run("alta y baja permitidas en "+string(status), func(t *testing.T) {
			f := newMemberFixture(t, status)

			_, addErr := f.service.AddMember(context.Background(), f.projectID, f.owner.ID, f.bruno.Email)
			removeErr := f.service.RemoveMember(context.Background(), f.projectID, f.owner.ID, f.carla.ID)

			require.NoError(t, addErr)
			require.NoError(t, removeErr)
			assert.True(t, f.members.has(f.projectID, f.bruno.ID))
			assert.False(t, f.members.has(f.projectID, f.carla.ID))
		})
	}
}

// Escenario: El propietario no puede quitarse (002/US3)
// RN-12: la baja del propietario se rechaza con ErrOwnerCannotLeave y su membresía se conserva. Como
// control, el mismo propietario sí quita a otro integrante, para que la regla alcance solo a sí
// mismo.
func TestUS3_PropietarioNoPuedeQuitarse(t *testing.T) {
	t.Run("el propietario intenta quitarse", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusInProgress)

		err := f.service.RemoveMember(context.Background(), f.projectID, f.owner.ID, f.owner.ID)

		require.ErrorIs(t, err, domain.ErrOwnerCannotLeave)
		assert.Zero(t, f.members.deleteCalls)
		assert.True(t, f.members.has(f.projectID, f.owner.ID))
	})

	t.Run("el propietario quita a otro integrante", func(t *testing.T) {
		f := newMemberFixture(t, domain.ProjectStatusInProgress)

		err := f.service.RemoveMember(context.Background(), f.projectID, f.owner.ID, f.carla.ID)

		require.NoError(t, err)
		assert.False(t, f.members.has(f.projectID, f.carla.ID))
		assert.True(t, f.members.has(f.projectID, f.owner.ID))
	})
}

// Escenario: Ya es integrante (002/US3)
// RN-13: un usuario no puede ser integrante dos veces del mismo proyecto. El alta de Carla, que ya
// es integrante, y la del propio propietario se rechazan con ErrAlreadyMember, también cuando el
// correo llega sin normalizar, y la lista de integrantes no cambia.
func TestUS3_YaEsIntegrante(t *testing.T) {
	variants := []struct {
		name  string
		email func(f memberFixture) string
	}{
		{name: "integrante no propietario", email: func(f memberFixture) string { return f.carla.Email }},
		{name: "integrante con el correo sin normalizar", email: func(memberFixture) string { return "  CARLA@example.com " }},
		{name: "el propio propietario", email: func(f memberFixture) string { return f.owner.Email }},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			f := newMemberFixture(t, domain.ProjectStatusPlanned)
			before := len(f.members.rows)

			view, err := f.service.AddMember(context.Background(), f.projectID, f.owner.ID, v.email(f))

			require.ErrorIs(t, err, domain.ErrAlreadyMember)
			assert.Nil(t, view)
			assert.Len(t, f.members.rows, before)
		})
	}
}

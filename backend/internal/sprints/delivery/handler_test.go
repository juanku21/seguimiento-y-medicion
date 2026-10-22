package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	backlogdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/delivery"
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	backlogservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Pruebas de integración de punta a punta de los sprints: atraviesan el router de Gin compuesto como
// en cmd/app, el middleware de sesión de la 001, RequireMembership de la 002, el servicio y el
// repositorio de sprints contra la base smye_test (Principios II y V). Las de US1 (T013) cubren
// POST /api/v1/projects/{projectId}/sprints; las historias siguientes agregan las suyas en este
// archivo. Los ayudantes llevan el prefijo "sprint" para no chocar con los de validation_test.go,
// que llevan el prefijo "validation".

// Textos exactos que fija el contrato para el 400 de entrada inválida, el 404 de proyecto
// inexistente y el 409 de sprints de solo lectura.
const (
	sprintInvalidFieldsError = "Hay campos inválidos en la solicitud."
	sprintProjectNotFound    = "El proyecto no existe."
	sprintReadOnlyMessage    = "El proyecto está finalizado y sus sprints son de solo lectura."
)

// sprintUserPassword es la contraseña de las cuentas que crean las pruebas; cumple las reglas de la
// 001.
const sprintUserPassword = "Sprints2026"

// Datos del caso normal de la spec (US1, escenario 1).
const (
	sprintName      = "Sprint 1"
	sprintGoal      = "Cerrar el alta de clientes"
	sprintStartDate = "2026-10-01"
	sprintEndDate   = "2026-10-14"
)

// sprintFixedNow es el instante del reloj que se inyecta por defecto en el servicio de sprints.
var sprintFixedNow = time.Date(2026, time.October, 21, 15, 30, 0, 0, time.UTC)

// sprintDetailBody refleja SprintDetail del contrato; se declara en la prueba para no depender de
// los DTO de producción al leer la respuesta.
type sprintDetailBody struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	Status    string `json:"status"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	CreatedBy struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// sprintValidationBody refleja ValidationErrorResponse del contrato.
type sprintValidationBody struct {
	Error  string `json:"error"`
	Fields []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields"`
}

// sprintConflictBody refleja SprintConflictResponse del contrato.
type sprintConflictBody struct {
	Error  string `json:"error"`
	Sprint *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"sprint"`
}

// sprintTestPorts son las dependencias del módulo de sprints que una prueba puede reemplazar: el
// puerto de esfuerzo, por un doble que informe esfuerzo registrado, y el reloj del servicio. db es
// la base de test, de solo lectura para los overrides, para que un doble pueda consultarla.
type sprintTestPorts struct {
	effort domain.SprintEffort
	now    func() time.Time
	db     *gorm.DB
}

// sprintTestEnv reúne el router compuesto como en cmd/app, la base de test y el servicio de
// autenticación con el que se crean las cuentas y las sesiones de cada escenario.
type sprintTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	auth   *authservice.AuthService
}

// newSprintTestEnv abre la base de test vacía y compone, igual que cmd/app, el límite global del
// cuerpo, la cadena real de la 001 (para el middleware de sesión), el módulo de proyectos de la 002
// con su subgrupo /:projectId, el módulo del backlog de la 003 con su grupo de historia protegido
// por RequireStoryAccess y las rutas de sprints colgadas del subgrupo del proyecto, sin CORS ni
// Swagger. Cada override recibe las dependencias por defecto y puede reemplazar el puerto de
// esfuerzo o el reloj.
func newSprintTestEnv(t *testing.T, overrides ...func(*sprintTestPorts)) sprintTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)

	// Cadena de autenticación de la 001.
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	router := gin.New()
	router.Use(httpserver.LimitRequestBody())
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)

	// Módulo de proyectos de la 002, que devuelve el subgrupo /api/v1/projects/:projectId con
	// RequireMembership.
	projectRepository := projectsrepository.NewProjectRepository(db)
	projectMembers := projectsrepository.NewProjectMemberRepository(db)
	projects := projectsservice.NewProjectService(
		projectRepository,
		projectMembers,
		projectsrepository.NewBacklogSummaryProvider(db),
		projectsrepository.NewSprintDirectory(),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(), cfg.Location, time.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// Módulo del backlog de la 003: rutas de colección en el subgrupo del proyecto y grupo plano
	// /api/v1/stories/:storyId con RequireStoryAccess sobre el mismo repositorio de historias.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(),
		backlogrepository.NewEffortRecords(),
		backlogrepository.NewDefectRecords(),
		backlogrepository.NewEstimationSessions(),
		backlogrepository.NewProjectDirectory(db),
	)
	backlogHandler := backlogdelivery.NewHandler(stories)
	backlogdelivery.RegisterRoutes(projectGroup, backlogHandler)
	backlogdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, storyRepository, backlogHandler)

	// Módulo de sprints con el puerto de esfuerzo y el reloj por defecto o los que pida la prueba.
	ports := sprintTestPorts{
		effort: repository.NewSprintEffort(),
		now:    func() time.Time { return sprintFixedNow },
		db:     db,
	}
	for _, override := range overrides {
		override(&ports)
	}
	sprints := service.NewSprintService(repository.NewSprintRepository(db), ports.effort, cfg.Location, ports.now)
	RegisterRoutes(projectGroup, NewHandler(sprints))
	return sprintTestEnv{router: router, db: db, auth: auth}
}

// signUp registra una cuenta con el servicio de la 001, inicia su sesión y devuelve el identificador
// de la cuenta y el token de la sesión.
func (e sprintTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	user, err := e.auth.Register(ctx, fullName, email, sprintUserPassword)
	require.NoError(t, err, "no se pudo registrar la cuenta de prueba %s", email)
	login, err := e.auth.Login(ctx, email, sprintUserPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de prueba %s", email)
	return user.ID, login.Token
}

// sprintSeedProject inserta directamente en la base un proyecto del propietario en el estado
// indicado, con las membresías del propietario y de cada integrante adicional, y devuelve su
// identificador, para preparar el "Dado" sin pasar por la API de la 002.
func sprintSeedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, status projectsdomain.ProjectStatus, extraMembers ...uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	project := projectsdomain.Project{
		ID:                           id,
		OwnerID:                      ownerID,
		Name:                         "Portal de Clientes " + id.String()[:8],
		NameNormalized:               "portal de clientes " + id.String()[:8],
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       status,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto")
	for _, userID := range append([]uuid.UUID{ownerID}, extraMembers...) {
		member := projectsdomain.ProjectMember{ID: uuid.New(), ProjectID: project.ID, UserID: userID}
		require.NoError(t, db.Create(&member).Error, "no se pudo sembrar la membresía de %s", userID)
	}
	return project.ID
}

// sprintDate convierte una fecha YYYY-MM-DD a medianoche UTC, como se guardan las fechas del sprint.
func sprintDate(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, value)
	require.NoError(t, err, "fecha de prueba inválida %q", value)
	return d
}

// sprintSeed inserta directamente en la base un sprint Planificado del proyecto con el período
// indicado, creado por el autor recibido, y devuelve su identificador, para preparar el "Dado" sin
// depender del endpoint bajo prueba.
func sprintSeed(t *testing.T, db *gorm.DB, projectID, authorID uuid.UUID, name, start, end string) uuid.UUID {
	t.Helper()
	sprint := domain.Sprint{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        name,
		Goal:        sprintGoal,
		StartDate:   sprintDate(t, start),
		EndDate:     sprintDate(t, end),
		Status:      domain.SprintStatusPlanned,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&sprint).Error, "no se pudo sembrar el sprint %s", name)
	return sprint.ID
}

// sprintsPath es la ruta de creación y listado de sprints del proyecto según contracts/openapi.yaml.
func sprintsPath(projectID uuid.UUID) string {
	return "/api/v1/projects/" + projectID.String() + "/sprints"
}

// sprintBody arma el cuerpo de la creación con los cuatro campos; cada prueba modifica o quita los
// que le interesan.
func sprintBody(name, goal, start, end string) map[string]any {
	return map[string]any{"name": name, "goal": goal, "startDate": start, "endDate": end}
}

// sprintNormalBody devuelve el cuerpo del caso normal de US1.
func sprintNormalBody() map[string]any {
	return sprintBody(sprintName, sprintGoal, sprintStartDate, sprintEndDate)
}

// sprintDo envía una solicitud con cuerpo JSON opcional y el token como Bearer, y devuelve la
// respuesta grabada.
func (e sprintTestEnv) sprintDo(t *testing.T, method, path, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err, "no se pudo serializar la solicitud")
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// postSprint envía el cuerpo a POST /api/v1/projects/{projectId}/sprints con el token como Bearer.
func (e sprintTestEnv) postSprint(t *testing.T, token string, projectID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodPost, sprintsPath(projectID), token, body)
}

// requireCreatedSprint exige un 201 cuyo cuerpo sea un SprintDetail con id UUID y lo devuelve
// decodificado.
func requireCreatedSprint(t *testing.T, rec *httptest.ResponseRecorder) sprintDetailBody {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 201 no es JSON válido: %s", rec.Body.String())
	_, err := uuid.Parse(body.ID)
	require.NoError(t, err, "id = %q no es un UUID", body.ID)
	return body
}

// sprintRawBody decodifica la respuesta como objeto crudo, para comprobar que un campo vale null o
// [] y no que simplemente falta.
func sprintRawBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw), "el cuerpo no es JSON válido: %s", rec.Body.String())
	return raw
}

// sprintRequireFieldRejected exige un 400 con ValidationErrorResponse que señale el campo indicado y
// cuyo mensaje mencione cada una de las piezas esperadas, sin distinguir mayúsculas.
func sprintRequireFieldRejected(t *testing.T, rec *httptest.ResponseRecorder, field string, parts ...string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintValidationBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 400 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, sprintInvalidFieldsError, body.Error)
	for _, f := range body.Fields {
		if f.Field != field {
			continue
		}
		msg := strings.ToLower(f.Message)
		assert.NotEmpty(t, msg, "el mensaje del campo %q no puede estar vacío", field)
		for _, part := range parts {
			assert.Contains(t, msg, strings.ToLower(part), "el mensaje de %s debería mencionar %q", field, part)
		}
		return
	}
	t.Errorf("la respuesta no señala el campo %q: %s", field, rec.Body.String())
}

// sprintCount cuenta los sprints guardados del proyecto, para verificar que un rechazo no creó
// ninguno (FR-011).
func sprintCount(t *testing.T, db *gorm.DB, projectID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.Sprint{}).Where("project_id = ?", projectID).Count(&n).Error,
		"no se pudieron contar los sprints")
	return n
}

// sprintStored lee de sprints la fila con el identificador devuelto por la API.
func sprintStored(t *testing.T, db *gorm.DB, id string) domain.Sprint {
	t.Helper()
	var sprint domain.Sprint
	require.NoError(t, db.First(&sprint, "id = ?", id).Error, "no se encontró el sprint %s en sprints", id)
	return sprint
}

// Escenario: Creación en un proyecto En curso (004/US1)
// Bruno, integrante que no es el propietario de un proyecto En curso sin sprints, crea "Sprint 1"
// del 2026-10-01 al 2026-10-14 con el objetivo "Cerrar el alta de clientes": responde 201 con el
// sprint planned, sin historias ([] y no null), sin datos de inicio ni de cierre y atribuido a Bruno,
// y la fila queda guardada igual (FR-006, FR-009, FR-060, RN-01).
func TestUS1_CreacionEnProyectoEnCurso(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)

	rec := env.postSprint(t, brunoToken, projectID, sprintNormalBody())

	// La respuesta es el detalle del sprint recién creado.
	body := requireCreatedSprint(t, rec)
	assert.Equal(t, projectID.String(), body.ProjectID)
	assert.Equal(t, sprintName, body.Name)
	assert.Equal(t, sprintGoal, body.Goal)
	assert.Equal(t, "planned", body.Status)
	assert.Equal(t, sprintStartDate, body.StartDate)
	assert.Equal(t, sprintEndDate, body.EndDate)
	assert.Equal(t, brunoID.String(), body.CreatedBy.ID, "el autor es el titular de la sesión")
	assert.Equal(t, "Bruno Díaz", body.CreatedBy.FullName)

	// Las historias vienen como lista vacía y los datos de inicio y de cierre como null.
	raw := sprintRawBody(t, rec)
	assert.Equal(t, []any{}, raw["stories"], "stories debe ser [] y no null")
	for _, key := range []string{"startedAt", "closedAt", "closedBy", "results", "hoursPerStoryPoint", "schedule"} {
		value, present := raw[key]
		assert.True(t, present, "falta el campo %q en el detalle", key)
		assert.Nil(t, value, "el campo %q debe ser null en un sprint planificado", key)
	}

	// La fila guardada coincide con la respuesta.
	stored := sprintStored(t, env.db, body.ID)
	assert.Equal(t, projectID, stored.ProjectID)
	assert.Equal(t, domain.SprintStatusPlanned, stored.Status)
	assert.Equal(t, brunoID, stored.CreatedByID)
	assert.Equal(t, sprintDate(t, sprintStartDate), stored.StartDate.UTC())
	assert.Equal(t, sprintDate(t, sprintEndDate), stored.EndDate.UTC())
	assert.Nil(t, stored.CommittedStoryPoints)
	assert.Nil(t, stored.StartedAt)
	assert.Nil(t, stored.ClosedAt)
	assert.EqualValues(t, 1, sprintCount(t, env.db, projectID))
}

// Escenario: Proyecto Planificado (004/US1)
// En un proyecto en estado Planificado la creación se acepta y el proyecto sigue planned, leído con
// GET /api/v1/projects/{projectId} de la 002: crear un sprint no cambia el estado del proyecto
// (FR-049, RN-07).
func TestUS1_ProyectoPlanificado(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusPlanned)

	rec := env.postSprint(t, anaToken, projectID, sprintNormalBody())

	body := requireCreatedSprint(t, rec)
	assert.Equal(t, "planned", body.Status)

	// El proyecto sigue Planificado según la API de la 002.
	projectRec := env.sprintDo(t, http.MethodGet, "/api/v1/projects/"+projectID.String(), anaToken, nil)
	require.Equal(t, http.StatusOK, projectRec.Code, "cuerpo: %s", projectRec.Body.String())
	var project struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(projectRec.Body.Bytes(), &project))
	assert.Equal(t, string(projectsdomain.ProjectStatusPlanned), project.Status, "el proyecto debe seguir planned")
}

// Escenario: Varios sprints planificados (004/US1)
// En un proyecto que ya tiene un sprint Planificado, crear otro con fechas que no se superponen se
// acepta y ambos coexisten en estado Planificado (FR-005, FR-006).
func TestUS1_VariosPlanificados(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	firstID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")

	rec := env.postSprint(t, anaToken, projectID, sprintBody("Sprint 2", sprintGoal, "2026-10-20", "2026-11-02"))

	body := requireCreatedSprint(t, rec)
	assert.Equal(t, "planned", body.Status)
	assert.EqualValues(t, 2, sprintCount(t, env.db, projectID))
	assert.Equal(t, domain.SprintStatusPlanned, sprintStored(t, env.db, firstID.String()).Status)
	assert.Equal(t, domain.SprintStatusPlanned, sprintStored(t, env.db, body.ID).Status)
}

// Escenario: Fechas contiguas (004/US1)
// Con un sprint que termina el 2026-10-14, crear otro que empieza el 2026-10-14 se acepta: compartir
// solo el día de borde no es superposición (FR-005, RN-05).
func TestUS1_FechasContiguas(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")

	rec := env.postSprint(t, anaToken, projectID, sprintBody("Sprint 2", sprintGoal, "2026-10-14", "2026-10-28"))

	body := requireCreatedSprint(t, rec)
	assert.Equal(t, "2026-10-14", body.StartDate)
	assert.EqualValues(t, 2, sprintCount(t, env.db, projectID))
}

// Escenario: Límite del Sprint Goal (004/US1)
// Un Sprint Goal de exactamente 500 caracteres se acepta y se guarda completo; uno de 501 se rechaza
// en el campo goal indicando el máximo, y no se crea nada (FR-003, FR-011, RN-03).
func TestUS1_LimiteDelSprintGoal(t *testing.T) {
	t.Run("500 caracteres", func(t *testing.T) {
		env := newSprintTestEnv(t)
		anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
		goal := strings.Repeat("ñ", 500)

		rec := env.postSprint(t, anaToken, projectID, sprintBody(sprintName, goal, sprintStartDate, sprintEndDate))

		body := requireCreatedSprint(t, rec)
		assert.Equal(t, goal, body.Goal)
		assert.Equal(t, goal, sprintStored(t, env.db, body.ID).Goal)
	})

	t.Run("501 caracteres", func(t *testing.T) {
		env := newSprintTestEnv(t)
		anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)

		rec := env.postSprint(t, anaToken, projectID, sprintBody(sprintName, strings.Repeat("ñ", 501), sprintStartDate, sprintEndDate))

		sprintRequireFieldRejected(t, rec, "goal", "500")
		assert.Zero(t, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint")
	})
}

// Escenario: Duración mínima (004/US1)
// Un sprint cuya fecha de fin es el día siguiente al de inicio se crea, porque es la duración mínima
// admitida (FR-004, RN-04).
func TestUS1_DuracionMinima(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)

	rec := env.postSprint(t, anaToken, projectID, sprintBody(sprintName, sprintGoal, "2026-10-01", "2026-10-02"))

	body := requireCreatedSprint(t, rec)
	assert.Equal(t, "2026-10-01", body.StartDate)
	assert.Equal(t, "2026-10-02", body.EndDate)
	assert.EqualValues(t, 1, sprintCount(t, env.db, projectID))
}

// Escenario: Campos obligatorios (004/US1)
// Sin nombre, sin Sprint Goal, sin fecha de inicio o sin fecha de fin, la creación se rechaza con
// 400 señalando el campo que falta, y no se crea nada (FR-001, FR-011, RN-03, RN-04).
func TestUS1_CamposObligatorios(t *testing.T) {
	cases := []struct {
		name  string
		field string
	}{
		{"sin nombre", "name"},
		{"sin Sprint Goal", "goal"},
		{"sin fecha de inicio", "startDate"},
		{"sin fecha de fin", "endDate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newSprintTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			body := sprintNormalBody()
			delete(body, tc.field)

			rec := env.postSprint(t, anaToken, projectID, body)

			sprintRequireFieldRejected(t, rec, tc.field, "obligatori")
			assert.Zero(t, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint")
		})
	}
}

// Escenario: Fin no posterior al inicio (004/US1)
// Una fecha de fin igual o anterior a la de inicio se rechaza en el campo endDate indicando que debe
// ser posterior, y no se crea nada (FR-004, FR-011).
func TestUS1_FinNoPosteriorAlInicio(t *testing.T) {
	cases := []struct {
		name string
		end  string
	}{
		{"igual", "2026-10-01"},
		{"anterior", "2026-09-30"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newSprintTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)

			rec := env.postSprint(t, anaToken, projectID, sprintBody(sprintName, sprintGoal, "2026-10-01", tc.end))

			sprintRequireFieldRejected(t, rec, "endDate", "posterior")
			assert.Zero(t, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint")
		})
	}
}

// Escenario: Superposición (004/US1)
// Con un sprint del 2026-10-01 al 2026-10-14, crear otro del 2026-10-10 al 2026-10-20 se rechaza con
// 409 indicando el identificador y el nombre del sprint con el que se superpone, y no se crea nada
// (FR-005, FR-011, SC-004).
func TestUS1_Superposicion(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	existingID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")

	rec := env.postSprint(t, anaToken, projectID, sprintBody("Sprint 2", sprintGoal, "2026-10-10", "2026-10-20"))

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintConflictBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
	assert.NotEmpty(t, body.Error)
	assert.Contains(t, body.Error, "Sprint 1", "el mensaje debe nombrar el sprint en conflicto")
	if assert.NotNil(t, body.Sprint, "falta el sprint en conflicto: %s", rec.Body.String()) {
		assert.Equal(t, existingID.String(), body.Sprint.ID)
		assert.Equal(t, "Sprint 1", body.Sprint.Name)
	}
	assert.EqualValues(t, 1, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint nuevo")
}

// Escenario: Proyecto finalizado (004/US1)
// En un proyecto Finalizado la creación de un integrante se rechaza con 409 porque sus sprints son de
// solo lectura, y no se crea nada (FR-059, RN-19).
func TestUS1_ProyectoFinalizado(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusFinished, brunoID)

	rec := env.postSprint(t, brunoToken, projectID, sprintNormalBody())

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintReadOnlyMessage+`"}`, rec.Body.String())
	assert.Zero(t, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint")
}

// Escenario: No integrante (004/US1)
// Quien no es integrante del proyecto recibe 404 de proyecto inexistente, con el mismo código, tipo
// de contenido y cuerpo, byte a byte, que ante un projectId que no existe, y no se crea nada
// (FR-058, SC-014).
func TestUS1_NoIntegrante(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)

	foreignRec := env.postSprint(t, outsiderToken, projectID, sprintNormalBody())
	missingRec := env.postSprint(t, outsiderToken, uuid.New(), sprintNormalBody())

	assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintProjectNotFound+`"}`, foreignRec.Body.String())
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	assert.Zero(t, sprintCount(t, env.db, projectID), "no debería haberse creado ningún sprint")
}

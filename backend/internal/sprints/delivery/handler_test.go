package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
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
// POST /api/v1/projects/{projectId}/sprints; las de US2 (T028) cubren POST y DELETE
// /api/v1/sprints/{sprintId}/stories detrás de RequireSprintAccess y la restricción de la 003 sobre
// historias comprometidas; las de US3 (T041) cubren POST /api/v1/sprints/{sprintId}/start; las de
// US4 (T055) cubren PUT /api/v1/stories/{storyId}/status detrás del RequireStoryAccess de la 003; las
// historias siguientes agregan las suyas en este archivo. Los ayudantes llevan el prefijo "sprint" para no chocar con los de validation_test.go,
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
		projectsrepository.NewSprintDirectory(db),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(db), cfg.Location, time.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// Módulo del backlog de la 003: rutas de colección en el subgrupo del proyecto y grupo plano
	// /api/v1/stories/:storyId con RequireStoryAccess sobre el mismo repositorio de historias.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(db),
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
	// El mismo repositorio sirve al servicio y a RequireSprintAccess del grupo plano
	// /api/v1/sprints/:sprintId, que exige la sesión real de la 001.
	sprintRepository := repository.NewSprintRepository(db)
	sprints := service.NewSprintService(sprintRepository, ports.effort, cfg.Location, ports.now)
	sprintHandler := NewHandler(sprints)
	RegisterRoutes(projectGroup, sprintHandler)
	RegisterSprintRoutes(router, authHandler.RequireSession, sprintRepository, sprintHandler)
	// Las rutas de sprints sobre una historia reciben, como en cmd/app, la sesión de la 001 y el
	// RequireStoryAccess de la 003 sobre el mismo repositorio de historias.
	RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), sprintHandler)
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

// Textos exactos que fija el contrato para los rechazos de la asignación y la baja de historias
// (US2): el 404 de historia inexistente, el 409 de sprint cerrado, el del lote no admitido, el de la
// historia Completada que se quiere quitar y el 409 de la 003 al quitarle a una historia comprometida
// sus Story Points o su último criterio.
const (
	sprintStoryNotFound      = "La historia no existe."
	sprintClosedMessage      = "El sprint está cerrado y no admite cambios."
	sprintNotAdmittedMessage = "Algunas historias no se pueden comprometer en el sprint; no se asignó ninguna."
	sprintCompletedRemoval   = "No se puede quitar una historia Completada. Primero devolvela a En progreso."
	sprintBacklogCommitted   = "La historia está comprometida en un sprint abierto; primero hay que quitarla del sprint."
)

// sprintCriterion es el criterio de aceptación con el que se crean las historias listas para
// planificar.
const sprintCriterion = "El reporte muestra la velocidad de los últimos cinco sprints."

// sprintStoriesDetailBody refleja de SprintDetail del contrato solo lo que verifican las pruebas de
// US2: el estado del sprint y sus historias comprometidas.
type sprintStoriesDetailBody struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Stories []struct {
		StoryID     string  `json:"storyId"`
		StoryPoints int     `json:"storyPoints"`
		Status      string  `json:"status"`
		CompletedAt *string `json:"completedAt"`
		Outcome     *string `json:"outcome"`
	} `json:"stories"`
}

// sprintNotAdmittedBody refleja StoriesNotAdmittedResponse del contrato, con un AdmissionProblem por
// historia rechazada.
type sprintNotAdmittedBody struct {
	Error   string `json:"error"`
	Stories []struct {
		StoryID string   `json:"storyId"`
		Reason  string   `json:"reason"`
		Missing []string `json:"missing"`
		Sprint  *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"sprint"`
		Message string `json:"message"`
	} `json:"stories"`
}

// sprintPoints devuelve un puntero a los Story Points indicados, para los cuerpos y las siembras.
func sprintPoints(n int) *int {
	return &n
}

// createStory crea una historia en el backlog del proyecto con el endpoint real de la 003, con los
// Story Points (nil es "sin estimar") y los criterios recibidos, y devuelve su identificador.
func (e sprintTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, title string, points *int, criteria ...string) uuid.UUID {
	t.Helper()
	body := map[string]any{"title": title, "priority": "high", "storyPoints": nil, "acceptanceCriteria": criteria}
	if points != nil {
		body["storyPoints"] = *points
	}
	rec := e.sprintDo(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, body)
	require.Equal(t, http.StatusCreated, rec.Code, "no se pudo crear la historia de prueba: %s", rec.Body.String())
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "el cuerpo del 201 de la 003 no es JSON válido")
	id, err := uuid.Parse(created.ID)
	require.NoError(t, err, "id = %q no es un UUID", created.ID)
	return id
}

// createReadyStory crea con la 003 una historia lista para planificar: 5 Story Points y un criterio.
func (e sprintTestEnv) createReadyStory(t *testing.T, token string, projectID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	return e.createStory(t, token, projectID, title, sprintPoints(5), sprintCriterion)
}

// sprintSetStoryStatus fija directamente en la base el estado de una historia, para preparar los
// "Dado" con historias en progreso o completadas, que la 003 no permite alcanzar por su API.
func sprintSetStoryStatus(t *testing.T, db *gorm.DB, storyID uuid.UUID, status backlogdomain.StoryStatus) {
	t.Helper()
	require.NoError(t, db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).Update("status", status).Error,
		"no se pudo fijar el estado de la historia")
}

// sprintSeedInStatus inserta directamente en la base un sprint del proyecto en el estado indicado,
// con los datos de inicio si está activo o cerrado y, si está cerrado, también los de cierre, y
// devuelve su identificador.
func sprintSeedInStatus(t *testing.T, db *gorm.DB, projectID, authorID uuid.UUID, name, start, end string, status domain.SprintStatus) uuid.UUID {
	t.Helper()
	id := sprintSeed(t, db, projectID, authorID, name, start, end)
	if status == domain.SprintStatusPlanned {
		return id
	}
	updates := map[string]any{"status": status, "started_at": sprintDate(t, start).Add(9 * time.Hour), "committed_story_points": 5}
	if status == domain.SprintStatusClosed {
		updates["closed_at"] = sprintDate(t, end).Add(18 * time.Hour)
		updates["closed_by_id"] = authorID
		updates["hours_per_story_point_hundredths"] = 650
	}
	require.NoError(t, db.Model(&domain.Sprint{}).Where("id = ?", id).Updates(updates).Error,
		"no se pudo fijar el estado del sprint %s", name)
	return id
}

// sprintSeedCommitment inserta directamente en la base el compromiso de una historia en un sprint y
// marca la historia con ever_committed, como lo dejaría una asignación. Con outcome, la fila queda
// congelada como la de un sprint cerrado; con completedAt, registra el completado dentro del sprint.
func sprintSeedCommitment(t *testing.T, db *gorm.DB, sprintID, storyID uuid.UUID, committedAt time.Time, outcome *domain.StoryOutcome, completedAt *time.Time) {
	t.Helper()
	row := domain.SprintStory{
		ID:          uuid.New(),
		SprintID:    sprintID,
		StoryID:     storyID,
		CommittedAt: committedAt,
		CompletedAt: completedAt,
		Outcome:     outcome,
	}
	if outcome != nil {
		title := "Historia congelada"
		priority := backlogdomain.StoryPriorityHigh
		row.TitleAtClose = &title
		row.PriorityAtClose = &priority
		row.StoryPointsAtClose = sprintPoints(5)
	}
	require.NoError(t, db.Create(&row).Error, "no se pudo sembrar el compromiso de la historia")
	require.NoError(t, db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).Update("ever_committed", true).Error,
		"no se pudo marcar ever_committed")
}

// sprintPath es la ruta del detalle de un sprint según contracts/openapi.yaml.
func sprintPath(sprintID uuid.UUID) string {
	return "/api/v1/sprints/" + sprintID.String()
}

// sprintStoriesPath es la ruta de asignación y baja de historias del sprint según
// contracts/openapi.yaml.
func sprintStoriesPath(sprintID uuid.UUID) string {
	return sprintPath(sprintID) + "/stories"
}

// assignStories envía POST /api/v1/sprints/{sprintId}/stories con los identificadores recibidos.
func (e sprintTestEnv) assignStories(t *testing.T, token string, sprintID uuid.UUID, storyIDs ...uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	ids := make([]string, 0, len(storyIDs))
	for _, id := range storyIDs {
		ids = append(ids, id.String())
	}
	return e.sprintDo(t, http.MethodPost, sprintStoriesPath(sprintID), token, map[string]any{"storyIds": ids})
}

// removeStories envía DELETE /api/v1/sprints/{sprintId}/stories con un parámetro storyId por
// historia.
func (e sprintTestEnv) removeStories(t *testing.T, token string, sprintID uuid.UUID, storyIDs ...uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	params := make([]string, 0, len(storyIDs))
	for _, id := range storyIDs {
		params = append(params, "storyId="+id.String())
	}
	return e.sprintDo(t, http.MethodDelete, sprintStoriesPath(sprintID)+"?"+strings.Join(params, "&"), token, nil)
}

// requireSprintStories exige un 200 cuyo cuerpo sea el detalle del sprint y lo devuelve decodificado.
func requireSprintStories(t *testing.T, rec *httptest.ResponseRecorder) sprintStoriesDetailBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintStoriesDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body
}

// sprintDetailStoryIDs devuelve los identificadores de las historias del detalle, en su orden.
func sprintDetailStoryIDs(body sprintStoriesDetailBody) []string {
	ids := make([]string, 0, len(body.Stories))
	for _, s := range body.Stories {
		ids = append(ids, s.StoryID)
	}
	return ids
}

// requireNotAdmitted exige un 409 con StoriesNotAdmittedResponse y su mensaje general, y lo devuelve
// decodificado.
func requireNotAdmitted(t *testing.T, rec *httptest.ResponseRecorder) sprintNotAdmittedBody {
	t.Helper()
	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintNotAdmittedBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, sprintNotAdmittedMessage, body.Error)
	require.NotEmpty(t, body.Stories, "el 409 debe listar las historias rechazadas: %s", rec.Body.String())
	return body
}

// sprintCommitmentCount cuenta las filas de sprint_stories que cumplen la condición, para verificar
// qué compromisos quedaron en la base.
func sprintCommitmentCount(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.SprintStory{}).Where(query, args...).Count(&n).Error,
		"no se pudieron contar los compromisos")
	return n
}

// sprintStoredStory lee de user_stories la historia indicada.
func sprintStoredStory(t *testing.T, db *gorm.DB, storyID uuid.UUID) backlogdomain.UserStory {
	t.Helper()
	var story backlogdomain.UserStory
	require.NoError(t, db.First(&story, "id = ?", storyID).Error, "no se encontró la historia %s", storyID)
	return story
}

// sprintCriteriaCount cuenta los criterios de aceptación guardados de la historia.
func sprintCriteriaCount(t *testing.T, db *gorm.DB, storyID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&backlogdomain.AcceptanceCriterion{}).Where("story_id = ?", storyID).Count(&n).Error,
		"no se pudieron contar los criterios")
	return n
}

// sprintRandomIDs devuelve n identificadores UUID nuevos y distintos.
func sprintRandomIDs(n int) []uuid.UUID {
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, uuid.New())
	}
	return ids
}

// sprintRequireBatchRejected exige un 400 con ValidationErrorResponse con al menos un campo cuyo
// nombre empiece por el prefijo indicado (storyIds, storyIds[i] o storyId) y tenga mensaje.
func sprintRequireBatchRejected(t *testing.T, rec *httptest.ResponseRecorder, fieldPrefix string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintValidationBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 400 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, sprintInvalidFieldsError, body.Error)
	for _, f := range body.Fields {
		if strings.HasPrefix(f.Field, fieldPrefix) {
			assert.NotEmpty(t, f.Message, "el mensaje del campo %q no puede estar vacío", f.Field)
			return
		}
	}
	t.Errorf("la respuesta no señala un campo %q: %s", fieldPrefix, rec.Body.String())
}

// sprintUS2Fixture es el "Dado" común de US2: Ana, propietaria de un proyecto En curso, y Bruno,
// integrante que no es el propietario y que ejecuta las acciones (RN-01), con un sprint Planificado
// "Sprint 1" del 2026-10-01 al 2026-10-14.
type sprintUS2Fixture struct {
	env         sprintTestEnv
	anaID       uuid.UUID
	brunoID     uuid.UUID
	brunoToken  string
	projectID   uuid.UUID
	sprintOneID uuid.UUID
}

// newSprintUS2Fixture compone el entorno y siembra las cuentas, el proyecto y el sprint Planificado.
func newSprintUS2Fixture(t *testing.T) sprintUS2Fixture {
	t.Helper()
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	sprintOneID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	return sprintUS2Fixture{env: env, anaID: anaID, brunoID: brunoID, brunoToken: brunoToken, projectID: projectID, sprintOneID: sprintOneID}
}

// Escenario: Asignación de tres historias (004/US2)
// Un integrante asigna a un sprint Planificado tres historias listas: responde 200 con las tres en
// stories, en pending y sin resultado; quedan tres compromisos abiertos en sprint_stories, y una
// cuarta asignación de una de ellas a otro sprint abierto se rechaza porque ya está comprometida
// (FR-017, FR-020, FR-021).
func TestUS2_AsignacionDeTresHistorias(t *testing.T) {
	f := newSprintUS2Fixture(t)
	s1 := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	s2 := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Filtrar por fecha")
	s3 := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Enviar por correo")

	rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, s1, s2, s3)

	// Las tres quedan en el detalle, en pending y sin resultado.
	body := requireSprintStories(t, rec)
	assert.Equal(t, f.sprintOneID.String(), body.ID)
	assert.Equal(t, "planned", body.Status)
	assert.ElementsMatch(t, []string{s1.String(), s2.String(), s3.String()}, sprintDetailStoryIDs(body))
	for _, s := range body.Stories {
		assert.Equal(t, "pending", s.Status, "la historia %s debe seguir pending", s.StoryID)
		assert.Nil(t, s.Outcome, "la historia %s no tiene resultado en un sprint abierto", s.StoryID)
		assert.Nil(t, s.CompletedAt)
	}

	// En la base hay un compromiso abierto por historia y las tres siguen en pending.
	assert.EqualValues(t, 3, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND outcome IS NULL", f.sprintOneID))
	for _, id := range []uuid.UUID{s1, s2, s3} {
		assert.Equal(t, backlogdomain.StoryStatusPending, sprintStoredStory(t, f.env.db, id).Status)
	}

	// Ya no están disponibles para otro sprint abierto.
	sprintTwoID := sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28")
	rejected := requireNotAdmitted(t, f.env.assignStories(t, f.brunoToken, sprintTwoID, s1))
	require.Len(t, rejected.Stories, 1)
	assert.Equal(t, s1.String(), rejected.Stories[0].StoryID)
	assert.Equal(t, "committed_elsewhere", rejected.Stories[0].Reason)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", sprintTwoID))
}

// Escenario: Asignación en sprint Activo (004/US2)
// A un sprint Activo se le suma una historia lista y la asignación se acepta, porque el alcance puede
// ajustarse mientras el sprint corre; el sprint sigue active (FR-017).
func TestUS2_AsignacionEnSprintActivo(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")

	rec := f.env.assignStories(t, f.brunoToken, activeID, storyID)

	body := requireSprintStories(t, rec)
	assert.Equal(t, "active", body.Status)
	assert.Equal(t, []string{storyID.String()}, sprintDetailStoryIDs(body))
	assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND story_id = ? AND outcome IS NULL", activeID, storyID))
	assert.Equal(t, domain.SprintStatusActive, sprintStored(t, f.env.db, activeID.String()).Status)
}

// Escenario: Quitar de un sprint Planificado (004/US2)
// Quitar una historia comprometida en un sprint Planificado la devuelve al backlog en pending y sin
// sprint (sin fila en sprint_stories), y después puede asignarse a otro sprint (FR-021, FR-022).
func TestUS2_QuitarDeSprintPlanificado(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)

	rec := f.env.removeStories(t, f.brunoToken, f.sprintOneID, storyID)

	// El sprint queda sin la historia y ella, en pending y sin compromiso.
	body := requireSprintStories(t, rec)
	assert.Empty(t, body.Stories)
	assert.Equal(t, backlogdomain.StoryStatusPending, sprintStoredStory(t, f.env.db, storyID).Status)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", storyID), "no debe quedar fila en sprint_stories")

	// La historia vuelve a estar disponible para otro sprint.
	sprintTwoID := sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28")
	reassigned := requireSprintStories(t, f.env.assignStories(t, f.brunoToken, sprintTwoID, storyID))
	assert.Equal(t, []string{storyID.String()}, sprintDetailStoryIDs(reassigned))
}

// Escenario: Reestimación de una comprometida (004/US2)
// Una historia comprometida en un sprint abierto y estimada en 5 se reestima en 8 con el PUT
// story-points de la 003: responde 200 porque sigue lista para planificar, y el detalle del sprint la
// muestra todavía comprometida, con 8 (FR-018, RN-22).
func TestUS2_ReestimacionDeComprometida(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)

	rec := f.env.sprintDo(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", f.brunoToken, map[string]any{"storyPoints": 8})

	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, sprintPoints(8), sprintStoredStory(t, f.env.db, storyID).StoryPoints)

	// La historia sigue comprometida en el sprint, ahora con 8 Story Points.
	detail := requireSprintStories(t, f.env.sprintDo(t, http.MethodGet, sprintPath(f.sprintOneID), f.brunoToken, nil))
	require.Equal(t, []string{storyID.String()}, sprintDetailStoryIDs(detail))
	assert.Equal(t, 8, detail.Stories[0].StoryPoints)
}

// Escenario: Quitar la única historia de un sprint Activo (004/US2)
// Quitar la única historia de un sprint Activo lo deja sin historias y sigue active, sin que sea un
// error; la historia, que estaba en progreso, vuelve a pending (FR-022).
func TestUS2_QuitarUnicaHistoriaDeSprintActivo(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSeedCommitment(t, f.env.db, activeID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)
	sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusInProgress)

	rec := f.env.removeStories(t, f.brunoToken, activeID, storyID)

	body := requireSprintStories(t, rec)
	assert.Equal(t, "active", body.Status)
	assert.Equal(t, []any{}, sprintRawBody(t, rec)["stories"], "stories debe ser [] y no null")
	assert.Equal(t, domain.SprintStatusActive, sprintStored(t, f.env.db, activeID.String()).Status)
	assert.Equal(t, backlogdomain.StoryStatusPending, sprintStoredStory(t, f.env.db, storyID).Status)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", activeID))
}

// Escenario: Tercer sprint (004/US2)
// Una historia que estuvo en dos sprints cerrados sin completarse se asigna a un tercero y se acepta;
// en la base quedan tres filas de sprint_stories para ella: las dos congeladas con not_completed y la
// abierta, sin resultado. El endpoint de historial se verifica en TestUS6_HistorialDeUnaHistoria
// (FR-020, FR-055).
func TestUS2_TercerSprint(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := env.createReadyStory(t, anaToken, projectID, "Exportar el reporte")
	notCompleted := domain.StoryOutcomeNotCompleted
	closedOne := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 1", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
	sprintSeedCommitment(t, env.db, closedOne, storyID, sprintDate(t, "2026-09-01"), &notCompleted, nil)
	closedTwo := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 2", "2026-09-15", "2026-09-28", domain.SprintStatusClosed)
	sprintSeedCommitment(t, env.db, closedTwo, storyID, sprintDate(t, "2026-09-15"), &notCompleted, nil)
	thirdID := sprintSeed(t, env.db, projectID, anaID, "Sprint 3", "2026-10-01", "2026-10-14")

	rec := env.assignStories(t, anaToken, thirdID, storyID)

	body := requireSprintStories(t, rec)
	assert.Equal(t, []string{storyID.String()}, sprintDetailStoryIDs(body))
	assert.EqualValues(t, 3, sprintCommitmentCount(t, env.db, "story_id = ?", storyID))
	assert.EqualValues(t, 2, sprintCommitmentCount(t, env.db, "story_id = ? AND outcome = ?", storyID, domain.StoryOutcomeNotCompleted))
	assert.EqualValues(t, 1, sprintCommitmentCount(t, env.db, "story_id = ? AND outcome IS NULL AND sprint_id = ?", storyID, thirdID))
}

// Escenario: Historia no lista (004/US2)
// Una historia sin Story Points o sin criterios de aceptación no se asigna: 409 con el motivo
// not_ready, lo que le falta y el mensaje de que no está lista para planificar, y no se crea ningún
// compromiso (FR-018, RN-08).
func TestUS2_HistoriaNoLista(t *testing.T) {
	cases := []struct {
		name     string
		points   *int
		criteria []string
		missing  string
	}{
		{"sin Story Points", nil, []string{sprintCriterion}, "story_points"},
		{"sin criterios", sprintPoints(5), []string{}, "acceptance_criteria"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSprintUS2Fixture(t)
			storyID := f.env.createStory(t, f.brunoToken, f.projectID, "Exportar el reporte", tc.points, tc.criteria...)

			rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, storyID)

			body := requireNotAdmitted(t, rec)
			require.Len(t, body.Stories, 1)
			assert.Equal(t, storyID.String(), body.Stories[0].StoryID)
			assert.Equal(t, "not_ready", body.Stories[0].Reason)
			assert.Equal(t, []string{tc.missing}, body.Stories[0].Missing)
			assert.Contains(t, strings.ToLower(body.Stories[0].Message), "lista para planificar")
			assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", storyID), "no debería haberse creado ningún compromiso")
		})
	}
}

// Escenario: Historia Completada (004/US2)
// Una historia en estado Completada no se asigna: 409 con el motivo completed y el mensaje de que ya
// está terminada, y no se crea ningún compromiso (FR-019).
func TestUS2_HistoriaCompletada(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusCompleted)

	rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, storyID)

	body := requireNotAdmitted(t, rec)
	require.Len(t, body.Stories, 1)
	assert.Equal(t, storyID.String(), body.Stories[0].StoryID)
	assert.Equal(t, "completed", body.Stories[0].Reason)
	assert.Equal(t, "La historia ya está terminada.", body.Stories[0].Message)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", storyID), "no debería haberse creado ningún compromiso")
}

// Escenario: Comprometida en otro sprint (004/US2)
// Una historia ya comprometida en un sprint Planificado no se asigna a otro sprint abierto: 409 con el
// motivo committed_elsewhere y el sprint en el que está, y el compromiso original no cambia (FR-020).
func TestUS2_ComprometidaEnOtroSprint(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)
	sprintTwoID := sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28")

	rec := f.env.assignStories(t, f.brunoToken, sprintTwoID, storyID)

	body := requireNotAdmitted(t, rec)
	require.Len(t, body.Stories, 1)
	problem := body.Stories[0]
	assert.Equal(t, storyID.String(), problem.StoryID)
	assert.Equal(t, "committed_elsewhere", problem.Reason)
	if assert.NotNil(t, problem.Sprint, "falta el sprint del compromiso: %s", rec.Body.String()) {
		assert.Equal(t, f.sprintOneID.String(), problem.Sprint.ID)
		assert.Equal(t, "Sprint 1", problem.Sprint.Name)
	}
	assert.Equal(t, "Ya está comprometida en el sprint «Sprint 1».", problem.Message)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", sprintTwoID))
	assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND story_id = ?", f.sprintOneID, storyID))
}

// Escenario: Sprint Cerrado (004/US2)
// Sobre un sprint Cerrado se rechaza con 409 tanto asignar como quitar una historia, porque un sprint
// cerrado no admite cambios, y sus compromisos no cambian (FR-024, RN-18).
func TestUS2_SprintCerrado(t *testing.T) {
	t.Run("asignar", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")

		rec := f.env.assignStories(t, f.brunoToken, closedID, storyID)

		assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.JSONEq(t, `{"error":"`+sprintClosedMessage+`"}`, rec.Body.String())
		assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", closedID))
	})

	t.Run("quitar", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
		notCompleted := domain.StoryOutcomeNotCompleted
		sprintSeedCommitment(t, f.env.db, closedID, storyID, sprintDate(t, "2026-09-01"), &notCompleted, nil)

		rec := f.env.removeStories(t, f.brunoToken, closedID, storyID)

		assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.JSONEq(t, `{"error":"`+sprintClosedMessage+`"}`, rec.Body.String())
		assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND story_id = ?", closedID, storyID),
			"la fila congelada debe seguir en la instantánea")
	})
}

// Escenario: Lote parcialmente inválido (004/US2)
// Un lote de tres historias donde una no tiene criterios se rechaza entero: 409 que lista solo la
// historia con problemas, ninguna de las tres queda asignada y ninguna se marca ever_committed
// (FR-023, SC-007).
func TestUS2_LoteParcialmenteInvalido(t *testing.T) {
	f := newSprintUS2Fixture(t)
	s1 := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	s2 := f.env.createStory(t, f.brunoToken, f.projectID, "Filtrar por fecha", sprintPoints(3))
	s3 := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Enviar por correo")

	rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, s1, s2, s3)

	body := requireNotAdmitted(t, rec)
	require.Len(t, body.Stories, 1, "solo la historia sin criterios tiene problemas: %s", rec.Body.String())
	assert.Equal(t, s2.String(), body.Stories[0].StoryID)
	assert.Equal(t, "not_ready", body.Stories[0].Reason)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", f.sprintOneID), "no debería haberse creado ninguna fila")
	for _, id := range []uuid.UUID{s1, s2, s3} {
		assert.False(t, sprintStoredStory(t, f.env.db, id).EverCommitted, "la historia %s no debe marcarse ever_committed", id)
	}
}

// Escenario: Historia de otro proyecto (004/US2)
// Asignar a un sprint una historia de otro proyecto, aunque quien pide sea integrante de ambos,
// responde 404 de historia inexistente y no crea ningún compromiso.
func TestUS2_HistoriaDeOtroProyecto(t *testing.T) {
	f := newSprintUS2Fixture(t)
	otherProjectID := sprintSeedProject(t, f.env.db, f.anaID, projectsdomain.ProjectStatusInProgress, f.brunoID)
	foreignStoryID := f.env.createReadyStory(t, f.brunoToken, otherProjectID, "Historia ajena")

	rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, foreignStoryID)

	assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintStoryNotFound+`"}`, rec.Body.String())
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", foreignStoryID))
}

// Escenario: Quitar una historia Completada (004/US2)
// Quitar del sprint Activo una historia Completada se rechaza con 409 indicando que primero debe
// devolverse a En progreso; la historia conserva completed y su compromiso conserva completed_at
// (FR-025, RN-21).
func TestUS2_QuitarHistoriaCompletada(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	completedAt := time.Date(2026, time.October, 20, 14, 30, 0, 0, time.UTC)
	sprintSeedCommitment(t, f.env.db, activeID, storyID, sprintDate(t, "2026-10-15"), nil, &completedAt)
	sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusCompleted)

	rec := f.env.removeStories(t, f.brunoToken, activeID, storyID)

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintCompletedRemoval+`"}`, rec.Body.String())
	assert.Equal(t, backlogdomain.StoryStatusCompleted, sprintStoredStory(t, f.env.db, storyID).Status)

	// El compromiso sigue en sprint_stories con el mismo completed_at.
	var row domain.SprintStory
	require.NoError(t, f.env.db.First(&row, "sprint_id = ? AND story_id = ?", activeID, storyID).Error, "el compromiso debe seguir en sprint_stories")
	if assert.NotNil(t, row.CompletedAt) {
		assert.True(t, completedAt.Equal(*row.CompletedAt), "completed_at = %v, se esperaba %v", *row.CompletedAt, completedAt)
	}
}

// Escenario: Comprometida no deja de estar lista (004/US2)
// A una historia comprometida en un sprint Planificado no se le puede volver a "sin estimar" con el
// PUT story-points de la 003 ni quitar su último criterio con el PATCH de la 003: 409 indicando que
// primero hay que quitarla del sprint, y la historia queda como estaba (FR-018, RN-22, RC-05).
func TestUS2_ComprometidaNoDejaDeEstarLista(t *testing.T) {
	t.Run("PUT story-points con null", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
		sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)

		rec := f.env.sprintDo(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", f.brunoToken, map[string]any{"storyPoints": nil})

		assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.JSONEq(t, `{"error":"`+sprintBacklogCommitted+`"}`, rec.Body.String())
		assert.Equal(t, sprintPoints(5), sprintStoredStory(t, f.env.db, storyID).StoryPoints, "la estimación no debe cambiar")
	})

	t.Run("PATCH con acceptanceCriteria vacío", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
		sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)

		rec := f.env.sprintDo(t, http.MethodPatch, "/api/v1/stories/"+storyID.String(), f.brunoToken, map[string]any{"acceptanceCriteria": []string{}})

		assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.JSONEq(t, `{"error":"`+sprintBacklogCommitted+`"}`, rec.Body.String())
		assert.EqualValues(t, 1, sprintCriteriaCount(t, f.env.db, storyID), "los criterios no deben cambiar")
	})
}

// Lotes inválidos de asignación y de baja, sin etiqueta de escenario porque son cotas de entrada y
// no escenarios de la spec (research.md sección 8, Principio VII): un identificador repetido o más
// de 100 identificadores responden 400 con los campos inválidos antes de llegar al servicio, y no
// crean ni borran compromisos.
func TestSprintStories_InvalidBatch(t *testing.T) {
	t.Run("asignar con un identificador repetido", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")

		rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, storyID, storyID)

		sprintRequireBatchRejected(t, rec, "storyIds")
		assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", f.sprintOneID))
	})

	t.Run("asignar más de 100", func(t *testing.T) {
		f := newSprintUS2Fixture(t)

		rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, sprintRandomIDs(101)...)

		sprintRequireBatchRejected(t, rec, "storyIds")
		assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", f.sprintOneID))
	})

	t.Run("quitar con un identificador repetido", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
		sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)

		rec := f.env.removeStories(t, f.brunoToken, f.sprintOneID, storyID, storyID)

		sprintRequireBatchRejected(t, rec, "storyId")
		assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", f.sprintOneID))
	})

	t.Run("quitar más de 100", func(t *testing.T) {
		f := newSprintUS2Fixture(t)

		rec := f.env.removeStories(t, f.brunoToken, f.sprintOneID, sprintRandomIDs(101)...)

		sprintRequireBatchRejected(t, rec, "storyId")
	})
}

// RC-06, sin etiqueta de escenario: la asignación marca user_stories.ever_committed en true y quitar
// la historia del sprint no lo devuelve a false, para que la 003 siga impidiendo eliminarla.
func TestSprintStories_EverCommitted(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	require.False(t, sprintStoredStory(t, f.env.db, storyID).EverCommitted, "la historia nueva no estuvo en ningún sprint")

	assigned := f.env.assignStories(t, f.brunoToken, f.sprintOneID, storyID)
	require.Equal(t, http.StatusOK, assigned.Code, "cuerpo: %s", assigned.Body.String())
	assert.True(t, sprintStoredStory(t, f.env.db, storyID).EverCommitted, "tras asignar, ever_committed debe ser true")

	removed := f.env.removeStories(t, f.brunoToken, f.sprintOneID, storyID)
	require.Equal(t, http.StatusOK, removed.Code, "cuerpo: %s", removed.Body.String())
	assert.True(t, sprintStoredStory(t, f.env.db, storyID).EverCommitted, "tras quitar, ever_committed debe seguir en true")
}

// Textos exactos que fija el contrato para los rechazos del inicio de un sprint (US3): el 409 de
// otro sprint activo, que nombra al activo, y el de la transición inválida de un sprint ya activo.
const (
	sprintStartTransitionMessage = "Transición no permitida: solo se inicia un sprint Planificado."
	sprintActiveExistsTemplate   = "El proyecto ya tiene el sprint activo «%s». Cerralo antes de iniciar otro."
)

// sprintStartedAt es el momento de inicio que debe devolver la API: el del reloj inyectado, en UTC
// con formato RFC 3339.
var sprintStartedAt = sprintFixedNow.UTC().Format(time.RFC3339)

// sprintStartPath es la ruta de inicio de un sprint según contracts/openapi.yaml.
func sprintStartPath(sprintID uuid.UUID) string {
	return sprintPath(sprintID) + "/start"
}

// startSprint envía POST /api/v1/sprints/{sprintId}/start, sin cuerpo, con el token como Bearer.
func (e sprintTestEnv) startSprint(t *testing.T, token string, sprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodPost, sprintStartPath(sprintID), token, nil)
}

// requireStartedSprint exige un 200 cuyo cuerpo sea el detalle de un sprint active con startedAt
// igual al reloj, y lo devuelve decodificado junto con el cuerpo crudo.
func requireStartedSprint(t *testing.T, rec *httptest.ResponseRecorder, sprintID uuid.UUID) (sprintStoriesDetailBody, map[string]any) {
	t.Helper()
	body := requireSprintStories(t, rec)
	raw := sprintRawBody(t, rec)
	assert.Equal(t, sprintID.String(), body.ID)
	assert.Equal(t, "active", body.Status)
	assert.Equal(t, sprintStartedAt, raw["startedAt"], "startedAt debe ser el instante del reloj")
	return body, raw
}

// requireStoredStarted exige que la fila del sprint quede active, con started_at igual al reloj y
// committed_story_points igual al valor indicado.
func requireStoredStarted(t *testing.T, db *gorm.DB, sprintID uuid.UUID, committed int) {
	t.Helper()
	stored := sprintStored(t, db, sprintID.String())
	assert.Equal(t, domain.SprintStatusActive, stored.Status)
	if assert.NotNil(t, stored.StartedAt, "started_at debe quedar registrado") {
		assert.True(t, sprintFixedNow.Equal(*stored.StartedAt), "started_at = %v, se esperaba %v", *stored.StartedAt, sprintFixedNow)
	}
	if assert.NotNil(t, stored.CommittedStoryPoints, "committed_story_points debe quedar congelado") {
		assert.Equal(t, committed, *stored.CommittedStoryPoints)
	}
}

// requireStoredUnchanged exige que la fila del sprint conserve el estado, el momento de inicio y los
// Story Points comprometidos que tenía antes del rechazo.
func requireStoredUnchanged(t *testing.T, db *gorm.DB, before domain.Sprint) {
	t.Helper()
	after := sprintStored(t, db, before.ID.String())
	assert.Equal(t, before.Status, after.Status, "el estado del sprint no debe cambiar")
	assert.Equal(t, before.CommittedStoryPoints, after.CommittedStoryPoints, "committed_story_points no debe cambiar")
	if before.StartedAt == nil {
		assert.Nil(t, after.StartedAt, "started_at debe seguir en null")
		return
	}
	if assert.NotNil(t, after.StartedAt, "started_at no debe borrarse") {
		assert.True(t, before.StartedAt.Equal(*after.StartedAt), "started_at = %v, se esperaba %v", *after.StartedAt, *before.StartedAt)
	}
}

// sprintCommittedSum suma en la base los Story Points vigentes de las historias comprometidas en el
// sprint, para comparar el valor congelado al iniciar con la fuente de verdad.
func sprintCommittedSum(t *testing.T, db *gorm.DB, sprintID uuid.UUID) int {
	t.Helper()
	var sum int
	require.NoError(t, db.Table("sprint_stories").
		Select("COALESCE(SUM(user_stories.story_points), 0)").
		Joins("JOIN user_stories ON user_stories.id = sprint_stories.story_id").
		Where("sprint_stories.sprint_id = ?", sprintID).
		Scan(&sum).Error, "no se pudieron sumar los Story Points comprometidos")
	return sum
}

// sprintStoredProject lee de projects la fila del proyecto, para verificar que iniciar un sprint no
// la escribe (RN-07).
func sprintStoredProject(t *testing.T, db *gorm.DB, projectID uuid.UUID) projectsdomain.Project {
	t.Helper()
	var project projectsdomain.Project
	require.NoError(t, db.First(&project, "id = ?", projectID).Error, "no se encontró el proyecto %s", projectID)
	return project
}

// requireProjectUntouched exige que el proyecto siga en el estado esperado según la API de la 002 y
// que su fila no se haya escrito: mismo estado y mismo updated_at que antes del inicio (RN-07).
func (e sprintTestEnv) requireProjectUntouched(t *testing.T, token string, before projectsdomain.Project, want projectsdomain.ProjectStatus) {
	t.Helper()
	rec := e.sprintDo(t, http.MethodGet, "/api/v1/projects/"+before.ID.String(), token, nil)
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var project struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &project))
	assert.Equal(t, string(want), project.Status, "el proyecto debe seguir %s", want)

	after := sprintStoredProject(t, e.db, before.ID)
	assert.Equal(t, want, after.Status)
	assert.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "la fila del proyecto no debe escribirse: updated_at = %v, antes %v", after.UpdatedAt, before.UpdatedAt)
}

// Escenario: Inicio con historias comprometidas (004/US3)
// Bruno, integrante que no es el propietario de un proyecto En curso, inicia un sprint Planificado
// con tres historias de 3, 5 y 8 Story Points: responde 200 con el sprint active, startedAt igual al
// reloj y las tres historias; en la base el sprint queda active con started_at igual al reloj y
// committed_story_points igual a la suma de sus historias, 16 (FR-026, FR-028).
func TestUS3_InicioConHistorias(t *testing.T) {
	f := newSprintUS2Fixture(t)
	s1 := f.env.createStory(t, f.brunoToken, f.projectID, "Exportar el reporte", sprintPoints(3), sprintCriterion)
	s2 := f.env.createStory(t, f.brunoToken, f.projectID, "Filtrar por fecha", sprintPoints(5), sprintCriterion)
	s3 := f.env.createStory(t, f.brunoToken, f.projectID, "Enviar por correo", sprintPoints(8), sprintCriterion)
	for _, id := range []uuid.UUID{s1, s2, s3} {
		sprintSeedCommitment(t, f.env.db, f.sprintOneID, id, sprintFixedNow.Add(-time.Hour), nil, nil)
	}

	rec := f.env.startSprint(t, f.brunoToken, f.sprintOneID)

	// La respuesta es el detalle del sprint activo con sus tres historias.
	body, _ := requireStartedSprint(t, rec, f.sprintOneID)
	assert.ElementsMatch(t, []string{s1.String(), s2.String(), s3.String()}, sprintDetailStoryIDs(body))

	// La fila guarda el inicio y congela la suma de los Story Points comprometidos.
	sum := sprintCommittedSum(t, f.env.db, f.sprintOneID)
	require.Equal(t, 16, sum, "la suma en la base debe ser 3 + 5 + 8")
	requireStoredStarted(t, f.env.db, f.sprintOneID, sum)
}

// Escenario: Proyecto Planificado no cambia (004/US3)
// En un proyecto en estado Planificado, Bruno, integrante que no es el propietario, inicia el sprint
// Planificado: el sprint queda active y el proyecto sigue planned, sin que se escriba su fila,
// porque los sprints no cambian el estado del proyecto (FR-029, RN-07).
func TestUS3_ProyectoPlanificadoNoCambia(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusPlanned, brunoID)
	sprintID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	before := sprintStoredProject(t, env.db, projectID)

	rec := env.startSprint(t, brunoToken, sprintID)

	requireStartedSprint(t, rec, sprintID)
	requireStoredStarted(t, env.db, sprintID, 0)
	env.requireProjectUntouched(t, brunoToken, before, projectsdomain.ProjectStatusPlanned)
}

// Escenario: Proyecto En curso no cambia (004/US3)
// En un proyecto que ya está En curso, iniciar un sprint lo deja active y el proyecto sigue
// in_progress, sin que se escriba su fila (FR-029, RN-07).
func TestUS3_ProyectoEnCursoNoCambia(t *testing.T) {
	f := newSprintUS2Fixture(t)
	before := sprintStoredProject(t, f.env.db, f.projectID)

	rec := f.env.startSprint(t, f.brunoToken, f.sprintOneID)

	requireStartedSprint(t, rec, f.sprintOneID)
	requireStoredStarted(t, f.env.db, f.sprintOneID, 0)
	f.env.requireProjectUntouched(t, f.brunoToken, before, projectsdomain.ProjectStatusInProgress)
}

// Escenario: Sprint vacío (004/US3)
// Un sprint Planificado sin historias se inicia y queda active y vacío: stories es [] y no null, y
// committed_story_points queda en 0 y no en null (FR-030).
func TestUS3_SprintVacio(t *testing.T) {
	f := newSprintUS2Fixture(t)

	rec := f.env.startSprint(t, f.brunoToken, f.sprintOneID)

	_, raw := requireStartedSprint(t, rec, f.sprintOneID)
	assert.Equal(t, []any{}, raw["stories"], "stories debe ser [] y no null")
	requireStoredStarted(t, f.env.db, f.sprintOneID, 0)
}

// Escenario: Inicio antes de la fecha (004/US3)
// Un sprint cuya fecha de inicio prevista (2026-11-02) es posterior al reloj (2026-10-21) se inicia
// igual: queda active con started_at igual al reloj y sus fechas previstas no cambian, porque el
// arranque lo decide el equipo y no el calendario (FR-031).
func TestUS3_InicioAntesDeLaFecha(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	sprintID := sprintSeed(t, env.db, projectID, anaID, "Sprint 3", "2026-11-02", "2026-11-15")
	require.True(t, sprintDate(t, "2026-11-02").After(sprintFixedNow), "la fecha prevista debe ser futura respecto del reloj")

	rec := env.startSprint(t, anaToken, sprintID)

	_, raw := requireStartedSprint(t, rec, sprintID)
	assert.Equal(t, "2026-11-02", raw["startDate"], "la fecha de inicio prevista no cambia")
	assert.Equal(t, "2026-11-15", raw["endDate"], "la fecha de fin prevista no cambia")
	requireStoredStarted(t, env.db, sprintID, 0)
	stored := sprintStored(t, env.db, sprintID.String())
	assert.Equal(t, sprintDate(t, "2026-11-02"), stored.StartDate.UTC())
	assert.Equal(t, sprintDate(t, "2026-11-15"), stored.EndDate.UTC())
}

// Escenario: Otro sprint activo (004/US3)
// En un proyecto con "Sprint 0" Activo, iniciar el Planificado "Sprint 1" se rechaza con 409
// SprintConflictResponse que nombra a "Sprint 0" y pide cerrarlo primero; "Sprint 1" sigue
// Planificado sin datos de inicio y "Sprint 0" sigue Activo con los suyos (FR-027, RN-06).
func TestUS3_OtroSprintActivo(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-15", "2026-09-28", domain.SprintStatusActive)
	activeBefore := sprintStored(t, f.env.db, activeID.String())
	plannedBefore := sprintStored(t, f.env.db, f.sprintOneID.String())

	rec := f.env.startSprint(t, f.brunoToken, f.sprintOneID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintConflictBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, fmt.Sprintf(sprintActiveExistsTemplate, "Sprint 0"), body.Error)
	if assert.NotNil(t, body.Sprint, "el 409 debe indicar el sprint activo: %s", rec.Body.String()) {
		assert.Equal(t, activeID.String(), body.Sprint.ID)
		assert.Equal(t, "Sprint 0", body.Sprint.Name)
	}
	requireStoredUnchanged(t, f.env.db, plannedBefore)
	requireStoredUnchanged(t, f.env.db, activeBefore)
}

// Escenario: Sprint ya activo (004/US3)
// Iniciar otra vez un sprint Activo se rechaza con 409 por transición inválida, sin el campo sprint,
// y nada cambia: conserva su started_at y sus committed_story_points (FR-032, RN-02).
func TestUS3_SprintYaActivo(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	before := sprintStored(t, f.env.db, activeID.String())

	rec := f.env.startSprint(t, f.brunoToken, activeID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintStartTransitionMessage+`"}`, rec.Body.String())
	requireStoredUnchanged(t, f.env.db, before)
}

// Escenario: Sprint cerrado (004/US3)
// Iniciar un sprint Cerrado se rechaza con 409 y un mensaje de error, sin el campo sprint, porque un
// sprint cerrado no vuelve atrás; sigue closed con sus datos de inicio y de cierre (FR-032, RN-18).
func TestUS3_SprintCerrado(t *testing.T) {
	f := newSprintUS2Fixture(t)
	closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
	before := sprintStored(t, f.env.db, closedID.String())

	rec := f.env.startSprint(t, f.brunoToken, closedID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	raw := sprintRawBody(t, rec)
	assert.NotEmpty(t, raw["error"], "el 409 debe explicar el rechazo: %s", rec.Body.String())
	assert.NotContains(t, raw, "sprint", "el rechazo de un sprint cerrado no nombra otro sprint")
	requireStoredUnchanged(t, f.env.db, before)
	after := sprintStored(t, f.env.db, closedID.String())
	assert.Equal(t, before.ClosedByID, after.ClosedByID, "closed_by_id no debe cambiar")
	if assert.NotNil(t, after.ClosedAt, "closed_at no debe borrarse") {
		assert.True(t, before.ClosedAt.Equal(*after.ClosedAt), "closed_at = %v, se esperaba %v", *after.ClosedAt, *before.ClosedAt)
	}
}

// Escenario: Proyecto finalizado (004/US3)
// En un proyecto Finalizado, iniciar un sprint Planificado se rechaza con 409 porque sus sprints son
// de solo lectura, y el sprint sigue planned sin datos de inicio (FR-059, RN-19).
func TestUS3_ProyectoFinalizado(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusFinished, brunoID)
	sprintID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	before := sprintStored(t, env.db, sprintID.String())

	rec := env.startSprint(t, brunoToken, sprintID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintReadOnlyMessage+`"}`, rec.Body.String())
	requireStoredUnchanged(t, env.db, before)
}

// Textos exactos que fija el contrato para los rechazos del avance de una historia (US4): el 409 de
// la historia fuera del sprint Activo (sin sprint o de un sprint Planificado) y el de la transición
// no admitida, que enumera las tres permitidas. El de sprint cerrado es sprintClosedMessage.
const (
	sprintOutsideActiveMessage   = "Solo se registra avance sobre historias del sprint activo."
	sprintProgressTransitionText = "Transición no permitida. Las admitidas son: Pendiente → En progreso, En progreso → Completada y Completada → En progreso."
)

// sprintProgressBody refleja StoryProgressResponse del contrato.
type sprintProgressBody struct {
	StoryID     string  `json:"storyId"`
	SprintID    string  `json:"sprintId"`
	Status      string  `json:"status"`
	CompletedAt *string `json:"completedAt"`
}

// sprintProgressSnapshot guarda el estado de una historia y el completed_at de cada uno de sus
// compromisos, para verificar que un rechazo no los cambió.
type sprintProgressSnapshot struct {
	storyID     uuid.UUID
	status      backlogdomain.StoryStatus
	completedAt map[uuid.UUID]*time.Time
}

// sprintStoryStatusPath es la ruta del registro del avance de una historia según
// contracts/openapi.yaml.
func sprintStoryStatusPath(storyID uuid.UUID) string {
	return "/api/v1/stories/" + storyID.String() + "/status"
}

// changeStoryStatus envía PUT /api/v1/stories/{storyId}/status con el estado destino.
func (e sprintTestEnv) changeStoryStatus(t *testing.T, token string, storyID uuid.UUID, status string) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodPut, sprintStoryStatusPath(storyID), token, map[string]any{"status": status})
}

// requireStoryProgress exige un 200 con StoryProgressResponse de la historia en el sprint y el estado
// indicados, con el campo completedAt presente, y devuelve el cuerpo decodificado.
func requireStoryProgress(t *testing.T, rec *httptest.ResponseRecorder, storyID, sprintID uuid.UUID, status string) sprintProgressBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintProgressBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, storyID.String(), body.StoryID)
	assert.Equal(t, sprintID.String(), body.SprintID)
	assert.Equal(t, status, body.Status)
	_, present := sprintRawBody(t, rec)["completedAt"]
	assert.True(t, present, "falta el campo completedAt en la respuesta: %s", rec.Body.String())
	return body
}

// requireProgressCompletedAt exige que completedAt de la respuesta sea el instante indicado.
func requireProgressCompletedAt(t *testing.T, body sprintProgressBody, want time.Time) {
	t.Helper()
	require.NotNil(t, body.CompletedAt, "completedAt debe registrar el instante del completado")
	got, err := time.Parse(time.RFC3339, *body.CompletedAt)
	require.NoError(t, err, "completedAt = %q no es una fecha y hora RFC 3339", *body.CompletedAt)
	assert.True(t, want.Equal(got), "completedAt = %v, se esperaba %v", got, want)
}

// sprintStoredCommitment lee de sprint_stories el compromiso de la historia en el sprint.
func sprintStoredCommitment(t *testing.T, db *gorm.DB, sprintID, storyID uuid.UUID) domain.SprintStory {
	t.Helper()
	var row domain.SprintStory
	require.NoError(t, db.First(&row, "sprint_id = ? AND story_id = ?", sprintID, storyID).Error,
		"no se encontró el compromiso de la historia %s en el sprint %s", storyID, sprintID)
	return row
}

// requireStoredProgress exige que la historia quede en el estado indicado y que su compromiso en el
// sprint tenga completed_at igual al instante indicado, o null si es nil.
func requireStoredProgress(t *testing.T, db *gorm.DB, sprintID, storyID uuid.UUID, status backlogdomain.StoryStatus, completedAt *time.Time) {
	t.Helper()
	assert.Equal(t, status, sprintStoredStory(t, db, storyID).Status)
	row := sprintStoredCommitment(t, db, sprintID, storyID)
	if completedAt == nil {
		assert.Nil(t, row.CompletedAt, "completed_at debe quedar en null")
		return
	}
	if assert.NotNil(t, row.CompletedAt, "completed_at debe quedar registrado") {
		assert.True(t, completedAt.Equal(*row.CompletedAt), "completed_at = %v, se esperaba %v", *row.CompletedAt, *completedAt)
	}
}

// sprintTakeProgressSnapshot lee el estado de la historia y el completed_at de todos sus compromisos.
func sprintTakeProgressSnapshot(t *testing.T, db *gorm.DB, storyID uuid.UUID) sprintProgressSnapshot {
	t.Helper()
	var rows []domain.SprintStory
	require.NoError(t, db.Where("story_id = ?", storyID).Find(&rows).Error, "no se pudieron leer los compromisos")
	snapshot := sprintProgressSnapshot{
		storyID:     storyID,
		status:      sprintStoredStory(t, db, storyID).Status,
		completedAt: make(map[uuid.UUID]*time.Time, len(rows)),
	}
	for _, row := range rows {
		snapshot.completedAt[row.ID] = row.CompletedAt
	}
	return snapshot
}

// requireProgressRejected exige un 409 con el mensaje indicado y que la historia conserve su estado
// y cada compromiso su completed_at, comparados con la foto previa al rechazo.
func requireProgressRejected(t *testing.T, db *gorm.DB, rec *httptest.ResponseRecorder, message string, before sprintProgressSnapshot) {
	t.Helper()
	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+message+`"}`, rec.Body.String())

	after := sprintTakeProgressSnapshot(t, db, before.storyID)
	assert.Equal(t, before.status, after.status, "el estado de la historia no debe cambiar")
	require.Len(t, after.completedAt, len(before.completedAt), "los compromisos de la historia no deben cambiar")
	for id, want := range before.completedAt {
		got := after.completedAt[id]
		if want == nil {
			assert.Nil(t, got, "completed_at del compromiso %s debe seguir en null", id)
			continue
		}
		if assert.NotNil(t, got, "completed_at del compromiso %s no debe borrarse", id) {
			assert.True(t, want.Equal(*got), "completed_at = %v, se esperaba %v", *got, *want)
		}
	}
}

// sprintUS4Fixture es el "Dado" común de US4: el de US2 más "Sprint 2", Activo del 2026-10-15 al
// 2026-10-28, que contiene al reloj por defecto (2026-10-21).
type sprintUS4Fixture struct {
	sprintUS2Fixture
	activeID uuid.UUID
}

// newSprintUS4Fixture compone el entorno con los overrides recibidos y siembra las cuentas, el
// proyecto En curso, el sprint Planificado de US2 y el sprint Activo.
func newSprintUS4Fixture(t *testing.T, overrides ...func(*sprintTestPorts)) sprintUS4Fixture {
	t.Helper()
	env := newSprintTestEnv(t, overrides...)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	sprintOneID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	activeID := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	return sprintUS4Fixture{
		sprintUS2Fixture: sprintUS2Fixture{env: env, anaID: anaID, brunoID: brunoID, brunoToken: brunoToken, projectID: projectID, sprintOneID: sprintOneID},
		activeID:         activeID,
	}
}

// commitInActive crea con la 003 una historia lista, la compromete en el sprint Activo y le fija el
// estado indicado; si está completed, su compromiso registra completedAt.
func (f sprintUS4Fixture) commitInActive(t *testing.T, title string, status backlogdomain.StoryStatus, completedAt *time.Time) uuid.UUID {
	t.Helper()
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, title)
	sprintSeedCommitment(t, f.env.db, f.activeID, storyID, sprintDate(t, "2026-10-15"), nil, completedAt)
	if status != backlogdomain.StoryStatusPending {
		sprintSetStoryStatus(t, f.env.db, storyID, status)
	}
	return storyID
}

// Escenario: Avance hasta Completada (004/US4)
// Bruno, integrante que no es el propietario, pasa una historia Pendiente del sprint Activo a En
// progreso y luego a Completada: ambas transiciones responden 200 con la historia en el sprint, sin
// completedAt la primera y con completedAt igual al reloj la segunda, y en la base la historia queda
// completed con completed_at igual al reloj en sprint_stories (FR-033, FR-034).
func TestUS4_AvanceHastaCompletada(t *testing.T) {
	f := newSprintUS4Fixture(t)
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusPending, nil)

	// Primero pasa a En progreso, sin fecha de completado.
	started := requireStoryProgress(t, f.env.changeStoryStatus(t, f.brunoToken, storyID, "in_progress"), storyID, f.activeID, "in_progress")
	assert.Nil(t, started.CompletedAt, "completedAt debe ser null en una historia En progreso")
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusInProgress, nil)

	// Después se completa y queda registrado el instante del reloj.
	completed := requireStoryProgress(t, f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed"), storyID, f.activeID, "completed")
	requireProgressCompletedAt(t, completed, sprintFixedNow)
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusCompleted, &sprintFixedNow)
}

// Escenario: Desmarcar una Completada (004/US4)
// Una historia Completada del sprint Activo se devuelve a En progreso: responde 200 con completedAt
// null (presente y no ausente) y en la base completed_at se borra (FR-035, RN-12).
func TestUS4_DesmarcarCompletada(t *testing.T) {
	f := newSprintUS4Fixture(t)
	completedAt := time.Date(2026, time.October, 20, 14, 30, 0, 0, time.UTC)
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusCompleted, &completedAt)

	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "in_progress")

	body := requireStoryProgress(t, rec, storyID, f.activeID, "in_progress")
	assert.Nil(t, body.CompletedAt, "completedAt debe volver a null")
	assert.Nil(t, sprintRawBody(t, rec)["completedAt"], "completedAt debe ser null en el JSON")
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusInProgress, nil)
}

// Escenario: Recompletar registra el último (004/US4)
// Una historia se completa con el reloj en un primer instante, vuelve a En progreso y se completa de
// nuevo con el reloj en un segundo instante distinto: la respuesta y sprint_stories registran el
// segundo, el del último completado (FR-035).
func TestUS4_RecompletarRegistraElUltimo(t *testing.T) {
	first := time.Date(2026, time.October, 19, 11, 0, 0, 0, time.UTC)
	second := time.Date(2026, time.October, 21, 16, 45, 0, 0, time.UTC)
	current := first
	f := newSprintUS4Fixture(t, func(p *sprintTestPorts) {
		p.now = func() time.Time { return current }
	})
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusInProgress, nil)

	// Primer completado y vuelta a En progreso.
	firstBody := requireStoryProgress(t, f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed"), storyID, f.activeID, "completed")
	requireProgressCompletedAt(t, firstBody, first)
	requireStoryProgress(t, f.env.changeStoryStatus(t, f.brunoToken, storyID, "in_progress"), storyID, f.activeID, "in_progress")

	// Segundo completado, con el reloj en otro instante.
	current = second
	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed")

	body := requireStoryProgress(t, rec, storyID, f.activeID, "completed")
	requireProgressCompletedAt(t, body, second)
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusCompleted, &second)
}

// Escenario: Reestimación durante el sprint (004/US4)
// Una historia En progreso del sprint Activo, estimada en 5, se reestima en 8 con el PUT story-points
// de la 003 y después se completa: el completado se acepta con completedAt igual al reloj, y la
// historia queda completed con los 8 Story Points vigentes, que tomará el cierre (FR-034, RN-22).
func TestUS4_ReestimacionDuranteElSprint(t *testing.T) {
	f := newSprintUS4Fixture(t)
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusInProgress, nil)

	// La reestimación con la 003 se acepta mientras el sprint está Activo.
	reestimated := f.env.sprintDo(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", f.brunoToken, map[string]any{"storyPoints": 8})
	require.Equal(t, http.StatusOK, reestimated.Code, "cuerpo: %s", reestimated.Body.String())

	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed")

	body := requireStoryProgress(t, rec, storyID, f.activeID, "completed")
	requireProgressCompletedAt(t, body, sprintFixedNow)
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusCompleted, &sprintFixedNow)
	assert.Equal(t, sprintPoints(8), sprintStoredStory(t, f.env.db, storyID).StoryPoints, "la historia conserva la estimación vigente")
}

// Escenario: Último día del sprint (004/US4)
// Con el reloj en la fecha de fin prevista del sprint Activo (2026-10-28), completar una historia se
// acepta y registra ese instante, porque el sprint admite avance hasta que se cierra (FR-036).
func TestUS4_UltimoDiaDelSprint(t *testing.T) {
	lastDay := time.Date(2026, time.October, 28, 18, 0, 0, 0, time.UTC)
	f := newSprintUS4Fixture(t, func(p *sprintTestPorts) {
		p.now = func() time.Time { return lastDay }
	})
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusInProgress, nil)

	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed")

	body := requireStoryProgress(t, rec, storyID, f.activeID, "completed")
	requireProgressCompletedAt(t, body, lastDay)
	requireStoredProgress(t, f.env.db, f.activeID, storyID, backlogdomain.StoryStatusCompleted, &lastDay)
	assert.Equal(t, domain.SprintStatusActive, sprintStored(t, f.env.db, f.activeID.String()).Status, "el sprint sigue Activo")
}

// Escenario: Sprint Planificado (004/US4)
// Pasar a En progreso o a Completada una historia Pendiente comprometida en un sprint Planificado se
// rechaza con 409 porque solo se registra avance dentro del sprint activo; la historia sigue pending
// y su compromiso sin completed_at (FR-036).
func TestUS4_SprintPlanificado(t *testing.T) {
	for _, target := range []string{"in_progress", "completed"} {
		t.Run("a "+target, func(t *testing.T) {
			f := newSprintUS4Fixture(t)
			storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
			sprintSeedCommitment(t, f.env.db, f.sprintOneID, storyID, sprintFixedNow.Add(-time.Hour), nil, nil)
			before := sprintTakeProgressSnapshot(t, f.env.db, storyID)

			rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, target)

			requireProgressRejected(t, f.env.db, rec, sprintOutsideActiveMessage, before)
		})
	}
}

// Escenario: Historia sin sprint (004/US4)
// Cambiar el estado de una historia del backlog sin sprint se rechaza con 409 indicando que solo se
// registra avance sobre historias del sprint activo; la historia sigue pending y sin compromisos
// (FR-037).
func TestUS4_HistoriaSinSprint(t *testing.T) {
	for _, target := range []string{"in_progress", "completed"} {
		t.Run("a "+target, func(t *testing.T) {
			f := newSprintUS4Fixture(t)
			storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
			before := sprintTakeProgressSnapshot(t, f.env.db, storyID)

			rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, target)

			requireProgressRejected(t, f.env.db, rec, sprintOutsideActiveMessage, before)
			assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", storyID), "no debe crearse ningún compromiso")
		})
	}
}

// Escenario: Sprint Cerrado (004/US4)
// Devolver a En progreso una historia completed cuyo último compromiso es de un sprint cerrado se
// rechaza con 409 de sprint cerrado; la historia sigue completed y la fila congelada conserva su
// completed_at (FR-036, RN-18).
func TestUS4_SprintCerrado(t *testing.T) {
	f := newSprintUS4Fixture(t)
	closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	completedAt := time.Date(2026, time.September, 12, 17, 0, 0, 0, time.UTC)
	outcome := domain.StoryOutcomeCompleted
	sprintSeedCommitment(t, f.env.db, closedID, storyID, sprintDate(t, "2026-09-01"), &outcome, &completedAt)
	sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusCompleted)
	before := sprintTakeProgressSnapshot(t, f.env.db, storyID)

	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "in_progress")

	requireProgressRejected(t, f.env.db, rec, sprintClosedMessage, before)
}

// Escenario: Salto de Pendiente a Completada (004/US4)
// Pasar directamente a Completada una historia Pendiente del sprint Activo se rechaza con 409 que
// enumera las transiciones admitidas; la historia sigue pending y su compromiso sin completed_at
// (FR-038, RN-11).
func TestUS4_SaltoDePendienteACompletada(t *testing.T) {
	f := newSprintUS4Fixture(t)
	storyID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusPending, nil)
	before := sprintTakeProgressSnapshot(t, f.env.db, storyID)

	rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, "completed")

	requireProgressRejected(t, f.env.db, rec, sprintProgressTransitionText, before)
}

// Sin etiqueta de escenario: el 404 del avance sobre una historia ajena, o inexistente, es idéntico
// byte a byte (código, tipo de contenido y cuerpo) al que responde la 003 para la misma historia,
// porque la ruta reutiliza su RequireStoryAccess (research.md sección 2, FR-058); la historia ajena
// no cambia.
func TestStoryProgress_NotFoundMatchesBacklog(t *testing.T) {
	f := newSprintUS4Fixture(t)
	_, outsiderToken := f.env.signUp(t, "Carla Gómez", "carla@example.com")
	foreignID := f.commitInActive(t, "Exportar el reporte", backlogdomain.StoryStatusInProgress, nil)
	before := sprintTakeProgressSnapshot(t, f.env.db, foreignID)

	cases := []struct {
		name    string
		storyID uuid.UUID
	}{
		{name: "historia ajena", storyID: foreignID},
		{name: "historia inexistente", storyID: uuid.New()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storyID := tc.storyID
			backlogRec := f.env.sprintDo(t, http.MethodGet, "/api/v1/stories/"+storyID.String(), outsiderToken, nil)
			require.Equal(t, http.StatusNotFound, backlogRec.Code, "la 003 debe responder 404: %s", backlogRec.Body.String())

			rec := f.env.changeStoryStatus(t, outsiderToken, storyID, "completed")

			assert.Equal(t, backlogRec.Code, rec.Code, "los códigos de estado deberían ser idénticos")
			assert.Equal(t, backlogRec.Header().Get("Content-Type"), rec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
			assert.Equal(t, backlogRec.Body.Bytes(), rec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
		})
	}

	after := sprintTakeProgressSnapshot(t, f.env.db, foreignID)
	assert.Equal(t, before, after, "la historia ajena no debe cambiar")
}

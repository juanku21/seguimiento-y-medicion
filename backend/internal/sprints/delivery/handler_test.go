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
	"github.com/stretchr/testify/mock"
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
// de US5 (T064) cubren POST /api/v1/sprints/{sprintId}/close; las de US6 (T074) cubren GET
// /api/v1/projects/{projectId}/sprints, GET /api/v1/sprints/{sprintId} y GET
// /api/v1/stories/{storyId}/sprints; las de US7 (T082) cubren PUT y DELETE /api/v1/sprints/{sprintId}. Los ayudantes llevan el prefijo "sprint" para no chocar con los de validation_test.go,
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

// newSprintUS2Fixture compone el entorno con los overrides que pida la prueba (por ejemplo, el doble
// de esfuerzo de US7) y siembra las cuentas, el proyecto y el sprint Planificado.
func newSprintUS2Fixture(t *testing.T, overrides ...func(*sprintTestPorts)) sprintUS2Fixture {
	t.Helper()
	env := newSprintTestEnv(t, overrides...)
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

// Textos exactos que fija el contrato para los rechazos del cierre de un sprint (US5): el 409 del
// sprint Planificado y el del sprint ya Cerrado, cuyo estado es definitivo.
const (
	sprintCloseNotActiveMessage = "Solo se puede cerrar un sprint Activo."
	sprintCloseClosedMessage    = "El sprint ya está cerrado; el estado Cerrado es definitivo."
)

// sprintClosedAt es el momento de cierre que debe devolver la API: el del reloj inyectado, en UTC con
// formato RFC 3339.
var sprintClosedAt = sprintFixedNow.UTC().Format(time.RFC3339)

// sprintResultsBody refleja SprintResults del contrato: los cinco totales de la instantánea.
type sprintResultsBody struct {
	CommittedStoryPoints int `json:"committedStoryPoints"`
	PlannedStoryPoints   int `json:"plannedStoryPoints"`
	CompletedStoryPoints int `json:"completedStoryPoints"`
	PlannedStoryCount    int `json:"plannedStoryCount"`
	CompletedStoryCount  int `json:"completedStoryCount"`
}

// sprintClosedBody refleja de SprintDetail del contrato lo que verifican las pruebas de US5: el
// estado, las cuatro fechas, quién cerró, las historias congeladas, los totales y el factor.
type sprintClosedBody struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	StartDate string  `json:"startDate"`
	EndDate   string  `json:"endDate"`
	StartedAt *string `json:"startedAt"`
	ClosedAt  *string `json:"closedAt"`
	CreatedBy struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"createdBy"`
	ClosedBy *struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"closedBy"`
	Stories []struct {
		StoryID     string  `json:"storyId"`
		Title       string  `json:"title"`
		Priority    string  `json:"priority"`
		StoryPoints int     `json:"storyPoints"`
		Status      string  `json:"status"`
		CompletedAt *string `json:"completedAt"`
		Outcome     *string `json:"outcome"`
	} `json:"stories"`
	Results            *sprintResultsBody `json:"results"`
	HoursPerStoryPoint *float64           `json:"hoursPerStoryPoint"`
}

// sprintFrozenRow es lo que la instantánea debe conservar de una historia al cerrar, además de su
// prioridad: su título, sus Story Points y su resultado.
type sprintFrozenRow struct {
	title   string
	points  int
	outcome domain.StoryOutcome
}

// sprintClosePath es la ruta de cierre de un sprint según contracts/openapi.yaml.
func sprintClosePath(sprintID uuid.UUID) string {
	return sprintPath(sprintID) + "/close"
}

// closeSprint envía POST /api/v1/sprints/{sprintId}/close, sin cuerpo, con el token como Bearer.
func (e sprintTestEnv) closeSprint(t *testing.T, token string, sprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodPost, sprintClosePath(sprintID), token, nil)
}

// requireClosedSprint exige un 200 cuyo cuerpo sea el detalle del sprint closed, con closedAt igual
// al reloj, closedBy igual a quien cerró, el factor congelado del proyecto (6,5), results presente y
// schedule null, y lo devuelve decodificado junto con el cuerpo crudo.
func requireClosedSprint(t *testing.T, rec *httptest.ResponseRecorder, sprintID, closerID uuid.UUID, closerName string) (sprintClosedBody, map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintClosedBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	raw := sprintRawBody(t, rec)
	assert.Equal(t, sprintID.String(), body.ID)
	assert.Equal(t, "closed", body.Status)
	if assert.NotNil(t, body.ClosedAt, "closedAt debe registrar el momento real de cierre") {
		assert.Equal(t, sprintClosedAt, *body.ClosedAt, "closedAt debe ser el instante del reloj")
	}
	if assert.NotNil(t, body.ClosedBy, "closedBy debe identificar a quien cerró") {
		assert.Equal(t, closerID.String(), body.ClosedBy.ID, "closedBy es el titular de la sesión")
		assert.Equal(t, closerName, body.ClosedBy.FullName)
	}
	if assert.NotNil(t, body.HoursPerStoryPoint, "hoursPerStoryPoint debe congelar el factor del proyecto") {
		assert.InDelta(t, 6.5, *body.HoursPerStoryPoint, 0.0001)
	}
	schedule, present := raw["schedule"]
	assert.True(t, present, "falta el campo schedule en el detalle")
	assert.Nil(t, schedule, "schedule debe ser null en un sprint cerrado")
	require.NotNil(t, body.Results, "results no puede ser null en un sprint cerrado: %s", rec.Body.String())
	return body, raw
}

// requireStoredClosed exige que la fila del sprint quede closed, con closed_at igual al reloj,
// closed_by_id igual a quien cerró y el factor del proyecto copiado en centésimas (650), y la
// devuelve.
func requireStoredClosed(t *testing.T, db *gorm.DB, sprintID, closerID uuid.UUID) domain.Sprint {
	t.Helper()
	stored := sprintStored(t, db, sprintID.String())
	assert.Equal(t, domain.SprintStatusClosed, stored.Status)
	if assert.NotNil(t, stored.ClosedAt, "closed_at debe quedar registrado") {
		assert.True(t, sprintFixedNow.Equal(*stored.ClosedAt), "closed_at = %v, se esperaba %v", *stored.ClosedAt, sprintFixedNow)
	}
	if assert.NotNil(t, stored.ClosedByID, "closed_by_id debe quedar registrado") {
		assert.Equal(t, closerID, *stored.ClosedByID)
	}
	if assert.NotNil(t, stored.HoursPerStoryPointHundredths, "el factor debe copiarse al cerrar") {
		assert.Equal(t, 650, *stored.HoursPerStoryPointHundredths)
	}
	return stored
}

// requireFrozenRows exige que el sprint conserve exactamente una fila congelada por cada historia
// esperada, con su título, prioridad high, Story Points y resultado al cierre, y completed_at solo
// en las completadas. También exige que la respuesta liste las mismas historias con esos datos.
func requireFrozenRows(t *testing.T, db *gorm.DB, body sprintClosedBody, sprintID uuid.UUID, want map[uuid.UUID]sprintFrozenRow) {
	t.Helper()
	rows := sprintStoredRows(t, db, sprintID)
	require.Len(t, rows, len(want), "el sprint debe conservar una fila por historia comprometida al cierre")
	for _, row := range rows {
		expected, ok := want[row.StoryID]
		if !assert.True(t, ok, "la historia %s no debería figurar en la instantánea", row.StoryID) {
			continue
		}
		if assert.NotNil(t, row.TitleAtClose, "title_at_close de %s", row.StoryID) {
			assert.Equal(t, expected.title, *row.TitleAtClose)
		}
		if assert.NotNil(t, row.PriorityAtClose, "priority_at_close de %s", row.StoryID) {
			assert.Equal(t, backlogdomain.StoryPriorityHigh, *row.PriorityAtClose)
		}
		if assert.NotNil(t, row.StoryPointsAtClose, "story_points_at_close de %s", row.StoryID) {
			assert.Equal(t, expected.points, *row.StoryPointsAtClose)
		}
		if assert.NotNil(t, row.Outcome, "outcome de %s", row.StoryID) {
			assert.Equal(t, expected.outcome, *row.Outcome)
		}
		if expected.outcome == domain.StoryOutcomeCompleted {
			assert.NotNil(t, row.CompletedAt, "completed_at se conserva en la historia completada %s", row.StoryID)
		} else {
			assert.Nil(t, row.CompletedAt, "completed_at queda null en la historia no completada %s", row.StoryID)
		}
	}

	// La respuesta lista las mismas historias con sus datos congelados.
	require.Len(t, body.Stories, len(want), "el detalle debe listar las historias de la instantánea")
	for _, story := range body.Stories {
		id, err := uuid.Parse(story.StoryID)
		require.NoError(t, err, "storyId = %q no es un UUID", story.StoryID)
		expected, ok := want[id]
		if !assert.True(t, ok, "la historia %s no debería figurar en el detalle", id) {
			continue
		}
		assert.Equal(t, expected.title, story.Title)
		assert.Equal(t, "high", story.Priority)
		assert.Equal(t, expected.points, story.StoryPoints)
		if assert.NotNil(t, story.Outcome, "outcome de %s en el detalle", id) {
			assert.Equal(t, string(expected.outcome), *story.Outcome)
		}
	}
}

// requireBackInBacklog exige que cada historia quede pending y sin ningún compromiso abierto, es
// decir, de vuelta en el backlog y sin sprint (FR-041).
func requireBackInBacklog(t *testing.T, db *gorm.DB, storyIDs ...uuid.UUID) {
	t.Helper()
	for _, id := range storyIDs {
		assert.Equal(t, backlogdomain.StoryStatusPending, sprintStoredStory(t, db, id).Status, "la historia %s debe volver a pending", id)
		assert.Zero(t, sprintCommitmentCount(t, db, "story_id = ? AND outcome IS NULL", id), "la historia %s no debe conservar un compromiso abierto", id)
	}
}

// sprintStoredRows lee todas las filas de sprint_stories del sprint, ordenadas por id, para verificar
// la instantánea y compararla antes y después de un cambio.
func sprintStoredRows(t *testing.T, db *gorm.DB, sprintID uuid.UUID) []domain.SprintStory {
	t.Helper()
	var rows []domain.SprintStory
	require.NoError(t, db.Where("sprint_id = ?", sprintID).Order("id").Find(&rows).Error, "no se pudieron leer las filas del sprint")
	return rows
}

// sprintUS5Fixture es el "Dado" común de US5: Ana, propietaria de un proyecto En curso con factor
// 6,5, y Bruno, integrante que no es el propietario y que ejecuta las acciones (RN-01), con un sprint
// Planificado "Sprint 1" creado por Ana en el período recibido.
type sprintUS5Fixture struct {
	sprintUS2Fixture
}

// newSprintUS5Fixture compone el entorno y siembra las cuentas, el proyecto y el sprint Planificado
// con el período indicado.
func newSprintUS5Fixture(t *testing.T, start, end string) sprintUS5Fixture {
	t.Helper()
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	sprintOneID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", start, end)
	return sprintUS5Fixture{sprintUS2Fixture{env: env, anaID: anaID, brunoID: brunoID, brunoToken: brunoToken, projectID: projectID, sprintOneID: sprintOneID}}
}

// newStories crea con la 003 una historia lista por cada valor de Story Points, con título
// "Historia N", y devuelve sus identificadores en el mismo orden.
func (f sprintUS5Fixture) newStories(t *testing.T, points ...int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		ids = append(ids, f.env.createStory(t, f.brunoToken, f.projectID, fmt.Sprintf("Historia %d", i+1), sprintPoints(p), sprintCriterion))
	}
	return ids
}

// assign compromete las historias en "Sprint 1" con la API de US2.
func (f sprintUS5Fixture) assign(t *testing.T, storyIDs ...uuid.UUID) {
	t.Helper()
	rec := f.env.assignStories(t, f.brunoToken, f.sprintOneID, storyIDs...)
	require.Equal(t, http.StatusOK, rec.Code, "no se pudieron asignar las historias: %s", rec.Body.String())
}

// activeWith crea historias listas con los Story Points indicados, las compromete en "Sprint 1" y lo
// inicia con la API de US3, que congela los Story Points comprometidos; devuelve los identificadores
// de las historias.
func (f sprintUS5Fixture) activeWith(t *testing.T, points ...int) []uuid.UUID {
	t.Helper()
	ids := f.newStories(t, points...)
	if len(ids) > 0 {
		f.assign(t, ids...)
	}
	rec := f.env.startSprint(t, f.brunoToken, f.sprintOneID)
	require.Equal(t, http.StatusOK, rec.Code, "no se pudo iniciar el sprint: %s", rec.Body.String())
	return ids
}

// advance lleva la historia del sprint Activo hasta el estado indicado con la API de US4, pasando
// por En progreso.
func (f sprintUS5Fixture) advance(t *testing.T, storyID uuid.UUID, to string) {
	t.Helper()
	targets := []string{"in_progress"}
	if to == "completed" {
		targets = append(targets, "completed")
	}
	for _, target := range targets {
		rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, target)
		require.Equal(t, http.StatusOK, rec.Code, "no se pudo pasar la historia a %s: %s", target, rec.Body.String())
	}
}

// Escenario: Cierre parcial (004/US5)
// Bruno cierra un sprint Activo con cuatro historias de 8, 5, 5 y 3 Story Points (21), dos completadas
// por 13, una en progreso y una pendiente: responde 200 con el sprint closed, 21 planificados y 13
// completados; las dos no completadas vuelven pending y sin compromiso abierto, y el sprint conserva
// cuatro filas congeladas (FR-039, FR-041 a FR-043, RN-15).
func TestUS5_CierreParcial(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	ids := f.activeWith(t, 8, 5, 5, 3)
	f.advance(t, ids[0], "completed")
	f.advance(t, ids[1], "completed")
	f.advance(t, ids[2], "in_progress")

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, sprintResultsBody{CommittedStoryPoints: 21, PlannedStoryPoints: 21, CompletedStoryPoints: 13, PlannedStoryCount: 4, CompletedStoryCount: 2}, *body.Results)
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
	requireFrozenRows(t, f.env.db, body, f.sprintOneID, map[uuid.UUID]sprintFrozenRow{
		ids[0]: {title: "Historia 1", points: 8, outcome: domain.StoryOutcomeCompleted},
		ids[1]: {title: "Historia 2", points: 5, outcome: domain.StoryOutcomeCompleted},
		ids[2]: {title: "Historia 3", points: 5, outcome: domain.StoryOutcomeNotCompleted},
		ids[3]: {title: "Historia 4", points: 3, outcome: domain.StoryOutcomeNotCompleted},
	})

	// Las no completadas vuelven al backlog y las completadas siguen completed.
	requireBackInBacklog(t, f.env.db, ids[2], ids[3])
	for _, id := range ids[:2] {
		assert.Equal(t, backlogdomain.StoryStatusCompleted, sprintStoredStory(t, f.env.db, id).Status)
	}
}

// Escenario: Todas completadas (004/US5)
// Un sprint Activo cuyas dos historias (5 y 3) están Completadas se cierra: ninguna vuelve al backlog
// y los Story Points planificados al cierre coinciden con los completados (8).
func TestUS5_TodasCompletadas(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	ids := f.activeWith(t, 5, 3)
	for _, id := range ids {
		f.advance(t, id, "completed")
	}

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, sprintResultsBody{CommittedStoryPoints: 8, PlannedStoryPoints: 8, CompletedStoryPoints: 8, PlannedStoryCount: 2, CompletedStoryCount: 2}, *body.Results)
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
	requireFrozenRows(t, f.env.db, body, f.sprintOneID, map[uuid.UUID]sprintFrozenRow{
		ids[0]: {title: "Historia 1", points: 5, outcome: domain.StoryOutcomeCompleted},
		ids[1]: {title: "Historia 2", points: 3, outcome: domain.StoryOutcomeCompleted},
	})
	for _, id := range ids {
		assert.Equal(t, backlogdomain.StoryStatusCompleted, sprintStoredStory(t, f.env.db, id).Status, "ninguna historia vuelve al backlog")
	}
}

// Escenario: Alcance agregado (004/US5)
// Un sprint iniciado con 20 Story Points que suma dos historias de 8 mientras está Activo cierra con
// 20 comprometidos al inicio y 36 planificados al cierre; si en cambio pierde una historia de 5
// durante el sprint, cierra con 20 y 15 y la historia quitada no figura en la instantánea. En ambos
// casos la diferencia es distinta de cero y deja visible el cambio de alcance (FR-043, SC-009).
func TestUS5_AlcanceAgregado(t *testing.T) {
	t.Run("suma dos historias de 8", func(t *testing.T) {
		f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
		initial := f.activeWith(t, 5, 5, 5, 5)
		added := f.newStories(t, 8, 8)
		f.assign(t, added...)

		rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

		body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
		assert.Equal(t, sprintResultsBody{CommittedStoryPoints: 20, PlannedStoryPoints: 36, CompletedStoryPoints: 0, PlannedStoryCount: 6, CompletedStoryCount: 0}, *body.Results)
		assert.Equal(t, 16, body.Results.PlannedStoryPoints-body.Results.CommittedStoryPoints, "la diferencia expresa el alcance agregado")
		requireBackInBacklog(t, f.env.db, append(initial, added...)...)
	})

	t.Run("quita una historia de 5", func(t *testing.T) {
		f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
		ids := f.activeWith(t, 8, 5, 5, 2)
		removed := f.env.removeStories(t, f.brunoToken, f.sprintOneID, ids[1])
		require.Equal(t, http.StatusOK, removed.Code, "no se pudo quitar la historia: %s", removed.Body.String())

		rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

		body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
		assert.Equal(t, sprintResultsBody{CommittedStoryPoints: 20, PlannedStoryPoints: 15, CompletedStoryPoints: 0, PlannedStoryCount: 3, CompletedStoryCount: 0}, *body.Results)
		assert.Equal(t, -5, body.Results.PlannedStoryPoints-body.Results.CommittedStoryPoints, "la diferencia negativa expresa el alcance quitado")
		requireFrozenRows(t, f.env.db, body, f.sprintOneID, map[uuid.UUID]sprintFrozenRow{
			ids[0]: {title: "Historia 1", points: 8, outcome: domain.StoryOutcomeNotCompleted},
			ids[2]: {title: "Historia 3", points: 5, outcome: domain.StoryOutcomeNotCompleted},
			ids[3]: {title: "Historia 4", points: 2, outcome: domain.StoryOutcomeNotCompleted},
		})
	})
}

// Escenario: Inmutable tras el cierre (004/US5)
// Cerrado un sprint con una historia completada de 8 y una pendiente de 5, renombrar y reestimar la
// pendiente con la 003 y cambiar el factor del proyecto a 8 con la 002 no altera el sprint: el
// detalle es idéntico byte a byte antes y después, y la fila del sprint y sus filas congeladas en la
// base no cambian (FR-044, SC-008).
func TestUS5_InmutableTrasElCierre(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	ids := f.activeWith(t, 8, 5)
	f.advance(t, ids[0], "completed")
	closed := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)
	requireClosedSprint(t, closed, f.sprintOneID, f.brunoID, "Bruno Díaz")

	// Foto del detalle y de la instantánea en la base antes de los cambios.
	detailBefore := f.env.sprintDo(t, http.MethodGet, sprintPath(f.sprintOneID), f.brunoToken, nil)
	require.Equal(t, http.StatusOK, detailBefore.Code, "cuerpo: %s", detailBefore.Body.String())
	sprintBefore := sprintStored(t, f.env.db, f.sprintOneID.String())
	rowsBefore := sprintStoredRows(t, f.env.db, f.sprintOneID)

	// Renombre y reestimación de la historia devuelta al backlog, con la 003.
	renamed := f.env.sprintDo(t, http.MethodPatch, "/api/v1/stories/"+ids[1].String(), f.brunoToken, map[string]any{"title": "Historia 2 renombrada"})
	require.Equal(t, http.StatusOK, renamed.Code, "no se pudo renombrar la historia: %s", renamed.Body.String())
	reestimated := f.env.sprintDo(t, http.MethodPut, "/api/v1/stories/"+ids[1].String()+"/story-points", f.brunoToken, map[string]any{"storyPoints": 13})
	require.Equal(t, http.StatusOK, reestimated.Code, "no se pudo reestimar la historia: %s", reestimated.Body.String())

	// Cambio del factor del proyecto a 8 con la 002, por su propietaria, conservando los demás datos.
	login, err := f.env.auth.Login(context.Background(), "ana@example.com", sprintUserPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de Ana")
	project := sprintStoredProject(t, f.env.db, f.projectID)
	factor := f.env.sprintDo(t, http.MethodPut, "/api/v1/projects/"+f.projectID.String(), login.Token, map[string]any{
		"name":               project.Name,
		"description":        nil,
		"startDate":          project.StartDate.UTC().Format(time.DateOnly),
		"plannedEndDate":     project.PlannedEndDate.UTC().Format(time.DateOnly),
		"hoursPerStoryPoint": 8,
	})
	require.Equal(t, http.StatusOK, factor.Code, "no se pudo cambiar el factor del proyecto: %s", factor.Body.String())

	// Los cambios ocurrieron de verdad en la historia y en el proyecto.
	story := sprintStoredStory(t, f.env.db, ids[1])
	require.Equal(t, "Historia 2 renombrada", story.Title)
	require.Equal(t, sprintPoints(13), story.StoryPoints)
	require.Equal(t, 800, sprintStoredProject(t, f.env.db, f.projectID).HoursPerStoryPointHundredths)

	// El detalle es idéntico byte a byte y la instantánea en la base no cambió.
	detailAfter := f.env.sprintDo(t, http.MethodGet, sprintPath(f.sprintOneID), f.brunoToken, nil)
	require.Equal(t, http.StatusOK, detailAfter.Code, "cuerpo: %s", detailAfter.Body.String())
	assert.Equal(t, detailBefore.Body.String(), detailAfter.Body.String(), "el detalle del sprint cerrado debe ser idéntico byte a byte")
	assert.Equal(t, sprintBefore, sprintStored(t, f.env.db, f.sprintOneID.String()), "la fila del sprint cerrado no debe cambiar")
	assert.Equal(t, rowsBefore, sprintStoredRows(t, f.env.db, f.sprintOneID), "las filas congeladas no deben cambiar")
}

// Escenario: Sin completadas (004/US5)
// Un sprint Activo con una historia pendiente (5) y una en progreso (3) se cierra igual: los Story
// Points completados quedan en 0, las dos historias vuelven al backlog y el sprint conserva sus dos
// filas como not_completed (FR-045).
func TestUS5_SinCompletadas(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	ids := f.activeWith(t, 5, 3)
	f.advance(t, ids[1], "in_progress")

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, sprintResultsBody{CommittedStoryPoints: 8, PlannedStoryPoints: 8, CompletedStoryPoints: 0, PlannedStoryCount: 2, CompletedStoryCount: 0}, *body.Results)
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
	requireFrozenRows(t, f.env.db, body, f.sprintOneID, map[uuid.UUID]sprintFrozenRow{
		ids[0]: {title: "Historia 1", points: 5, outcome: domain.StoryOutcomeNotCompleted},
		ids[1]: {title: "Historia 2", points: 3, outcome: domain.StoryOutcomeNotCompleted},
	})
	requireBackInBacklog(t, f.env.db, ids...)
}

// Escenario: Sprint vacío (004/US5)
// Un sprint Activo sin historias se cierra con la instantánea en cero: los cinco totales en 0 (y no
// null), stories [] y ninguna fila en sprint_stories (FR-045).
func TestUS5_SprintVacio(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	f.activeWith(t)

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, raw := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, sprintResultsBody{}, *body.Results, "los cinco totales deben ser 0")
	assert.Equal(t, []any{}, raw["stories"], "stories debe ser [] y no null")
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
	assert.Zero(t, sprintCommitmentCount(t, f.env.db, "sprint_id = ?", f.sprintOneID), "un sprint vacío no tiene filas congeladas")
}

// Escenario: Cierre anticipado (004/US5)
// Un sprint Activo cuya fecha de fin (2026-10-28) todavía no llegó según el reloj (2026-10-21) se
// cierra: responde 200 con closedAt igual al reloj, el momento real de cierre, y las fechas previstas
// y el inicio real no cambian (FR-040).
func TestUS5_CierreAnticipado(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	f.activeWith(t, 5)
	require.True(t, sprintDate(t, "2026-10-28").After(sprintFixedNow), "la fecha de fin debe ser futura respecto del reloj")

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, "2026-10-15", body.StartDate, "la fecha de inicio prevista no cambia")
	assert.Equal(t, "2026-10-28", body.EndDate, "la fecha de fin prevista no cambia")
	if assert.NotNil(t, body.StartedAt, "el inicio real se conserva") {
		assert.Equal(t, sprintStartedAt, *body.StartedAt)
	}
	stored := requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
	assert.Equal(t, sprintDate(t, "2026-10-28"), stored.EndDate.UTC())
}

// Escenario: Cierre tardío (004/US5)
// Un sprint Activo cuya fecha de fin (2026-10-14) ya pasó según el reloj (2026-10-21) se cierra con
// 200, sin ningún aviso ni error en la respuesta, y queda registrado el momento real de cierre
// (FR-040).
func TestUS5_CierreTardio(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-01", "2026-10-14")
	f.activeWith(t, 5)
	require.True(t, sprintDate(t, "2026-10-14").Before(sprintFixedNow), "la fecha de fin debe haber pasado respecto del reloj")

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, raw := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.NotContains(t, raw, "error", "el cierre tardío no lleva aviso de error")
	assert.Equal(t, "2026-10-14", body.EndDate, "la fecha de fin prevista no cambia")
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
}

// Escenario: Sprint planificado (004/US5)
// Cerrar un sprint Planificado con una historia comprometida se rechaza con 409 indicando que solo se
// cierra un sprint Activo; el sprint sigue planned sin datos de cierre y la historia conserva su
// compromiso abierto (FR-047, data-model.md sección 4).
func TestUS5_SprintPlanificado(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	f.assign(t, f.newStories(t, 5)...)
	before := sprintStored(t, f.env.db, f.sprintOneID.String())

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintCloseNotActiveMessage+`"}`, rec.Body.String())
	requireStoredUnchanged(t, f.env.db, before)
	after := sprintStored(t, f.env.db, f.sprintOneID.String())
	assert.Nil(t, after.ClosedAt, "closed_at debe seguir en null")
	assert.Nil(t, after.ClosedByID, "closed_by_id debe seguir en null")
	assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND outcome IS NULL", f.sprintOneID), "el compromiso sigue abierto")
}

// Escenario: Sprint cerrado (004/US5)
// Sobre un sprint ya Cerrado, cerrarlo de nuevo e iniciarlo (el intento de reabrirlo) se rechazan con
// 409 porque el estado Cerrado es definitivo; el sprint conserva sus datos de cierre y su fila
// congelada, y la historia sigue completed (FR-046, RN-18).
func TestUS5_SprintCerrado(t *testing.T) {
	cases := []struct {
		name    string
		send    func(e sprintTestEnv, t *testing.T, token string, sprintID uuid.UUID) *httptest.ResponseRecorder
		message string
	}{
		{name: "cerrar de nuevo", send: sprintTestEnv.closeSprint, message: sprintCloseClosedMessage},
		{name: "iniciar", send: sprintTestEnv.startSprint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
			closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
			storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
			completedAt := time.Date(2026, time.September, 12, 17, 0, 0, 0, time.UTC)
			outcome := domain.StoryOutcomeCompleted
			sprintSeedCommitment(t, f.env.db, closedID, storyID, sprintDate(t, "2026-09-01"), &outcome, &completedAt)
			sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusCompleted)
			sprintBefore := sprintStored(t, f.env.db, closedID.String())
			rowsBefore := sprintStoredRows(t, f.env.db, closedID)

			rec := tc.send(f.env, t, f.brunoToken, closedID)

			// El cierre repetido usa el texto exacto del contrato; el inicio, el rechazo que ya
			// fija US3 para un sprint cerrado.
			require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
			if tc.message != "" {
				assert.JSONEq(t, `{"error":"`+tc.message+`"}`, rec.Body.String())
			} else {
				assert.NotEmpty(t, sprintRawBody(t, rec)["error"], "el 409 debe explicar el rechazo: %s", rec.Body.String())
			}
			assert.Equal(t, sprintBefore, sprintStored(t, f.env.db, closedID.String()), "la fila del sprint cerrado no debe cambiar")
			assert.Equal(t, rowsBefore, sprintStoredRows(t, f.env.db, closedID), "la fila congelada no debe cambiar")
			assert.Equal(t, backlogdomain.StoryStatusCompleted, sprintStoredStory(t, f.env.db, storyID).Status)
		})
	}
}

// Escenario: Proyecto finalizado (004/US5)
// En un proyecto Finalizado con un sprint Activo que tiene una historia En progreso, cerrar el sprint
// se rechaza con 409 porque sus sprints son de solo lectura: el sprint sigue active sin datos de
// cierre, la historia sigue in_progress y su compromiso sigue abierto (FR-059, RN-19). La historia
// se crea con la 003 mientras el proyecto está En curso y después se finaliza el proyecto en la base.
func TestUS5_ProyectoFinalizado(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-11-01", "2026-11-14", domain.SprintStatusActive)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	sprintSeedCommitment(t, f.env.db, activeID, storyID, sprintDate(t, "2026-11-01"), nil, nil)
	sprintSetStoryStatus(t, f.env.db, storyID, backlogdomain.StoryStatusInProgress)
	require.NoError(t, f.env.db.Model(&projectsdomain.Project{}).Where("id = ?", f.projectID).
		Update("status", projectsdomain.ProjectStatusFinished).Error, "no se pudo finalizar el proyecto")
	before := sprintStored(t, f.env.db, activeID.String())

	rec := f.env.closeSprint(t, f.brunoToken, activeID)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintReadOnlyMessage+`"}`, rec.Body.String())
	requireStoredUnchanged(t, f.env.db, before)
	after := sprintStored(t, f.env.db, activeID.String())
	assert.Nil(t, after.ClosedAt, "closed_at debe seguir en null")
	assert.Nil(t, after.ClosedByID, "closed_by_id debe seguir en null")
	assert.Equal(t, backlogdomain.StoryStatusInProgress, sprintStoredStory(t, f.env.db, storyID).Status)
	assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "story_id = ? AND outcome IS NULL", storyID), "el compromiso sigue abierto")
}

// Sin etiqueta de escenario: cerrar un sprint no cambia el estado del proyecto, esté Planificado o
// En curso, ni escribe su fila (FR-049, RN-07).
func TestSprintClose_ProjectStatusUntouched(t *testing.T) {
	for _, status := range []projectsdomain.ProjectStatus{projectsdomain.ProjectStatusPlanned, projectsdomain.ProjectStatusInProgress} {
		t.Run(string(status), func(t *testing.T) {
			env := newSprintTestEnv(t)
			anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
			projectID := sprintSeedProject(t, env.db, anaID, status, brunoID)
			sprintID := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 1", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
			before := sprintStoredProject(t, env.db, projectID)

			rec := env.closeSprint(t, brunoToken, sprintID)

			requireClosedSprint(t, rec, sprintID, brunoID, "Bruno Díaz")
			env.requireProjectUntouched(t, brunoToken, before, status)
		})
	}
}

// Sin etiqueta de escenario: closedBy es el titular de la sesión que cierra, no el autor del sprint.
// Ana crea el sprint y Bruno, integrante que no es el propietario, lo cierra: la respuesta informa a
// Bruno en closedBy y a Ana en createdBy, y closed_by_id guarda a Bruno (FR-040, RN-01).
func TestSprintClose_ClosedByIsSessionHolder(t *testing.T) {
	f := newSprintUS5Fixture(t, "2026-10-15", "2026-10-28")
	f.activeWith(t, 5)

	rec := f.env.closeSprint(t, f.brunoToken, f.sprintOneID)

	body, _ := requireClosedSprint(t, rec, f.sprintOneID, f.brunoID, "Bruno Díaz")
	assert.Equal(t, f.anaID.String(), body.CreatedBy.ID, "createdBy sigue siendo la autora")
	require.NotNil(t, body.ClosedBy)
	assert.NotEqual(t, body.CreatedBy.ID, body.ClosedBy.ID, "quien cierra no es la autora")
	requireStoredClosed(t, f.env.db, f.sprintOneID, f.brunoID)
}

// sprintListItemBody refleja SprintListItem del contrato: los datos del sprint, sus cuatro fechas,
// la cantidad de historias y los totales de la instantánea, null si no está cerrado.
type sprintListItemBody struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Goal       string             `json:"goal"`
	Status     string             `json:"status"`
	StartDate  string             `json:"startDate"`
	EndDate    string             `json:"endDate"`
	StartedAt  *string            `json:"startedAt"`
	ClosedAt   *string            `json:"closedAt"`
	StoryCount int                `json:"storyCount"`
	Results    *sprintResultsBody `json:"results"`
}

// sprintListBody refleja SprintListResponse del contrato.
type sprintListBody struct {
	Sprints        []sprintListItemBody `json:"sprints"`
	AppliedFilters struct {
		Status []string `json:"status"`
	} `json:"appliedFilters"`
	EmptyReason *string `json:"emptyReason"`
}

// sprintScheduleBody refleja SprintSchedule del contrato: exactamente uno de los dos campos es
// distinto de null.
type sprintScheduleBody struct {
	RemainingDays *int `json:"remainingDays"`
	OverdueDays   *int `json:"overdueDays"`
}

// sprintStoryHistoryBody refleja StorySprintHistoryResponse del contrato.
type sprintStoryHistoryBody struct {
	Sprints []struct {
		SprintID     string  `json:"sprintId"`
		SprintName   string  `json:"sprintName"`
		SprintStatus string  `json:"sprintStatus"`
		StartDate    string  `json:"startDate"`
		EndDate      string  `json:"endDate"`
		Outcome      *string `json:"outcome"`
		CompletedAt  *string `json:"completedAt"`
	} `json:"sprints"`
}

// listSprints envía GET /api/v1/projects/{projectId}/sprints con un parámetro status por cada estado
// recibido.
func (e sprintTestEnv) listSprints(t *testing.T, token string, projectID uuid.UUID, statuses ...string) *httptest.ResponseRecorder {
	t.Helper()
	path := sprintsPath(projectID)
	if len(statuses) > 0 {
		params := make([]string, 0, len(statuses))
		for _, s := range statuses {
			params = append(params, "status="+s)
		}
		path += "?" + strings.Join(params, "&")
	}
	return e.sprintDo(t, http.MethodGet, path, token, nil)
}

// requireSprintList exige un 200 con SprintListResponse, con sprints y appliedFilters.status como
// listas y nunca null y con el campo emptyReason presente, y devuelve el cuerpo decodificado junto
// con el crudo.
func requireSprintList(t *testing.T, rec *httptest.ResponseRecorder) (sprintListBody, map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintListBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	raw := sprintRawBody(t, rec)
	_, isList := raw["sprints"].([]any)
	require.True(t, isList, "sprints debe ser una lista y no null: %s", rec.Body.String())
	filters, ok := raw["appliedFilters"].(map[string]any)
	require.True(t, ok, "falta appliedFilters: %s", rec.Body.String())
	_, isList = filters["status"].([]any)
	require.True(t, isList, "appliedFilters.status debe ser una lista y no null: %s", rec.Body.String())
	_, present := raw["emptyReason"]
	require.True(t, present, "falta el campo emptyReason: %s", rec.Body.String())
	return body, raw
}

// sprintListIDs devuelve los identificadores del listado, en su orden.
func sprintListIDs(body sprintListBody) []string {
	ids := make([]string, 0, len(body.Sprints))
	for _, s := range body.Sprints {
		ids = append(ids, s.ID)
	}
	return ids
}

// getSprintDetail envía GET /api/v1/sprints/{sprintId} con el token como Bearer.
func (e sprintTestEnv) getSprintDetail(t *testing.T, token string, sprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodGet, sprintPath(sprintID), token, nil)
}

// requireSchedule exige un 200 con el detalle del sprint cuyo schedule es un objeto con
// remainingDays y overdueDays, y lo devuelve decodificado.
func requireSchedule(t *testing.T, rec *httptest.ResponseRecorder) sprintScheduleBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	raw := sprintRawBody(t, rec)
	schedule, ok := raw["schedule"].(map[string]any)
	require.True(t, ok, "schedule debe ser un objeto en un sprint activo: %s", rec.Body.String())
	_, hasRemaining := schedule["remainingDays"]
	_, hasOverdue := schedule["overdueDays"]
	require.True(t, hasRemaining && hasOverdue, "schedule debe traer remainingDays y overdueDays: %s", rec.Body.String())
	var body struct {
		Schedule sprintScheduleBody `json:"schedule"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body.Schedule
}

// listStorySprints envía GET /api/v1/stories/{storyId}/sprints con el token como Bearer.
func (e sprintTestEnv) listStorySprints(t *testing.T, token string, storyID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodGet, "/api/v1/stories/"+storyID.String()+"/sprints", token, nil)
}

// requireStoryHistory exige un 200 con StorySprintHistoryResponse cuyo sprints sea una lista y no
// null, y lo devuelve decodificado.
func requireStoryHistory(t *testing.T, rec *httptest.ResponseRecorder) sprintStoryHistoryBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	_, isList := sprintRawBody(t, rec)["sprints"].([]any)
	require.True(t, isList, "sprints debe ser una lista y no null: %s", rec.Body.String())
	var body sprintStoryHistoryBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body
}

// sprintUTC formatea un instante como lo devuelve la API: en UTC con formato RFC 3339.
func sprintUTC(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

// sprintMoment arma un instante UTC a partir de una fecha YYYY-MM-DD y una hora. Las pruebas usan el
// mediodía UTC o poco después, que cae en la misma fecha de calendario en la zona del sistema.
func sprintMoment(t *testing.T, date string, hour, minute int) time.Time {
	t.Helper()
	return sprintDate(t, date).Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// sprintUS6Fixture es el "Dado" común de US6: Ana, propietaria de un proyecto En curso, y Bruno,
// integrante que no es el propietario y que consulta (RN-01), sin sprints. El reloj del servicio es
// mutable, para que cada inicio y cada cierre registre su propio instante.
type sprintUS6Fixture struct {
	env        sprintTestEnv
	anaID      uuid.UUID
	brunoID    uuid.UUID
	brunoToken string
	projectID  uuid.UUID
	clock      *time.Time
}

// newSprintUS6Fixture compone el entorno con el reloj mutable, que arranca en sprintFixedNow, y
// siembra las cuentas y el proyecto.
func newSprintUS6Fixture(t *testing.T) sprintUS6Fixture {
	t.Helper()
	clock := sprintFixedNow
	env := newSprintTestEnv(t, func(p *sprintTestPorts) {
		p.now = func() time.Time { return clock }
	})
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	return sprintUS6Fixture{env: env, anaID: anaID, brunoID: brunoID, brunoToken: brunoToken, projectID: projectID, clock: &clock}
}

// setClock fija el instante que devuelve el reloj del servicio desde ahora.
func (f sprintUS6Fixture) setClock(moment time.Time) {
	*f.clock = moment
}

// sprintGoalFor es el Sprint Goal con el que la fixture crea cada sprint, distinto por nombre.
func sprintGoalFor(name string) string {
	return "Objetivo de " + name
}

// createSprint crea con la API de US1 un sprint Planificado con el período indicado y el Sprint Goal
// de sprintGoalFor, y devuelve su identificador.
func (f sprintUS6Fixture) createSprint(t *testing.T, name, start, end string) uuid.UUID {
	t.Helper()
	created := requireCreatedSprint(t, f.env.postSprint(t, f.brunoToken, f.projectID, sprintBody(name, sprintGoalFor(name), start, end)))
	return uuid.MustParse(created.ID)
}

// newStories crea con la 003 una historia lista por cada valor de Story Points, con el prefijo de
// título recibido seguido de su posición, y devuelve sus identificadores en el mismo orden.
func (f sprintUS6Fixture) newStories(t *testing.T, prefix string, points ...int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		ids = append(ids, f.env.createStory(t, f.brunoToken, f.projectID, fmt.Sprintf("%s %d", prefix, i+1), sprintPoints(p), sprintCriterion))
	}
	return ids
}

// startWith compromete las historias en el sprint con la API de US2 y lo inicia con la de US3 en el
// instante indicado.
func (f sprintUS6Fixture) startWith(t *testing.T, sprintID uuid.UUID, startedAt time.Time, storyIDs ...uuid.UUID) {
	t.Helper()
	if len(storyIDs) > 0 {
		rec := f.env.assignStories(t, f.brunoToken, sprintID, storyIDs...)
		require.Equal(t, http.StatusOK, rec.Code, "no se pudieron asignar las historias: %s", rec.Body.String())
	}
	f.setClock(startedAt)
	rec := f.env.startSprint(t, f.brunoToken, sprintID)
	require.Equal(t, http.StatusOK, rec.Code, "no se pudo iniciar el sprint: %s", rec.Body.String())
}

// completeAndClose lleva a Completada, con la API de US4, cada historia indicada del sprint Activo y
// después lo cierra con la de US5 en el instante indicado.
func (f sprintUS6Fixture) completeAndClose(t *testing.T, sprintID uuid.UUID, closedAt time.Time, completed ...uuid.UUID) {
	t.Helper()
	for _, storyID := range completed {
		for _, target := range []string{"in_progress", "completed"} {
			rec := f.env.changeStoryStatus(t, f.brunoToken, storyID, target)
			require.Equal(t, http.StatusOK, rec.Code, "no se pudo pasar la historia a %s: %s", target, rec.Body.String())
		}
	}
	f.setClock(closedAt)
	rec := f.env.closeSprint(t, f.brunoToken, sprintID)
	require.Equal(t, http.StatusOK, rec.Code, "no se pudo cerrar el sprint: %s", rec.Body.String())
}

// sprintClosedRun describe un sprint que la fixture crea, inicia y cierra: su período previsto, sus
// momentos reales de inicio y cierre, los Story Points de sus historias y cuántas de ellas, las
// primeras, quedan completadas.
type sprintClosedRun struct {
	name      string
	start     string
	end       string
	startedAt time.Time
	closedAt  time.Time
	points    []int
	completed int
}

// runClosed crea el sprint descrito con historias nuevas, lo inicia, completa las primeras historias
// y lo cierra, todo con la API, y devuelve su identificador.
func (f sprintUS6Fixture) runClosed(t *testing.T, run sprintClosedRun) uuid.UUID {
	t.Helper()
	sprintID := f.createSprint(t, run.name, run.start, run.end)
	stories := f.newStories(t, "Historia de "+run.name, run.points...)
	f.startWith(t, sprintID, run.startedAt, stories...)
	f.completeAndClose(t, sprintID, run.closedAt, stories[:run.completed]...)
	return sprintID
}

// activeDetailAt siembra un sprint Activo del 2026-10-01 al 2026-10-14, fija el reloj en el instante
// indicado y devuelve el detalle que consulta Bruno.
func (f sprintUS6Fixture) activeDetailAt(t *testing.T, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	sprintID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 1", "2026-10-01", "2026-10-14", domain.SprintStatusActive)
	f.setClock(now)
	return f.env.getSprintDetail(t, f.brunoToken, sprintID)
}

// requireListedSprint exige que el ítem del listado tenga el identificador, el nombre, el Sprint Goal,
// el estado y las fechas previstas indicados.
func requireListedSprint(t *testing.T, item sprintListItemBody, id uuid.UUID, name, status, start, end string) {
	t.Helper()
	assert.Equal(t, id.String(), item.ID, "id de %s", name)
	assert.Equal(t, name, item.Name)
	assert.Equal(t, sprintGoalFor(name), item.Goal, "goal de %s", name)
	assert.Equal(t, status, item.Status, "status de %s", name)
	assert.Equal(t, start, item.StartDate, "startDate de %s", name)
	assert.Equal(t, end, item.EndDate, "endDate de %s", name)
}

// Escenario: Historial de cerrados (004/US6)
// En un proyecto con tres sprints cerrados con resultados distintos, el historial (?status=closed)
// devuelve los tres, del más reciente al más antiguo, cada uno con su nombre, su Sprint Goal, sus
// cuatro fechas, la cantidad de historias y los cinco totales congelados de su instantánea (FR-051).
func TestUS6_HistorialDeCerrados(t *testing.T) {
	f := newSprintUS6Fixture(t)
	runs := []sprintClosedRun{
		{name: "Sprint 1", start: "2026-09-01", end: "2026-09-14", startedAt: sprintMoment(t, "2026-09-01", 12, 0), closedAt: sprintMoment(t, "2026-09-14", 20, 0), points: []int{8, 5, 3}, completed: 2},
		{name: "Sprint 2", start: "2026-09-15", end: "2026-09-28", startedAt: sprintMoment(t, "2026-09-15", 12, 0), closedAt: sprintMoment(t, "2026-09-28", 20, 0), points: []int{5, 5}, completed: 1},
		{name: "Sprint 3", start: "2026-10-01", end: "2026-10-14", startedAt: sprintMoment(t, "2026-10-01", 12, 0), closedAt: sprintMoment(t, "2026-10-14", 20, 0), points: []int{3, 2, 1}, completed: 3},
	}
	ids := make([]uuid.UUID, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, f.runClosed(t, run))
	}
	want := []sprintResultsBody{
		{CommittedStoryPoints: 16, PlannedStoryPoints: 16, CompletedStoryPoints: 13, PlannedStoryCount: 3, CompletedStoryCount: 2},
		{CommittedStoryPoints: 10, PlannedStoryPoints: 10, CompletedStoryPoints: 5, PlannedStoryCount: 2, CompletedStoryCount: 1},
		{CommittedStoryPoints: 6, PlannedStoryPoints: 6, CompletedStoryPoints: 6, PlannedStoryCount: 3, CompletedStoryCount: 3},
	}

	rec := f.env.listSprints(t, f.brunoToken, f.projectID, "closed")

	body, raw := requireSprintList(t, rec)
	assert.Nil(t, raw["emptyReason"], "con resultados emptyReason es null")
	assert.Equal(t, []string{"closed"}, body.AppliedFilters.Status)
	require.Len(t, body.Sprints, 3, "el historial debe listar los tres sprints cerrados: %s", rec.Body.String())

	// El listado va en orden start_date DESC: la posición 0 es el último sprint de runs.
	for pos, item := range body.Sprints {
		i := len(runs) - 1 - pos
		run := runs[i]
		requireListedSprint(t, item, ids[i], run.name, "closed", run.start, run.end)
		if assert.NotNil(t, item.StartedAt, "startedAt de %s", run.name) {
			assert.Equal(t, sprintUTC(run.startedAt), *item.StartedAt)
		}
		if assert.NotNil(t, item.ClosedAt, "closedAt de %s", run.name) {
			assert.Equal(t, sprintUTC(run.closedAt), *item.ClosedAt)
		}
		assert.Equal(t, len(run.points), item.StoryCount, "storyCount de %s", run.name)
		if assert.NotNil(t, item.Results, "results de %s", run.name) {
			assert.Equal(t, want[i], *item.Results, "totales de %s", run.name)
		}
	}
}

// Escenario: Listado completo y filtro (004/US6)
// En un proyecto con un sprint Cerrado, uno Activo y uno Planificado, el listado sin filtro devuelve
// los tres con su estado en orden start_date DESC, con results solo en el Cerrado; con
// ?status=closed devuelve solo el Cerrado, y con ?status=planned&status=active los otros dos
// (FR-050 a FR-052).
func TestUS6_ListadoCompletoYFiltro(t *testing.T) {
	f := newSprintUS6Fixture(t)
	closedID := f.runClosed(t, sprintClosedRun{
		name: "Sprint 1", start: "2026-09-15", end: "2026-09-28",
		startedAt: sprintMoment(t, "2026-09-15", 12, 0), closedAt: sprintMoment(t, "2026-09-28", 20, 0),
		points: []int{5, 3}, completed: 1,
	})
	activeID := f.createSprint(t, "Sprint 2", "2026-10-15", "2026-10-28")
	f.startWith(t, activeID, sprintMoment(t, "2026-10-15", 12, 0), f.newStories(t, "Historia activa", 8, 2)...)
	plannedID := f.createSprint(t, "Sprint 3", "2026-11-01", "2026-11-14")

	t.Run("sin filtro", func(t *testing.T) {
		body, raw := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID))
		assert.Nil(t, raw["emptyReason"], "con resultados emptyReason es null")
		assert.Equal(t, []string{}, body.AppliedFilters.Status, "sin filtro appliedFilters.status es []")
		require.Equal(t, []string{plannedID.String(), activeID.String(), closedID.String()}, sprintListIDs(body),
			"el orden debe ser start_date DESC, id")
		requireListedSprint(t, body.Sprints[0], plannedID, "Sprint 3", "planned", "2026-11-01", "2026-11-14")
		assert.Nil(t, body.Sprints[0].StartedAt, "un sprint planificado no tiene inicio real")
		assert.Nil(t, body.Sprints[0].Results, "results es null en un sprint planificado")
		assert.Zero(t, body.Sprints[0].StoryCount)
		requireListedSprint(t, body.Sprints[1], activeID, "Sprint 2", "active", "2026-10-15", "2026-10-28")
		assert.Nil(t, body.Sprints[1].ClosedAt, "un sprint activo no tiene cierre")
		assert.Nil(t, body.Sprints[1].Results, "results es null en un sprint activo")
		assert.Equal(t, 2, body.Sprints[1].StoryCount)
		requireListedSprint(t, body.Sprints[2], closedID, "Sprint 1", "closed", "2026-09-15", "2026-09-28")
		assert.NotNil(t, body.Sprints[2].Results, "results tiene valor en un sprint cerrado")
	})

	t.Run("solo cerrados", func(t *testing.T) {
		body, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, "closed"))
		assert.Equal(t, []string{"closed"}, body.AppliedFilters.Status)
		assert.Equal(t, []string{closedID.String()}, sprintListIDs(body), "el historial muestra solo el cerrado")
	})

	t.Run("planificados y activos", func(t *testing.T) {
		body, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, "planned", "active"))
		assert.ElementsMatch(t, []string{"planned", "active"}, body.AppliedFilters.Status)
		assert.Equal(t, []string{plannedID.String(), activeID.String()}, sprintListIDs(body))
	})
}

// Escenario: Historial de una historia (004/US6)
// Una historia que estuvo planificada en tres sprints hasta completarse (no completada en los dos
// primeros, completada en el tercero) informa los tres sprints en orden ascendente de inicio, con
// not_completed, not_completed y completed, y completedAt solo en el tercero (FR-055, RN-17).
func TestUS6_HistorialDeUnaHistoria(t *testing.T) {
	f := newSprintUS6Fixture(t)
	storyID := f.newStories(t, "Historia viajera", 5)[0]
	type stage struct {
		name, start, end string
		complete         bool
	}
	stages := []stage{
		{name: "Sprint 1", start: "2026-09-01", end: "2026-09-14"},
		{name: "Sprint 2", start: "2026-09-15", end: "2026-09-28"},
		{name: "Sprint 3", start: "2026-10-01", end: "2026-10-14", complete: true},
	}

	// La misma historia pasa por los tres sprints: se compromete, se inicia el sprint y se cierra,
	// completándola solo en el último.
	ids := make([]uuid.UUID, 0, len(stages))
	for _, s := range stages {
		sprintID := f.createSprint(t, s.name, s.start, s.end)
		f.startWith(t, sprintID, sprintMoment(t, s.start, 12, 0), storyID)
		var completed []uuid.UUID
		if s.complete {
			completed = append(completed, storyID)
		}
		f.completeAndClose(t, sprintID, sprintMoment(t, s.end, 20, 0), completed...)
		ids = append(ids, sprintID)
	}

	body := requireStoryHistory(t, f.env.listStorySprints(t, f.brunoToken, storyID))

	require.Len(t, body.Sprints, 3, "la historia estuvo en tres sprints")
	wantOutcomes := []string{"not_completed", "not_completed", "completed"}
	for i, item := range body.Sprints {
		assert.Equal(t, ids[i].String(), item.SprintID, "orden ascendente de inicio")
		assert.Equal(t, stages[i].name, item.SprintName)
		assert.Equal(t, "closed", item.SprintStatus)
		assert.Equal(t, stages[i].start, item.StartDate)
		assert.Equal(t, stages[i].end, item.EndDate)
		if assert.NotNil(t, item.Outcome, "outcome en %s", stages[i].name) {
			assert.Equal(t, wantOutcomes[i], *item.Outcome, "outcome en %s", stages[i].name)
		}
		if stages[i].complete {
			assert.NotNil(t, item.CompletedAt, "completedAt en el sprint donde se completó")
		} else {
			assert.Nil(t, item.CompletedAt, "completedAt es null donde no se completó")
		}
	}
}

// Escenario: Días restantes (004/US6)
// El detalle de un sprint Activo que termina el 2026-10-14, consultado el 2026-10-04, informa 10 días
// restantes, con el día de fin incluido, y overdueDays null (FR-053, RN-23).
func TestUS6_DiasRestantes(t *testing.T) {
	f := newSprintUS6Fixture(t)

	schedule := requireSchedule(t, f.activeDetailAt(t, sprintMoment(t, "2026-10-04", 12, 0)))

	if assert.NotNil(t, schedule.RemainingDays, "remainingDays debe tener valor") {
		assert.Equal(t, 10, *schedule.RemainingDays)
	}
	assert.Nil(t, schedule.OverdueDays, "overdueDays es null si el sprint no venció")
}

// Escenario: Termina hoy (004/US6)
// El detalle de un sprint Activo que termina el día de la consulta informa 0 días restantes, porque
// el día de fin se cuenta y el sprint sigue en curso, y overdueDays null (RN-23).
func TestUS6_TerminaHoy(t *testing.T) {
	f := newSprintUS6Fixture(t)

	schedule := requireSchedule(t, f.activeDetailAt(t, sprintMoment(t, "2026-10-14", 12, 0)))

	if assert.NotNil(t, schedule.RemainingDays, "remainingDays debe tener valor el último día") {
		assert.Equal(t, 0, *schedule.RemainingDays)
	}
	assert.Nil(t, schedule.OverdueDays, "el último día no está vencido")
}

// Escenario: Vencido (004/US6)
// El detalle de un sprint Activo cuya fecha de fin (2026-10-14) pasó hace 3 días informa overdueDays
// 3 y remainingDays null, nunca un número negativo (RN-23).
func TestUS6_Vencido(t *testing.T) {
	f := newSprintUS6Fixture(t)

	schedule := requireSchedule(t, f.activeDetailAt(t, sprintMoment(t, "2026-10-17", 12, 0)))

	assert.Nil(t, schedule.RemainingDays, "remainingDays es null en un sprint vencido")
	if assert.NotNil(t, schedule.OverdueDays, "overdueDays debe tener valor") {
		assert.Equal(t, 3, *schedule.OverdueDays)
	}
}

// Escenario: Sin días restantes fuera de Activo (004/US6)
// Los detalles de un sprint Planificado y de uno Cerrado traen schedule en null, porque los días
// restantes solo aplican al sprint en curso (FR-053).
func TestUS6_SinDiasRestantesFueraDeActivo(t *testing.T) {
	f := newSprintUS6Fixture(t)
	closedID := f.runClosed(t, sprintClosedRun{
		name: "Sprint 1", start: "2026-10-01", end: "2026-10-14",
		startedAt: sprintMoment(t, "2026-10-01", 12, 0), closedAt: sprintMoment(t, "2026-10-14", 20, 0),
		points: []int{5}, completed: 1,
	})
	plannedID := f.createSprint(t, "Sprint 2", "2026-10-15", "2026-10-28")
	f.setClock(sprintMoment(t, "2026-10-20", 12, 0))

	for name, sprintID := range map[string]uuid.UUID{"planificado": plannedID, "cerrado": closedID} {
		t.Run(name, func(t *testing.T) {
			rec := f.env.getSprintDetail(t, f.brunoToken, sprintID)
			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			schedule, present := sprintRawBody(t, rec)["schedule"]
			assert.True(t, present, "falta el campo schedule en el detalle")
			assert.Nil(t, schedule, "schedule debe ser null en un sprint %s", name)
		})
	}
}

// Escenario: Detalle desde la instantánea (004/US6)
// Cerrado un sprint con una historia no completada ("Historia de Sprint 1 1", prioridad alta, 5 Story
// Points), renombrarla, cambiarle la prioridad y reestimarla con la 003 no cambia el detalle del
// sprint: muestra el título, la prioridad y los Story Points que tenía al cerrarse (FR-043, FR-053).
func TestUS6_DetalleDesdeLaInstantanea(t *testing.T) {
	f := newSprintUS6Fixture(t)
	sprintID := f.createSprint(t, "Sprint 1", "2026-10-01", "2026-10-14")
	storyID := f.newStories(t, "Historia de Sprint 1", 5)[0]
	f.startWith(t, sprintID, sprintMoment(t, "2026-10-01", 12, 0), storyID)
	f.completeAndClose(t, sprintID, sprintMoment(t, "2026-10-14", 20, 0))

	// Renombre, cambio de prioridad y reestimación de la historia devuelta al backlog, con la 003; se
	// comprueba que los cambios ocurrieron de verdad.
	patched := f.env.sprintDo(t, http.MethodPatch, "/api/v1/stories/"+storyID.String(), f.brunoToken, map[string]any{"title": "Historia renombrada", "priority": "low"})
	require.Equal(t, http.StatusOK, patched.Code, "no se pudo modificar la historia: %s", patched.Body.String())
	reestimated := f.env.sprintDo(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", f.brunoToken, map[string]any{"storyPoints": 13})
	require.Equal(t, http.StatusOK, reestimated.Code, "no se pudo reestimar la historia: %s", reestimated.Body.String())
	story := sprintStoredStory(t, f.env.db, storyID)
	require.Equal(t, "Historia renombrada", story.Title)
	require.Equal(t, backlogdomain.StoryPriorityLow, story.Priority)
	require.Equal(t, sprintPoints(13), story.StoryPoints)

	rec := f.env.getSprintDetail(t, f.brunoToken, sprintID)

	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body sprintClosedBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	require.Len(t, body.Stories, 1, "el detalle debe listar la historia de la instantánea")
	assert.Equal(t, storyID.String(), body.Stories[0].StoryID)
	assert.Equal(t, "Historia de Sprint 1 1", body.Stories[0].Title, "título al cierre")
	assert.Equal(t, "high", body.Stories[0].Priority, "prioridad al cierre")
	assert.Equal(t, 5, body.Stories[0].StoryPoints, "Story Points al cierre")
}

// Escenario: Cuatro fechas (004/US6)
// Un sprint previsto del 2026-10-01 al 2026-10-14 que arrancó el 2026-10-03 y se cerró el 2026-10-10
// figura en el historial con las cuatro fechas por separado, de modo que la duración real es
// distinguible de la prevista (FR-051).
func TestUS6_CuatroFechas(t *testing.T) {
	f := newSprintUS6Fixture(t)
	startedAt := sprintMoment(t, "2026-10-03", 12, 0)
	closedAt := sprintMoment(t, "2026-10-10", 20, 30)
	sprintID := f.runClosed(t, sprintClosedRun{
		name: "Sprint 1", start: "2026-10-01", end: "2026-10-14",
		startedAt: startedAt, closedAt: closedAt, points: []int{5}, completed: 0,
	})

	body, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, "closed"))

	require.Len(t, body.Sprints, 1, "el historial debe listar el sprint cerrado")
	item := body.Sprints[0]
	requireListedSprint(t, item, sprintID, "Sprint 1", "closed", "2026-10-01", "2026-10-14")
	if assert.NotNil(t, item.StartedAt, "startedAt debe registrar el inicio real") {
		assert.Equal(t, sprintUTC(startedAt), *item.StartedAt)
	}
	if assert.NotNil(t, item.ClosedAt, "closedAt debe registrar el cierre real") {
		assert.Equal(t, sprintUTC(closedAt), *item.ClosedAt)
	}
}

// Escenario: Proyecto sin sprints (004/US6)
// El listado de un proyecto sin ningún sprint responde 200 con sprints [] y emptyReason no_sprints,
// también cuando se filtra por estado, sin que eso sea un error (FR-054).
func TestUS6_ProyectoSinSprints(t *testing.T) {
	f := newSprintUS6Fixture(t)

	for name, statuses := range map[string][]string{"sin filtro": nil, "con filtro": {"closed"}} {
		t.Run(name, func(t *testing.T) {
			body, raw := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, statuses...))
			assert.Equal(t, []any{}, raw["sprints"], "sprints debe ser []")
			if assert.NotNil(t, body.EmptyReason, "emptyReason debe explicar el vacío") {
				assert.Equal(t, "no_sprints", *body.EmptyReason)
			}
		})
	}
}

// Escenario: Cerrados sin completadas (004/US6)
// Un proyecto cuyos dos sprints cerrados no completaron ninguna historia los lista en el historial
// con 0 Story Points y 0 historias completadas, sin que eso sea un error (FR-045, FR-051).
func TestUS6_CerradosSinCompletadas(t *testing.T) {
	f := newSprintUS6Fixture(t)
	first := f.runClosed(t, sprintClosedRun{
		name: "Sprint 1", start: "2026-09-15", end: "2026-09-28",
		startedAt: sprintMoment(t, "2026-09-15", 12, 0), closedAt: sprintMoment(t, "2026-09-28", 20, 0),
		points: []int{5, 3}, completed: 0,
	})
	second := f.runClosed(t, sprintClosedRun{
		name: "Sprint 2", start: "2026-10-01", end: "2026-10-14",
		startedAt: sprintMoment(t, "2026-10-01", 12, 0), closedAt: sprintMoment(t, "2026-10-14", 20, 0),
		points: []int{8}, completed: 0,
	})

	body, raw := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, "closed"))

	assert.Nil(t, raw["emptyReason"], "con resultados emptyReason es null")
	require.Equal(t, []string{second.String(), first.String()}, sprintListIDs(body))
	want := []sprintResultsBody{
		{CommittedStoryPoints: 8, PlannedStoryPoints: 8, PlannedStoryCount: 1},
		{CommittedStoryPoints: 8, PlannedStoryPoints: 8, PlannedStoryCount: 2},
	}
	for i, item := range body.Sprints {
		if assert.NotNil(t, item.Results, "results de %s", item.Name) {
			assert.Equal(t, want[i], *item.Results, "totales de %s", item.Name)
		}
	}
}

// Escenario: No integrante (004/US6)
// Un usuario que no es integrante del proyecto recibe al listar sus sprints, con y sin filtro, el
// mismo 404 que ante un proyecto inexistente, idéntico byte a byte y sin datos del proyecto (FR-058).
func TestUS6_NoIntegrante(t *testing.T) {
	f := newSprintUS6Fixture(t)
	sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	_, outsiderToken := f.env.signUp(t, "Carla Gómez", "carla@example.com")

	for name, statuses := range map[string][]string{"sin filtro": nil, "con filtro": {"closed"}} {
		t.Run(name, func(t *testing.T) {
			foreignRec := f.env.listSprints(t, outsiderToken, f.projectID, statuses...)
			missingRec := f.env.listSprints(t, outsiderToken, uuid.New(), statuses...)

			assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
			assert.JSONEq(t, `{"error":"`+sprintProjectNotFound+`"}`, foreignRec.Body.String())
			assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
			assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
			assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
		})
	}
}

// Escenario: Sprint inexistente (004/US6)
// Un integrante que consulta el detalle de un sprint que no existe recibe el 404 de sprint
// inexistente del contrato (FR-058).
func TestUS6_SprintInexistente(t *testing.T) {
	f := newSprintUS6Fixture(t)

	rec := f.env.getSprintDetail(t, f.brunoToken, uuid.New())

	assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, sprintNotFoundBody, rec.Body.String())
}

// Sin etiqueta de escenario: el listado no incluye sprints de otro proyecto. Bruno integra dos
// proyectos con un sprint cada uno y el listado de cada proyecto devuelve solo el suyo (FR-050).
func TestSprintList_ExcludesOtherProjects(t *testing.T) {
	f := newSprintUS6Fixture(t)
	ownID := f.createSprint(t, "Sprint propio", "2026-10-01", "2026-10-14")
	otherProjectID := sprintSeedProject(t, f.env.db, f.anaID, projectsdomain.ProjectStatusInProgress, f.brunoID)
	otherID := sprintSeed(t, f.env.db, otherProjectID, f.anaID, "Sprint ajeno", "2026-10-01", "2026-10-14")

	own, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID))
	other, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, otherProjectID))

	assert.Equal(t, []string{ownID.String()}, sprintListIDs(own), "el listado solo trae los sprints del proyecto")
	assert.Equal(t, []string{otherID.String()}, sprintListIDs(other), "el listado solo trae los sprints del proyecto")
}

// Sin etiqueta de escenario: una historia quitada de un sprint Activo antes de su cierre no figura en
// el historial de ese sprint. El historial de la historia quitada viene vacío, el de la que se quedó
// muestra el sprint, y el historial de cerrados cuenta solo la historia que quedó (FR-055, RN-17).
func TestStorySprints_RemovedBeforeCloseNotListed(t *testing.T) {
	f := newSprintUS6Fixture(t)
	sprintID := f.createSprint(t, "Sprint 1", "2026-10-01", "2026-10-14")
	stories := f.newStories(t, "Historia", 5, 3)
	f.startWith(t, sprintID, sprintMoment(t, "2026-10-01", 12, 0), stories...)
	removed := f.env.removeStories(t, f.brunoToken, sprintID, stories[0])
	require.Equal(t, http.StatusOK, removed.Code, "no se pudo quitar la historia: %s", removed.Body.String())
	f.completeAndClose(t, sprintID, sprintMoment(t, "2026-10-14", 20, 0))

	removedHistory := requireStoryHistory(t, f.env.listStorySprints(t, f.brunoToken, stories[0]))
	keptHistory := requireStoryHistory(t, f.env.listStorySprints(t, f.brunoToken, stories[1]))
	list, _ := requireSprintList(t, f.env.listSprints(t, f.brunoToken, f.projectID, "closed"))

	assert.Empty(t, removedHistory.Sprints, "la historia quitada no figura en el sprint")
	if assert.Len(t, keptHistory.Sprints, 1, "la historia que quedó figura en el sprint") {
		assert.Equal(t, sprintID.String(), keptHistory.Sprints[0].SprintID)
	}
	if assert.Len(t, list.Sprints, 1) {
		assert.Equal(t, 1, list.Sprints[0].StoryCount, "el sprint cuenta solo la historia que quedó")
	}
}

// Sin etiqueta de escenario: un listado con miles de status=closed responde 200 con solo el sprint
// cerrado y el filtro aplicado una sola vez, porque la cláusula IN debe recibir cada estado admitido
// una vez y no un parámetro por repetición (T101, hallazgo de seguridad: filtro de estado repetible
// sin deduplicar). Las repeticiones quedan por debajo del tope de 10000 parámetros de consulta de
// net/url: por encima de ese tope la consulta no se interpreta y el manejador no recibe ningún valor
// de status, de modo que la petición nunca llegaría a la cláusula IN.
func TestSprintList_EstadoRepetidoMilesDeVeces(t *testing.T) {
	const repetitions = 9000
	f := newSprintUS6Fixture(t)
	closedID := f.runClosed(t, sprintClosedRun{
		name: "Sprint 1", start: "2026-09-15", end: "2026-09-28",
		startedAt: sprintMoment(t, "2026-09-15", 12, 0), closedAt: sprintMoment(t, "2026-09-28", 20, 0),
		points: []int{5, 3}, completed: 1,
	})
	f.createSprint(t, "Sprint 2", "2026-11-01", "2026-11-14")
	statuses := make([]string, repetitions)
	for i := range statuses {
		statuses[i] = "closed"
	}

	rec := f.env.listSprints(t, f.brunoToken, f.projectID, statuses...)

	// Solo se lista el cerrado y el filtro informado contiene closed una única vez.
	body, _ := requireSprintList(t, rec)
	assert.Equal(t, []string{"closed"}, body.AppliedFilters.Status, "el filtro aplicado debe informar closed una sola vez")
	assert.Equal(t, []string{closedID.String()}, sprintListIDs(body), "el listado muestra solo el cerrado")
}

// Textos exactos que fija el contrato para los rechazos de la modificación y la eliminación de un
// sprint (US7): el 409 de modificar un sprint que no está Planificado, el de eliminar uno ya iniciado
// y el de eliminar uno con esfuerzo registrado. El de sprint cerrado es sprintClosedMessage. Los dos
// últimos son el 409 de la 003 al eliminar una historia que estuvo en un sprint.
const (
	sprintUpdateNotPlannedMessage = "Solo se puede modificar un sprint Planificado."
	sprintDeleteStartedMessage    = "Solo se puede eliminar un sprint que nunca fue iniciado."
	sprintDeleteEffortMessage     = "El sprint tiene esfuerzo registrado. Primero hay que eliminar esos registros."
	sprintStoryDeletionBlocked    = "La historia tiene historial asociado y no se puede eliminar."
	sprintStoryHistoryReason      = "Estuvo asignada a un sprint."
)

// sprintEffortDouble es el doble de testify del puerto SprintEffort: cada prueba configura qué
// informa sobre el esfuerzo registrado del sprint, porque la feature de esfuerzo todavía no existe
// (plan.md, cómo se preparan los escenarios).
type sprintEffortDouble struct {
	mock.Mock
}

// HasEffort devuelve lo que la prueba configuró para el esfuerzo del sprint.
func (m *sprintEffortDouble) HasEffort(ctx context.Context, sprintID uuid.UUID) (bool, error) {
	args := m.Called(ctx, sprintID)
	return args.Bool(0), args.Error(1)
}

// updateSprint envía el cuerpo a PUT /api/v1/sprints/{sprintId} con el token como Bearer.
func (e sprintTestEnv) updateSprint(t *testing.T, token string, sprintID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodPut, sprintPath(sprintID), token, body)
}

// deleteSprint envía DELETE /api/v1/sprints/{sprintId}, sin cuerpo, con el token como Bearer.
func (e sprintTestEnv) deleteSprint(t *testing.T, token string, sprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.sprintDo(t, http.MethodDelete, sprintPath(sprintID), token, nil)
}

// requireUpdatedSprint exige un 200 cuyo cuerpo sea el detalle del sprint con los datos del cuerpo
// enviado y el estado indicado, y lo devuelve decodificado con sus historias.
func requireUpdatedSprint(t *testing.T, rec *httptest.ResponseRecorder, sprintID uuid.UUID, body map[string]any, status string) sprintStoriesDetailBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var detail sprintDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, sprintID.String(), detail.ID)
	assert.Equal(t, body["name"], detail.Name)
	assert.Equal(t, body["goal"], detail.Goal)
	assert.Equal(t, body["startDate"], detail.StartDate)
	assert.Equal(t, body["endDate"], detail.EndDate)
	assert.Equal(t, status, detail.Status)
	return requireSprintStories(t, rec)
}

// requireStoredSprintData exige que la fila del sprint guarde el nombre, el Sprint Goal y las fechas
// del cuerpo enviado, y la devuelve.
func requireStoredSprintData(t *testing.T, db *gorm.DB, sprintID uuid.UUID, body map[string]any) domain.Sprint {
	t.Helper()
	stored := sprintStored(t, db, sprintID.String())
	assert.Equal(t, body["name"], stored.Name)
	assert.Equal(t, body["goal"], stored.Goal)
	assert.Equal(t, sprintDate(t, body["startDate"].(string)), stored.StartDate.UTC())
	assert.Equal(t, sprintDate(t, body["endDate"].(string)), stored.EndDate.UTC())
	return stored
}

// requireSprintDataUntouched exige que la fila del sprint conserve el nombre, el Sprint Goal, las
// fechas y el momento de última actualización que tenía antes del rechazo, además del estado y los
// datos de inicio que verifica requireStoredUnchanged: ningún cambio parcial (FR-014).
func requireSprintDataUntouched(t *testing.T, db *gorm.DB, before domain.Sprint) {
	t.Helper()
	requireStoredUnchanged(t, db, before)
	after := sprintStored(t, db, before.ID.String())
	assert.Equal(t, before.Name, after.Name, "el nombre no debe cambiar")
	assert.Equal(t, before.Goal, after.Goal, "el Sprint Goal no debe cambiar")
	assert.True(t, before.StartDate.Equal(after.StartDate), "start_date = %v, se esperaba %v", after.StartDate, before.StartDate)
	assert.True(t, before.EndDate.Equal(after.EndDate), "end_date = %v, se esperaba %v", after.EndDate, before.EndDate)
	assert.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "la fila del sprint no debe escribirse: updated_at = %v, antes %v", after.UpdatedAt, before.UpdatedAt)
}

// requireSprintDeleted exige un 204 sin cuerpo y que no quede la fila del sprint en sprints ni
// ninguna suya en sprint_stories.
func requireSprintDeleted(t *testing.T, db *gorm.DB, rec *httptest.ResponseRecorder, sprintID uuid.UUID) {
	t.Helper()
	require.Equal(t, http.StatusNoContent, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Empty(t, rec.Body.String(), "el 204 no lleva cuerpo")
	var n int64
	require.NoError(t, db.Model(&domain.Sprint{}).Where("id = ?", sprintID).Count(&n).Error, "no se pudo contar el sprint")
	assert.Zero(t, n, "no debe quedar la fila del sprint en sprints")
	assert.Zero(t, sprintCommitmentCount(t, db, "sprint_id = ?", sprintID), "no deben quedar filas del sprint en sprint_stories")
}

// commitStories crea con la 003, sobre el "Dado" de US2 que también usa US7, una historia lista por
// título y la compromete en el sprint indicado, como lo dejaría una asignación, y devuelve sus
// identificadores.
func (f sprintUS2Fixture) commitStories(t *testing.T, sprintID uuid.UUID, titles ...string) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, 0, len(titles))
	for _, title := range titles {
		id := f.env.createReadyStory(t, f.brunoToken, f.projectID, title)
		sprintSeedCommitment(t, f.env.db, sprintID, id, sprintFixedNow.Add(-time.Hour), nil, nil)
		ids = append(ids, id)
	}
	return ids
}

// requireCommittedOpen exige que cada historia siga comprometida en el sprint, sin resultado, y en
// pending: el sprint no cambió sus historias.
func requireCommittedOpen(t *testing.T, db *gorm.DB, sprintID uuid.UUID, storyIDs ...uuid.UUID) {
	t.Helper()
	assert.EqualValues(t, len(storyIDs), sprintCommitmentCount(t, db, "sprint_id = ? AND outcome IS NULL", sprintID))
	for _, id := range storyIDs {
		assert.EqualValues(t, 1, sprintCommitmentCount(t, db, "sprint_id = ? AND story_id = ? AND outcome IS NULL", sprintID, id),
			"la historia %s debe seguir comprometida", id)
		assert.Equal(t, backlogdomain.StoryStatusPending, sprintStoredStory(t, db, id).Status)
	}
}

// Escenario: Modificación completa (004/US7)
// Bruno, integrante que no es el propietario, cambia nombre, fechas y Sprint Goal de un sprint
// Planificado con dos historias: responde 200 con el detalle actualizado, el sprint sigue planned
// sin datos de inicio, conserva su autor y sus dos historias siguen comprometidas (FR-012, FR-015).
func TestUS7_ModificacionCompleta(t *testing.T) {
	f := newSprintUS2Fixture(t)
	stories := f.commitStories(t, f.sprintOneID, "Exportar el reporte", "Filtrar por fecha")
	body := sprintBody("Sprint 1 corregido", "Publicar el alta de clientes", "2026-10-02", "2026-10-16")

	rec := f.env.updateSprint(t, f.brunoToken, f.sprintOneID, body)

	// La respuesta es el detalle con los datos nuevos, el mismo estado y las mismas historias.
	detail := requireUpdatedSprint(t, rec, f.sprintOneID, body, "planned")
	assert.ElementsMatch(t, []string{stories[0].String(), stories[1].String()}, sprintDetailStoryIDs(detail))

	// La fila guarda los datos nuevos y no toca el estado, el inicio ni el autor.
	stored := requireStoredSprintData(t, f.env.db, f.sprintOneID, body)
	assert.Equal(t, domain.SprintStatusPlanned, stored.Status)
	assert.Nil(t, stored.StartedAt)
	assert.Nil(t, stored.CommittedStoryPoints)
	assert.Equal(t, f.anaID, stored.CreatedByID, "el autor del sprint no cambia")
	requireCommittedOpen(t, f.env.db, f.sprintOneID, stories...)
}

// Escenario: Eliminación con historias (004/US7)
// Eliminar un sprint Planificado que nunca fue iniciado y tiene dos historias responde 204: no queda
// fila en sprints ni en sprint_stories, las dos historias quedan pending y sin sprint (conservando
// ever_committed), y un sprint nuevo en el mismo período se acepta (FR-048, RC-06).
func TestUS7_EliminacionConHistorias(t *testing.T) {
	f := newSprintUS2Fixture(t)
	stories := f.commitStories(t, f.sprintOneID, "Exportar el reporte", "Filtrar por fecha")

	rec := f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)

	// El sprint y sus compromisos desaparecen y las historias vuelven al backlog.
	requireSprintDeleted(t, f.env.db, rec, f.sprintOneID)
	for _, id := range stories {
		story := sprintStoredStory(t, f.env.db, id)
		assert.Equal(t, backlogdomain.StoryStatusPending, story.Status, "la historia %s debe volver a pending", id)
		assert.True(t, story.EverCommitted, "la historia %s conserva ever_committed", id)
		assert.Zero(t, sprintCommitmentCount(t, f.env.db, "story_id = ?", id), "la historia %s queda sin sprint", id)
	}

	// El período quedó libre para otro sprint.
	created := requireCreatedSprint(t, f.env.postSprint(t, f.brunoToken, f.projectID, sprintBody("Sprint 1 bis", sprintGoal, "2026-10-01", "2026-10-14")))
	assert.Equal(t, "2026-10-01", created.StartDate)
}

// Escenario: Mover fechas (004/US7)
// Un sprint Planificado con dos historias se corre a un período libre (del 2026-11-01 al
// 2026-11-14, después de "Sprint 2"): responde 200 con las fechas nuevas y las historias siguen
// comprometidas (FR-012, FR-015).
func TestUS7_MoverFechas(t *testing.T) {
	f := newSprintUS2Fixture(t)
	sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28")
	stories := f.commitStories(t, f.sprintOneID, "Exportar el reporte", "Filtrar por fecha")
	body := sprintBody("Sprint 1", sprintGoal, "2026-11-01", "2026-11-14")

	rec := f.env.updateSprint(t, f.brunoToken, f.sprintOneID, body)

	detail := requireUpdatedSprint(t, rec, f.sprintOneID, body, "planned")
	assert.ElementsMatch(t, []string{stories[0].String(), stories[1].String()}, sprintDetailStoryIDs(detail))
	requireStoredSprintData(t, f.env.db, f.sprintOneID, body)
	requireCommittedOpen(t, f.env.db, f.sprintOneID, stories...)
}

// Escenario: Mismo nombre (004/US7)
// Renombrar un sprint Planificado con el mismo nombre que ya tenía se acepta con 200, aunque el resto
// de los datos tampoco cambie (FR-012).
func TestUS7_MismoNombre(t *testing.T) {
	f := newSprintUS2Fixture(t)
	body := sprintBody("Sprint 1", sprintGoal, "2026-10-01", "2026-10-14")

	rec := f.env.updateSprint(t, f.brunoToken, f.sprintOneID, body)

	requireUpdatedSprint(t, rec, f.sprintOneID, body, "planned")
	requireStoredSprintData(t, f.env.db, f.sprintOneID, body)
}

// Escenario: Fechas contiguas (004/US7)
// Con "Sprint 1" del 2026-10-01 al 2026-10-14, mover "Sprint 2" para que empiece el 2026-10-14 se
// acepta, porque compartir el día de borde no es superposición (FR-013, RN-05).
func TestUS7_FechasContiguas(t *testing.T) {
	f := newSprintUS2Fixture(t)
	sprintTwoID := sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-20", "2026-11-02")
	body := sprintBody("Sprint 2", sprintGoal, "2026-10-14", "2026-10-28")

	rec := f.env.updateSprint(t, f.brunoToken, sprintTwoID, body)

	requireUpdatedSprint(t, rec, sprintTwoID, body, "planned")
	requireStoredSprintData(t, f.env.db, sprintTwoID, body)
}

// Escenario: Eliminación sin historias (004/US7)
// Eliminar un sprint Planificado sin historias responde 204 y no tiene efectos sobre el backlog: una
// historia del proyecto, fuera del sprint, no se escribe (FR-048).
func TestUS7_EliminacionSinHistorias(t *testing.T) {
	f := newSprintUS2Fixture(t)
	storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
	before := sprintStoredStory(t, f.env.db, storyID)

	rec := f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)

	requireSprintDeleted(t, f.env.db, rec, f.sprintOneID)
	after := sprintStoredStory(t, f.env.db, storyID)
	assert.Equal(t, before.Status, after.Status)
	assert.False(t, after.EverCommitted, "la historia nunca estuvo en un sprint")
	assert.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "la historia no debe escribirse: updated_at = %v, antes %v", after.UpdatedAt, before.UpdatedAt)
}

// Escenario: Historia de sprint eliminado (004/US7)
// Una historia que estuvo comprometida en un sprint que después se eliminó no se puede eliminar del
// backlog: el DELETE de la 003 responde 409 con el motivo sprint_history y la historia sigue
// existiendo (RC-06, FR-048).
func TestUS7_HistoriaDeSprintEliminado(t *testing.T) {
	f := newSprintUS2Fixture(t)
	stories := f.commitStories(t, f.sprintOneID, "Exportar el reporte")
	requireSprintDeleted(t, f.env.db, f.env.deleteSprint(t, f.brunoToken, f.sprintOneID), f.sprintOneID)

	rec := f.env.sprintDo(t, http.MethodDelete, "/api/v1/stories/"+stories[0].String(), f.brunoToken, nil)

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintStoryDeletionBlocked+`","reasons":[{"code":"sprint_history","message":"`+sprintStoryHistoryReason+`"}]}`, rec.Body.String())
	sprintStoredStory(t, f.env.db, stories[0])
}

// Escenario: Esfuerzo eliminado antes (004/US7)
// Con el puerto de esfuerzo reemplazado por un doble que primero informa esfuerzo registrado y
// después ya no, la primera eliminación se rechaza con 409 y la segunda, hecha cuando el autor ya
// eliminó su registro, se acepta con 204 (FR-048, RN-20).
func TestUS7_EsfuerzoEliminadoAntes(t *testing.T) {
	effort := &sprintEffortDouble{}
	f := newSprintUS2Fixture(t, func(p *sprintTestPorts) { p.effort = effort })
	effort.On("HasEffort", mock.Anything, f.sprintOneID).Return(true, nil).Once()
	effort.On("HasEffort", mock.Anything, f.sprintOneID).Return(false, nil).Once()

	first := f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)
	require.Equal(t, http.StatusConflict, first.Code, "con esfuerzo registrado debe rechazarse: %s", first.Body.String())
	sprintStored(t, f.env.db, f.sprintOneID.String())

	second := f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)

	requireSprintDeleted(t, f.env.db, second, f.sprintOneID)
	effort.AssertExpectations(t)
}

// Escenario: Datos inválidos (004/US7)
// Dejar el nombre o el Sprint Goal vacíos, poner una fecha de fin igual a la de inicio o un Sprint
// Goal de 501 caracteres se rechaza con 400 señalando el campo y el motivo; los demás datos del
// cuerpo, que sí son válidos y distintos, tampoco se aplican: ningún cambio parcial (FR-013, FR-014).
func TestUS7_DatosInvalidos(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value string
		part  string
	}{
		{"nombre vacío", "name", "", "obligatori"},
		{"Sprint Goal vacío", "goal", "", "obligatori"},
		{"fin no posterior", "endDate", "2026-10-03", "posterior"},
		{"Sprint Goal de 501", "goal", strings.Repeat("ñ", 501), "500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSprintUS2Fixture(t)
			before := sprintStored(t, f.env.db, f.sprintOneID.String())
			body := sprintBody("Sprint 1 corregido", "Publicar el alta de clientes", "2026-10-03", "2026-10-17")
			body[tc.field] = tc.value

			rec := f.env.updateSprint(t, f.brunoToken, f.sprintOneID, body)

			sprintRequireFieldRejected(t, rec, tc.field, tc.part)
			requireSprintDataUntouched(t, f.env.db, before)
		})
	}
}

// Escenario: Superposición (004/US7)
// Mover "Sprint 2" a un período que se superpone con "Sprint 1" se rechaza con 409 indicando el
// identificador y el nombre de "Sprint 1", y "Sprint 2" no cambia; en cambio, mover "Sprint 1" a un
// período que se superpone con el suyo propio se acepta, porque un sprint no choca consigo mismo
// (FR-013, SC-004).
func TestUS7_Superposicion(t *testing.T) {
	t.Run("con otro sprint", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		sprintTwoID := sprintSeed(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-20", "2026-11-02")
		before := sprintStored(t, f.env.db, sprintTwoID.String())

		rec := f.env.updateSprint(t, f.brunoToken, sprintTwoID, sprintBody("Sprint 2", sprintGoal, "2026-10-10", "2026-10-25"))

		require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
		var body sprintConflictBody
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
		assert.Contains(t, body.Error, "Sprint 1", "el mensaje debe nombrar el sprint en conflicto")
		if assert.NotNil(t, body.Sprint, "falta el sprint en conflicto: %s", rec.Body.String()) {
			assert.Equal(t, f.sprintOneID.String(), body.Sprint.ID)
			assert.Equal(t, "Sprint 1", body.Sprint.Name)
		}
		requireSprintDataUntouched(t, f.env.db, before)
	})

	t.Run("consigo mismo", func(t *testing.T) {
		f := newSprintUS2Fixture(t)
		body := sprintBody("Sprint 1", sprintGoal, "2026-10-03", "2026-10-16")

		rec := f.env.updateSprint(t, f.brunoToken, f.sprintOneID, body)

		requireUpdatedSprint(t, rec, f.sprintOneID, body, "planned")
		requireStoredSprintData(t, f.env.db, f.sprintOneID, body)
	})
}

// Escenario: Modificar activo (004/US7)
// Modificar un sprint Activo se rechaza con 409 indicando que solo se modifica un sprint
// Planificado, y el sprint no cambia (FR-016).
func TestUS7_ModificarActivo(t *testing.T) {
	f := newSprintUS2Fixture(t)
	activeID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	before := sprintStored(t, f.env.db, activeID.String())

	rec := f.env.updateSprint(t, f.brunoToken, activeID, sprintBody("Sprint 2 corregido", sprintGoal, "2026-10-15", "2026-10-30"))

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintUpdateNotPlannedMessage+`"}`, rec.Body.String())
	requireSprintDataUntouched(t, f.env.db, before)
}

// Escenario: Modificar cerrado (004/US7)
// Modificar un sprint Cerrado se rechaza con 409 porque un sprint cerrado no admite cambios, y el
// sprint no cambia (FR-016, RN-18).
func TestUS7_ModificarCerrado(t *testing.T) {
	f := newSprintUS2Fixture(t)
	closedID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 0", "2026-09-01", "2026-09-14", domain.SprintStatusClosed)
	before := sprintStored(t, f.env.db, closedID.String())

	rec := f.env.updateSprint(t, f.brunoToken, closedID, sprintBody("Sprint 0 corregido", sprintGoal, "2026-09-01", "2026-09-15"))

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintClosedMessage+`"}`, rec.Body.String())
	requireSprintDataUntouched(t, f.env.db, before)
}

// Escenario: Eliminar activo o cerrado (004/US7)
// Eliminar un sprint Activo o uno Cerrado se rechaza con 409 indicando que solo se elimina un sprint
// que nunca fue iniciado; el sprint y su compromiso siguen en la base (FR-048, RN-20, SC-016).
func TestUS7_EliminarActivoOCerrado(t *testing.T) {
	notCompleted := domain.StoryOutcomeNotCompleted
	cases := []struct {
		name    string
		status  domain.SprintStatus
		start   string
		end     string
		outcome *domain.StoryOutcome
	}{
		{"activo", domain.SprintStatusActive, "2026-10-15", "2026-10-28", nil},
		{"cerrado", domain.SprintStatusClosed, "2026-09-01", "2026-09-14", &notCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSprintUS2Fixture(t)
			sprintID := sprintSeedInStatus(t, f.env.db, f.projectID, f.anaID, "Sprint 2", tc.start, tc.end, tc.status)
			storyID := f.env.createReadyStory(t, f.brunoToken, f.projectID, "Exportar el reporte")
			sprintSeedCommitment(t, f.env.db, sprintID, storyID, sprintDate(t, tc.start), tc.outcome, nil)
			before := sprintStored(t, f.env.db, sprintID.String())

			rec := f.env.deleteSprint(t, f.brunoToken, sprintID)

			assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.JSONEq(t, `{"error":"`+sprintDeleteStartedMessage+`"}`, rec.Body.String())
			requireSprintDataUntouched(t, f.env.db, before)
			assert.EqualValues(t, 1, sprintCommitmentCount(t, f.env.db, "sprint_id = ? AND story_id = ?", sprintID, storyID),
				"el compromiso debe seguir en sprint_stories")
		})
	}
}

// Escenario: Esfuerzo registrado (004/US7)
// Con el puerto de esfuerzo reemplazado por un doble que informa esfuerzo registrado, eliminar un
// sprint Planificado con dos historias se rechaza con 409 indicando que primero hay que eliminar esos
// registros; el sprint y sus historias comprometidas no cambian (FR-048, RN-20, SC-016).
func TestUS7_EsfuerzoRegistrado(t *testing.T) {
	effort := &sprintEffortDouble{}
	f := newSprintUS2Fixture(t, func(p *sprintTestPorts) { p.effort = effort })
	effort.On("HasEffort", mock.Anything, f.sprintOneID).Return(true, nil)
	stories := f.commitStories(t, f.sprintOneID, "Exportar el reporte", "Filtrar por fecha")
	before := sprintStored(t, f.env.db, f.sprintOneID.String())

	rec := f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintDeleteEffortMessage+`"}`, rec.Body.String())
	requireSprintDataUntouched(t, f.env.db, before)
	requireCommittedOpen(t, f.env.db, f.sprintOneID, stories...)
	effort.AssertCalled(t, "HasEffort", mock.Anything, f.sprintOneID)
}

// Escenario: Sprint eliminado (004/US7)
// Sobre un sprint ya eliminado, GET, PUT y DELETE responden el 404 de sprint inexistente del
// contrato (FR-058).
func TestUS7_SprintEliminado(t *testing.T) {
	f := newSprintUS2Fixture(t)
	requireSprintDeleted(t, f.env.db, f.env.deleteSprint(t, f.brunoToken, f.sprintOneID), f.sprintOneID)

	cases := []struct {
		name string
		send func(t *testing.T) *httptest.ResponseRecorder
	}{
		{"GET", func(t *testing.T) *httptest.ResponseRecorder {
			return f.env.getSprintDetail(t, f.brunoToken, f.sprintOneID)
		}},
		{"PUT", func(t *testing.T) *httptest.ResponseRecorder {
			return f.env.updateSprint(t, f.brunoToken, f.sprintOneID, sprintBody("Sprint 1", sprintGoal, "2026-10-01", "2026-10-14"))
		}},
		{"DELETE", func(t *testing.T) *httptest.ResponseRecorder {
			return f.env.deleteSprint(t, f.brunoToken, f.sprintOneID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := tc.send(t)

			assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, sprintNotFoundBody, rec.Body.String())
		})
	}
}

// Pruebas de caracterización transversales de la fase de cierre (T088), sin etiqueta 004/USn: fijan
// comportamiento que ya existe porque las once rutas de contracts/openapi.yaml ya están detrás de los
// middlewares de sesión y de acceso, y las reglas de solo lectura ya están implementadas, así que no
// tienen etapa RED. Cubren la sesión obligatoria en cada acción (SC-013), los 404 indistinguibles
// entre recurso ajeno e inexistente (SC-014), la solo lectura del proyecto Finalizado (SC-012) y que
// ninguna respuesta exponga el hash de la contraseña (Principio VI).

// sprintEndpointIDs son los identificadores con los que se arman las rutas y los cuerpos de los once
// endpoints: el proyecto, el sprint Planificado (detalle, modificación, eliminación, historias e
// inicio), el sprint Activo (cierre), la historia libre que se asigna, la comprometida en el
// Planificado que se quita y la comprometida en el Activo que avanza y cuyo historial se consulta.
type sprintEndpointIDs struct {
	projectID      uuid.UUID
	plannedID      uuid.UUID
	activeID       uuid.UUID
	freeStoryID    uuid.UUID
	plannedStoryID uuid.UUID
	activeStoryID  uuid.UUID
}

// sprintEndpointRequest es una solicitud válida a uno de los once endpoints. scope indica de qué
// recurso de la ruta depende el 404 (colección, sprint o historia) y write, si la acción escribe.
type sprintEndpointRequest struct {
	name   string
	method string
	path   string
	body   map[string]any
	scope  string
	write  bool
}

// Ámbitos de los endpoints según el recurso de la ruta.
const (
	sprintScopeCollection = "collection"
	sprintScopeSprint     = "sprint"
	sprintScopeStory      = "story"
)

// sprintEndpointRequests arma, en el orden de contracts/openapi.yaml, una solicitud válida a cada uno
// de los once endpoints de la feature con los identificadores recibidos: dos de colección del
// proyecto, siete sobre un sprint y dos sobre una historia; ocho escriben y tres leen.
func sprintEndpointRequests(ids sprintEndpointIDs) []sprintEndpointRequest {
	return []sprintEndpointRequest{
		{"createSprint", http.MethodPost, sprintsPath(ids.projectID), sprintBody("Sprint 3", sprintGoal, "2026-11-01", "2026-11-14"), sprintScopeCollection, true},
		{"listSprints", http.MethodGet, sprintsPath(ids.projectID), nil, sprintScopeCollection, false},
		{"getSprint", http.MethodGet, sprintPath(ids.plannedID), nil, sprintScopeSprint, false},
		{"updateSprint", http.MethodPut, sprintPath(ids.plannedID), sprintBody("Sprint 1 corregido", sprintGoal, "2026-10-01", "2026-10-14"), sprintScopeSprint, true},
		{"deleteSprint", http.MethodDelete, sprintPath(ids.plannedID), nil, sprintScopeSprint, true},
		{"assignSprintStories", http.MethodPost, sprintStoriesPath(ids.plannedID), map[string]any{"storyIds": []string{ids.freeStoryID.String()}}, sprintScopeSprint, true},
		{"removeSprintStories", http.MethodDelete, sprintStoriesPath(ids.plannedID) + "?storyId=" + ids.plannedStoryID.String(), nil, sprintScopeSprint, true},
		{"startSprint", http.MethodPost, sprintStartPath(ids.plannedID), nil, sprintScopeSprint, true},
		{"closeSprint", http.MethodPost, sprintClosePath(ids.activeID), nil, sprintScopeSprint, true},
		{"changeStoryStatus", http.MethodPut, sprintStoryStatusPath(ids.activeStoryID), map[string]any{"status": "in_progress"}, sprintScopeStory, true},
		{"listStorySprints", http.MethodGet, "/api/v1/stories/" + ids.activeStoryID.String() + "/sprints", nil, sprintScopeStory, false},
	}
}

// sprintEndpointsFixture es el "Dado" común de las pruebas de caracterización: Ana es propietaria y
// Bruno integrante de un proyecto con un sprint Planificado y uno Activo, cada uno con una historia
// comprometida, y una historia lista sin sprint.
type sprintEndpointsFixture struct {
	env        sprintTestEnv
	brunoToken string
	ids        sprintEndpointIDs
}

// newSprintEndpointsFixture siembra el "Dado" común con el proyecto En curso, para que la 003 admita
// crear las historias, y después lo deja en el estado indicado directamente en la base.
func newSprintEndpointsFixture(t *testing.T, status projectsdomain.ProjectStatus) sprintEndpointsFixture {
	t.Helper()
	env := newSprintTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)

	// Sprint Planificado con una historia comprometida y sprint Activo, que contiene al reloj por
	// defecto, con otra; la tercera historia queda lista y sin sprint.
	plannedID := sprintSeed(t, env.db, projectID, anaID, "Sprint 1", "2026-10-01", "2026-10-14")
	activeID := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 2", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
	plannedStoryID := env.createReadyStory(t, brunoToken, projectID, "Alta de clientes")
	sprintSeedCommitment(t, env.db, plannedID, plannedStoryID, sprintDate(t, "2026-09-30"), nil, nil)
	activeStoryID := env.createReadyStory(t, brunoToken, projectID, "Exportar el reporte")
	sprintSeedCommitment(t, env.db, activeID, activeStoryID, sprintDate(t, "2026-10-15"), nil, nil)
	freeStoryID := env.createReadyStory(t, brunoToken, projectID, "Buscar clientes")

	if status != projectsdomain.ProjectStatusInProgress {
		require.NoError(t, env.db.Model(&projectsdomain.Project{}).Where("id = ?", projectID).
			Update("status", status).Error, "no se pudo fijar el estado del proyecto")
	}
	return sprintEndpointsFixture{
		env:        env,
		brunoToken: brunoToken,
		ids: sprintEndpointIDs{
			projectID:      projectID,
			plannedID:      plannedID,
			activeID:       activeID,
			freeStoryID:    freeStoryID,
			plannedStoryID: plannedStoryID,
			activeStoryID:  activeStoryID,
		},
	}
}

// sendSprintEndpoint envía la solicitud con la cabecera Authorization recibida, o sin ella si viene
// vacía, a diferencia de sprintDo, que siempre la agrega. Toda respuesta pasa por la verificación de
// que no expone el hash de la contraseña.
func (e sprintTestEnv) sendSprintEndpoint(t *testing.T, req sprintEndpointRequest, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if req.body != nil {
		var err error
		payload, err = json.Marshal(req.body)
		require.NoError(t, err, "no se pudo serializar la solicitud")
	}
	httpReq := httptest.NewRequest(req.method, req.path, bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		httpReq.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, httpReq)
	requireNoPasswordHash(t, rec)
	return rec
}

// sprintTamperedToken altera el primer carácter de la firma del JWT, de modo que el token conserva su
// forma pero la firma ya no verifica.
func sprintTamperedToken(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "el token de prueba no tiene la forma de un JWT")
	replacement := "A"
	if strings.HasPrefix(parts[2], "A") {
		replacement = "B"
	}
	parts[2] = replacement + parts[2][1:]
	return strings.Join(parts, ".")
}

// requireNoPasswordHash exige que la respuesta no contenga el hash de la contraseña en ninguna de
// sus dos grafías (Principio VI).
func requireNoPasswordHash(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := rec.Body.String()
	assert.NotContains(t, body, "password_hash", "la respuesta no debe exponer el hash de la contraseña")
	assert.NotContains(t, body, "passwordHash", "la respuesta no debe exponer el hash de la contraseña")
}

// sprintDatabaseSnapshot lee como JSON, ordenadas por id, todas las filas de las tablas que pueden
// tocar las acciones de la feature, para verificar que un rechazo no produjo ningún cambio. El "Dado"
// de sprintEndpointsFixture siembra filas en las seis, así que una tabla vacía indica que la lectura
// falló y que la comparación no probaría nada.
func sprintDatabaseSnapshot(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	tables := []string{"projects", "project_members", "user_stories", "acceptance_criteria", "sprints", "sprint_stories"}
	snapshot := make(map[string][]string, len(tables))
	for _, table := range tables {
		var rows []string
		require.NoError(t, db.Raw("SELECT row_to_json(t)::text FROM "+table+" t ORDER BY t.id").Scan(&rows).Error,
			"no se pudieron leer las filas de %s", table)
		require.NotEmpty(t, rows, "la foto de %s no debería estar vacía", table)
		snapshot[table] = rows
	}
	return snapshot
}

// Caracterización: sesión obligatoria en cada acción (SC-013)
// Cada uno de los once endpoints, sin cabecera Authorization y con un token de firma adulterada,
// responde 401 con el mensaje de la 001 y no cambia ninguna fila.
func TestSprintEndpoints_SinSesion(t *testing.T) {
	f := newSprintEndpointsFixture(t, projectsdomain.ProjectStatusInProgress)
	tampered := "Bearer " + sprintTamperedToken(t, f.brunoToken)

	requests := sprintEndpointRequests(f.ids)
	require.Len(t, requests, 11, "la feature expone once endpoints")
	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			for _, auth := range []struct {
				name   string
				header string
			}{{"sin cabecera", ""}, {"token adulterado", tampered}} {
				before := sprintDatabaseSnapshot(t, f.env.db)

				rec := f.env.sendSprintEndpoint(t, req, auth.header)

				assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s: cuerpo: %s", auth.name, rec.Body.String())
				assert.JSONEq(t, sprintAccessSessionBody, rec.Body.String(), auth.name)
				assert.Equal(t, before, sprintDatabaseSnapshot(t, f.env.db), "%s: no debería cambiar ninguna fila", auth.name)
			}
		})
	}
}

// Caracterización: 404 indistinguible entre recurso ajeno e inexistente (SC-014)
// Carla, que no integra el proyecto, recibe en las siete acciones de sprint el mismo 404 ante el
// sprint ajeno que ante un sprintId inexistente, y en las dos de colección el mismo 404 ante el
// proyecto ajeno que ante un projectId inexistente: igual código, tipo de contenido y cuerpo byte a
// byte, sin cambiar ninguna fila.
func TestSprintEndpoints_NotFoundIndistinguible(t *testing.T) {
	f := newSprintEndpointsFixture(t, projectsdomain.ProjectStatusInProgress)
	_, carlaToken := f.env.signUp(t, "Carla Gómez", "carla@example.com")
	carla := "Bearer " + carlaToken

	// Las solicitudes "inexistentes" son las mismas con proyecto y sprints que no existen; las
	// historias de cuerpos y consultas no cambian, porque el 404 lo decide el recurso de la ruta.
	missingIDs := f.ids
	missingIDs.projectID = uuid.New()
	missingIDs.plannedID = uuid.New()
	missingIDs.activeID = uuid.New()
	foreign := sprintEndpointRequests(f.ids)
	missing := sprintEndpointRequests(missingIDs)

	checked := map[string]int{}
	for i, req := range foreign {
		if req.scope == sprintScopeStory {
			continue
		}
		checked[req.scope]++
		t.Run(req.name, func(t *testing.T) {
			before := sprintDatabaseSnapshot(t, f.env.db)

			foreignRec := f.env.sendSprintEndpoint(t, req, carla)
			missingRec := f.env.sendSprintEndpoint(t, missing[i], carla)

			want := sprintNotFoundBody
			if req.scope == sprintScopeCollection {
				want = `{"error":"` + sprintProjectNotFound + `"}`
			}
			assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
			assert.JSONEq(t, want, foreignRec.Body.String())
			assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
			assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
			assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
			assert.Equal(t, before, sprintDatabaseSnapshot(t, f.env.db), "no debería cambiar ninguna fila")
		})
	}
	assert.Equal(t, map[string]int{sprintScopeSprint: 7, sprintScopeCollection: 2}, checked,
		"se comparan siete acciones de sprint y dos de colección")
}

// Caracterización: 404 indistinguible en el historial de sprints de una historia (SC-014)
// Carla, que no integra el proyecto, recibe al consultar el historial de sprints de la historia
// comprometida en el sprint Activo el mismo 404 que ante un storyId inexistente: igual código, tipo
// de contenido y cuerpo byte a byte, sin cambiar ninguna fila.
func TestSprintEndpoints_StorySprintsNotFoundIndistinguible(t *testing.T) {
	f := newSprintEndpointsFixture(t, projectsdomain.ProjectStatusInProgress)
	_, carlaToken := f.env.signUp(t, "Carla Gómez", "carla@example.com")
	carla := "Bearer " + carlaToken

	// Se toma la solicitud listStorySprints de la tabla común, con la historia ajena y con una
	// historia que no existe; el resto de los identificadores no interviene en la ruta.
	missingIDs := f.ids
	missingIDs.activeStoryID = uuid.New()
	findListStorySprints := func(ids sprintEndpointIDs) sprintEndpointRequest {
		for _, req := range sprintEndpointRequests(ids) {
			if req.name == "listStorySprints" {
				return req
			}
		}
		require.FailNow(t, "falta la solicitud listStorySprints en la tabla de endpoints")
		return sprintEndpointRequest{}
	}
	foreign := findListStorySprints(f.ids)
	missing := findListStorySprints(missingIDs)
	before := sprintDatabaseSnapshot(t, f.env.db)

	foreignRec := f.env.sendSprintEndpoint(t, foreign, carla)
	missingRec := f.env.sendSprintEndpoint(t, missing, carla)

	// La historia ajena responde el 404 del contrato, idéntico al de la inexistente, y nada cambia.
	assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
	assert.JSONEq(t, `{"error":"`+sprintStoryNotFound+`"}`, foreignRec.Body.String())
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	assert.Equal(t, before, sprintDatabaseSnapshot(t, f.env.db), "no debería cambiar ninguna fila")
}

// Caracterización: solo lectura del proyecto Finalizado (SC-012)
// Bruno, integrante de un proyecto Finalizado, recibe 409 con el mensaje de solo lectura en las ocho
// escrituras, sin que cambie ninguna fila, y 200 en las tres lecturas.
func TestSprintEndpoints_ProyectoFinalizado(t *testing.T) {
	f := newSprintEndpointsFixture(t, projectsdomain.ProjectStatusFinished)
	bruno := "Bearer " + f.brunoToken

	counts := map[bool]int{}
	for _, req := range sprintEndpointRequests(f.ids) {
		counts[req.write]++
		t.Run(req.name, func(t *testing.T) {
			before := sprintDatabaseSnapshot(t, f.env.db)

			rec := f.env.sendSprintEndpoint(t, req, bruno)

			if !req.write {
				assert.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
				return
			}
			assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.JSONEq(t, `{"error":"`+sprintReadOnlyMessage+`"}`, rec.Body.String())
			assert.Equal(t, before, sprintDatabaseSnapshot(t, f.env.db), "no debería cambiar ninguna fila")
		})
	}
	assert.Equal(t, map[bool]int{true: 8, false: 3}, counts, "la feature tiene ocho escrituras y tres lecturas")
}

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
)

// Pruebas de integración de punta a punta del backlog: atraviesan el router de Gin compuesto como en
// cmd/app, el middleware de sesión de la 001, RequireMembership de la 002, el servicio y el
// repositorio del backlog contra la base smye_test (Principios II y V). Las de US1 (T015) cubren
// POST /api/v1/projects/{projectId}/stories, las de US2 (T030), GET
// /api/v1/projects/{projectId}/stories, y las de US3 (T040), GET /api/v1/stories/{storyId}; las
// historias siguientes agregan las suyas en este archivo. Los ayudantes llevan el prefijo "backlog"
// para no chocar con los de validation_test.go y middleware_test.go.

// Textos exactos que fija el contrato para el 400 de entrada inválida, el 404 de proyecto
// inexistente y el 409 de backlog de solo lectura.
const (
	backlogInvalidFieldsError = "Hay campos inválidos en la solicitud."
	backlogProjectNotFound    = "El proyecto no existe."
	backlogReadOnlyMessage    = "El proyecto está finalizado y su backlog es de solo lectura."
)

// backlogScale es el texto con la escala completa que debe enumerar el rechazo de Story Points.
const backlogScale = "0, 1, 2, 3, 5, 8, 13, 20, 40 y 100"

// backlogUserPassword es la contraseña de las cuentas que crean las pruebas; cumple las reglas de
// la 001.
const backlogUserPassword = "Backlog2026"

// Datos del caso normal de la spec (US1, escenario 1).
const (
	backlogStoryTitle       = "Exportar el reporte de velocidad a PDF"
	backlogStoryDescription = "El equipo necesita compartir la velocidad con la gerencia."
	backlogFirstCriterion   = "El PDF incluye la velocidad de los últimos cinco sprints."
	backlogSecondCriterion  = "El archivo se descarga con el nombre del proyecto."
)

// backlogStoryDetailBody refleja StoryDetail del contrato; se declara en la prueba para no depender
// de los DTO de producción al leer la respuesta. StoryPoints es un puntero para distinguir el 0 de
// "sin estimar".
type backlogStoryDetailBody struct {
	ID                 string   `json:"id"`
	ProjectID          string   `json:"projectId"`
	Title              string   `json:"title"`
	Description        *string  `json:"description"`
	Priority           string   `json:"priority"`
	Status             string   `json:"status"`
	StoryPoints        *int     `json:"storyPoints"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Readiness          struct {
		Ready   bool     `json:"ready"`
		Missing []string `json:"missing"`
	} `json:"readiness"`
	CreatedBy struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"createdBy"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	EverAssignedToSprint bool   `json:"everAssignedToSprint"`
}

// backlogValidationBody refleja ValidationErrorResponse del contrato.
type backlogValidationBody struct {
	Error  string `json:"error"`
	Fields []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields"`
}

// backlogTestPorts son los cuatro puertos hacia features que todavía no existen, más el repositorio
// de historias y el puerto de proyectos. Por defecto son las implementaciones de backlog/repository;
// una prueba los reemplaza por dobles para preparar historias comprometidas o con historial (plan.md,
// cómo se preparan los escenarios), o los envuelve para cambiar la base entre la resolución del
// acceso y la escritura (T091). db es la base de test, de solo lectura para los overrides, para que
// un envoltorio pueda escribir en ella.
type backlogTestPorts struct {
	sprints  domain.SprintAssignments
	effort   domain.EffortRecords
	defects  domain.DefectRecords
	sessions domain.EstimationSessions
	stories  domain.StoryRepository
	projects domain.ProjectDirectory
	db       *gorm.DB
}

// backlogTestEnv reúne el router compuesto como en cmd/app, la base de test y el servicio de
// autenticación con el que se crean las cuentas y las sesiones de cada escenario.
type backlogTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	auth   *authservice.AuthService
}

// newBacklogTestEnv abre la base de test vacía y compone, igual que cmd/app, el límite global del
// cuerpo, el rechazo de queries mal formadas, la cadena real de la 001 (para el middleware de
// sesión), el módulo de proyectos de la 002 con su subgrupo /:projectId y las rutas del backlog
// colgadas de ese subgrupo, sin CORS ni Swagger. Cada override recibe los puertos por defecto y puede reemplazar cualquiera por un doble.
func newBacklogTestEnv(t *testing.T, overrides ...func(*backlogTestPorts)) backlogTestEnv {
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
	router.Use(httpserver.RejectMalformedQuery())
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

	// Módulo del backlog con los puertos por defecto o los dobles que pida la prueba.
	ports := backlogTestPorts{
		sprints:  repository.NewSprintAssignments(db),
		effort:   repository.NewEffortRecords(),
		defects:  repository.NewDefectRecords(),
		sessions: repository.NewEstimationSessions(db),
		stories:  repository.NewStoryRepository(db),
		projects: repository.NewProjectDirectory(db),
		db:       db,
	}
	for _, override := range overrides {
		override(&ports)
	}
	// La misma instancia del repositorio de historias sirve al servicio y a RequireStoryAccess del
	// grupo plano /api/v1/stories/:storyId, igual que en cmd/app.
	stories := service.NewStoryService(
		ports.stories,
		ports.sprints,
		ports.effort,
		ports.defects,
		ports.sessions,
		ports.projects,
	)
	handler := NewHandler(stories)
	RegisterRoutes(projectGroup, handler)
	RegisterStoryRoutes(router, authHandler.RequireSession, ports.stories, handler)
	return backlogTestEnv{router: router, db: db, auth: auth}
}

// signUp registra una cuenta con el servicio de la 001, inicia su sesión y devuelve el identificador
// de la cuenta y el token de la sesión.
func (e backlogTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	user, err := e.auth.Register(ctx, fullName, email, backlogUserPassword)
	require.NoError(t, err, "no se pudo registrar la cuenta de prueba %s", email)
	login, err := e.auth.Login(ctx, email, backlogUserPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de prueba %s", email)
	return user.ID, login.Token
}

// backlogSeedProject inserta directamente en la base un proyecto del propietario en el estado
// indicado, con las membresías del propietario y de cada integrante adicional, y devuelve su
// identificador, para preparar el "Dado" sin pasar por la API de la 002.
func backlogSeedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, status projectsdomain.ProjectStatus, extraMembers ...uuid.UUID) uuid.UUID {
	t.Helper()
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         "Portal de Clientes",
		NameNormalized:               "portal de clientes",
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
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

// backlogStoriesPath es la ruta de creación y listado de historias del proyecto según
// contracts/openapi.yaml.
func backlogStoriesPath(projectID uuid.UUID) string {
	return "/api/v1/projects/" + projectID.String() + "/stories"
}

// postStory envía el cuerpo a POST /api/v1/projects/{projectId}/stories con el token como Bearer.
func (e backlogTestEnv) postStory(t *testing.T, token string, projectID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err, "no se pudo serializar la solicitud")
	req := httptest.NewRequest(http.MethodPost, backlogStoriesPath(projectID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// backlogMinimalBody devuelve un cuerpo válido con solo título y prioridad; cada prueba agrega o
// modifica los campos que le interesan.
func backlogMinimalBody() map[string]any {
	return map[string]any{"title": backlogStoryTitle, "priority": "high"}
}

// backlogFullBody devuelve el cuerpo del caso normal de US1: prioridad Alta, 5 Story Points y dos
// criterios. Los Story Points van como json.Number para enviar su texto literal.
func backlogFullBody() map[string]any {
	return map[string]any{
		"title":              backlogStoryTitle,
		"description":        backlogStoryDescription,
		"priority":           "high",
		"storyPoints":        json.Number("5"),
		"acceptanceCriteria": []string{backlogFirstCriterion, backlogSecondCriterion},
	}
}

// requireCreatedStory exige un 201 cuyo cuerpo sea un StoryDetail con id UUID y lo devuelve
// decodificado.
func requireCreatedStory(t *testing.T, rec *httptest.ResponseRecorder) backlogStoryDetailBody {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, "cuerpo: %s", rec.Body.String())
	var body backlogStoryDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 201 no es JSON válido: %s", rec.Body.String())
	_, err := uuid.Parse(body.ID)
	require.NoError(t, err, "id = %q no es un UUID", body.ID)
	return body
}

// backlogRawBody decodifica la respuesta como objeto crudo, para comprobar que un campo vale null o
// [] y no que simplemente falta.
func backlogRawBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw), "el cuerpo no es JSON válido: %s", rec.Body.String())
	return raw
}

// backlogRequireFieldRejected exige un 400 con ValidationErrorResponse que señale el campo indicado
// y cuyo mensaje mencione cada una de las piezas esperadas, sin distinguir mayúsculas.
func backlogRequireFieldRejected(t *testing.T, rec *httptest.ResponseRecorder, field string, parts ...string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, "cuerpo: %s", rec.Body.String())
	var body backlogValidationBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 400 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, backlogInvalidFieldsError, body.Error)
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

// backlogCountRows cuenta las filas del modelo indicado, opcionalmente filtradas.
func backlogCountRows(t *testing.T, db *gorm.DB, model any, query string, args ...any) int64 {
	t.Helper()
	tx := db.Model(model)
	if query != "" {
		tx = tx.Where(query, args...)
	}
	var n int64
	require.NoError(t, tx.Count(&n).Error, "no se pudieron contar las filas")
	return n
}

// backlogRequireNothingCreated exige que no exista ninguna historia ni ningún criterio (FR-013).
func backlogRequireNothingCreated(t *testing.T, db *gorm.DB) {
	t.Helper()
	assert.Zero(t, backlogCountRows(t, db, &domain.UserStory{}, ""), "no debería haberse creado ninguna historia")
	assert.Zero(t, backlogCountRows(t, db, &domain.AcceptanceCriterion{}, ""), "no debería haberse creado ningún criterio")
}

// backlogStoredStory lee de user_stories la fila con el identificador devuelto por la API.
func backlogStoredStory(t *testing.T, db *gorm.DB, id string) domain.UserStory {
	t.Helper()
	var story domain.UserStory
	require.NoError(t, db.First(&story, "id = ?", id).Error, "no se encontró la historia %s en user_stories", id)
	return story
}

// backlogStoredCriteria lee de acceptance_criteria los criterios de la historia ordenados por
// posición.
func backlogStoredCriteria(t *testing.T, db *gorm.DB, storyID string) []domain.AcceptanceCriterion {
	t.Helper()
	var criteria []domain.AcceptanceCriterion
	require.NoError(t, db.Where("story_id = ?", storyID).Order("position").Find(&criteria).Error,
		"no se pudieron leer los criterios de la historia")
	return criteria
}

// Escenario: Creación completa (003/US1)
// Bruno, integrante que no es el propietario de un proyecto En curso, crea "Exportar el reporte de
// velocidad a PDF" con prioridad Alta, 5 Story Points y dos criterios: la historia nace pending, con
// esos datos, los criterios en el orden enviado, atribuida a Bruno y lista para planificar, tanto en
// la respuesta como en la base (FR-001, FR-008, FR-011, FR-049, RN-02).
func TestUS1_CreacionCompleta(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress, brunoID)

	rec := env.postStory(t, brunoToken, projectID, backlogFullBody())

	// La respuesta es el detalle completo de la historia creada.
	got := requireCreatedStory(t, rec)
	assert.Equal(t, projectID.String(), got.ProjectID)
	assert.Equal(t, backlogStoryTitle, got.Title)
	require.NotNil(t, got.Description, "la descripción no debería ser null")
	assert.Equal(t, backlogStoryDescription, *got.Description)
	assert.Equal(t, "high", got.Priority)
	assert.Equal(t, "pending", got.Status)
	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 5, *got.StoryPoints)
	assert.Equal(t, []string{backlogFirstCriterion, backlogSecondCriterion}, got.AcceptanceCriteria)
	assert.True(t, got.Readiness.Ready, "la historia debería estar lista para planificar")
	assert.Empty(t, got.Readiness.Missing)
	assert.Equal(t, brunoID.String(), got.CreatedBy.ID, "el autor debería ser el titular de la sesión")
	assert.Equal(t, "Bruno Díaz", got.CreatedBy.FullName)
	assert.False(t, got.EverAssignedToSprint)
	_, err := time.Parse(time.RFC3339, got.CreatedAt)
	assert.NoError(t, err, "createdAt = %q no es una fecha y hora RFC 3339", got.CreatedAt)
	_, err = time.Parse(time.RFC3339, got.UpdatedAt)
	assert.NoError(t, err, "updatedAt = %q no es una fecha y hora RFC 3339", got.UpdatedAt)
	assert.NotContains(t, rec.Body.String(), "password", "la respuesta no debería exponer datos de la contraseña")

	// La base guarda la historia y sus criterios con posiciones consecutivas desde 0.
	stored := backlogStoredStory(t, env.db, got.ID)
	assert.Equal(t, projectID, stored.ProjectID)
	assert.Equal(t, domain.StoryStatusPending, stored.Status)
	assert.Equal(t, brunoID, stored.CreatedByID)
	require.NotNil(t, stored.StoryPoints)
	assert.Equal(t, 5, *stored.StoryPoints)
	criteria := backlogStoredCriteria(t, env.db, got.ID)
	require.Len(t, criteria, 2)
	assert.Equal(t, 0, criteria[0].Position)
	assert.Equal(t, backlogFirstCriterion, criteria[0].Text)
	assert.Equal(t, 1, criteria[1].Position)
	assert.Equal(t, backlogSecondCriterion, criteria[1].Text)
}

// Escenario: Solo título y prioridad (003/US1)
// Con solo título y prioridad la historia se crea igual: sin estimar (storyPoints null), sin
// descripción (null), sin criterios ([] y no null) y no lista para planificar, con las dos
// condiciones faltantes en orden (FR-005, FR-015, FR-029, FR-040).
func TestUS1_SoloTituloYPrioridad(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)

	rec := env.postStory(t, token, projectID, map[string]any{
		"title":    "Revisar los permisos de los integrantes",
		"priority": "low",
	})

	// La respuesta informa los campos ausentes como null o [] y la historia no lista.
	got := requireCreatedStory(t, rec)
	assert.Equal(t, "Revisar los permisos de los integrantes", got.Title)
	assert.Equal(t, "low", got.Priority)
	assert.Equal(t, "pending", got.Status)
	raw := backlogRawBody(t, rec)
	for _, key := range []string{"storyPoints", "description"} {
		value, present := raw[key]
		assert.True(t, present, "el campo %s debería estar presente", key)
		assert.Nil(t, value, "el campo %s debería ser null", key)
	}
	assert.Equal(t, []any{}, raw["acceptanceCriteria"], "acceptanceCriteria debería ser [] y no null")
	assert.False(t, got.Readiness.Ready, "la historia no debería estar lista para planificar")
	assert.Equal(t, []string{"story_points", "acceptance_criteria"}, got.Readiness.Missing)

	// La base guarda la historia sin estimación, sin descripción y sin criterios.
	stored := backlogStoredStory(t, env.db, got.ID)
	assert.Nil(t, stored.StoryPoints, "story_points debería ser NULL")
	assert.Nil(t, stored.Description, "description debería ser NULL")
	assert.Empty(t, backlogStoredCriteria(t, env.db, got.ID))
}

// Escenario: Proyecto planificado (003/US1)
// En un proyecto Planificado la creación se acepta, porque el backlog se carga antes de que el
// proyecto arranque (US1-3).
func TestUS1_ProyectoPlanificado(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusPlanned)

	got := requireCreatedStory(t, env.postStory(t, token, projectID, backlogMinimalBody()))

	assert.Equal(t, projectID.String(), got.ProjectID)
	assert.Equal(t, int64(1), backlogCountRows(t, env.db, &domain.UserStory{}, "project_id = ?", projectID))
}

// Escenario: Cero Story Points (003/US1)
// Con exactamente 0 Story Points y un criterio, la historia se crea con la estimación 0, distinta de
// "sin estimar", y queda lista para planificar (FR-015, RN-07).
func TestUS1_CeroStoryPoints(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	body := backlogMinimalBody()
	body["storyPoints"] = json.Number("0")
	body["acceptanceCriteria"] = []string{backlogFirstCriterion}

	rec := env.postStory(t, token, projectID, body)

	// La respuesta lleva storyPoints: 0, no null, y la historia lista para planificar.
	got := requireCreatedStory(t, rec)
	raw := backlogRawBody(t, rec)
	assert.Equal(t, float64(0), raw["storyPoints"], "storyPoints debería ser 0 y no null")
	assert.True(t, got.Readiness.Ready, "con 0 Story Points y un criterio la historia está lista")
	assert.Empty(t, got.Readiness.Missing)

	// La base guarda 0 y no NULL.
	stored := backlogStoredStory(t, env.db, got.ID)
	require.NotNil(t, stored.StoryPoints, "story_points debería ser 0 y no NULL")
	assert.Equal(t, 0, *stored.StoryPoints)
}

// Escenario: Límites del título (003/US1)
// Un título de exactamente 150 caracteres se acepta y uno de 151 se rechaza indicando el máximo, sin
// crear nada (FR-002, RN-03). Se usan caracteres de dos bytes para que el límite se cuente en
// caracteres y no en bytes.
func TestUS1_LimitesDelTitulo(t *testing.T) {
	t.Run("150 caracteres se acepta", func(t *testing.T) {
		env := newBacklogTestEnv(t)
		ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
		title := strings.Repeat("ñ", 150)
		body := backlogMinimalBody()
		body["title"] = title

		got := requireCreatedStory(t, env.postStory(t, token, projectID, body))

		assert.Equal(t, title, got.Title)
		assert.Equal(t, title, backlogStoredStory(t, env.db, got.ID).Title)
	})
	t.Run("151 caracteres se rechaza", func(t *testing.T) {
		env := newBacklogTestEnv(t)
		ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
		body := backlogMinimalBody()
		body["title"] = strings.Repeat("ñ", 151)

		rec := env.postStory(t, token, projectID, body)

		backlogRequireFieldRejected(t, rec, "title", "150")
		backlogRequireNothingCreated(t, env.db)
	})
}

// Escenario: Límites del criterio (003/US1)
// Un criterio de exactamente 500 caracteres y otro de 1 carácter se aceptan, en el orden enviado
// (FR-006, RN-08).
func TestUS1_LimitesDelCriterio(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	longest := strings.Repeat("é", 500)
	body := backlogMinimalBody()
	body["acceptanceCriteria"] = []string{longest, "a"}

	got := requireCreatedStory(t, env.postStory(t, token, projectID, body))

	assert.Equal(t, []string{longest, "a"}, got.AcceptanceCriteria)
	criteria := backlogStoredCriteria(t, env.db, got.ID)
	require.Len(t, criteria, 2)
	assert.Equal(t, 500, utf8.RuneCountInString(criteria[0].Text))
	assert.Equal(t, "a", criteria[1].Text)
}

// Escenario: Título repetido (003/US1)
// Dos historias con el mismo título se crean ambas, con identificadores distintos, porque el título
// no es único dentro del backlog (FR-012).
func TestUS1_TituloRepetido(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)

	first := requireCreatedStory(t, env.postStory(t, token, projectID, backlogMinimalBody()))
	second := requireCreatedStory(t, env.postStory(t, token, projectID, backlogMinimalBody()))

	assert.NotEqual(t, first.ID, second.ID, "las dos historias deberían tener identificadores distintos")
	assert.Equal(t, int64(2), backlogCountRows(t, env.db, &domain.UserStory{}, "project_id = ? AND title = ?", projectID, backlogStoryTitle))
}

// Escenario: Campos obligatorios (003/US1)
// Sin título, con un título solo de espacios o sin prioridad, la creación se rechaza indicando qué
// campo corregir y no se crea ninguna historia ni ningún criterio (FR-002, FR-004, FR-013). El
// cuerpo lleva un criterio válido para comprobar que tampoco se crea.
func TestUS1_CamposObligatorios(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(body map[string]any)
		field string
	}{
		{"sin título", func(body map[string]any) { delete(body, "title") }, "title"},
		{"título solo de espacios", func(body map[string]any) { body["title"] = "   \t  " }, "title"},
		{"sin prioridad", func(body map[string]any) { delete(body, "priority") }, "priority"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			body := backlogFullBody()
			tc.edit(body)

			rec := env.postStory(t, token, projectID, body)

			backlogRequireFieldRejected(t, rec, tc.field)
			backlogRequireNothingCreated(t, env.db)
		})
	}
}

// Escenario: Story Points fuera de la escala (003/US1)
// Con 4, 7, un valor negativo o un decimal, la creación se rechaza indicando los valores admitidos
// de la escala y no se crea nada (FR-014, FR-019, RN-06, SC-007).
func TestUS1_StoryPointsFueraDeEscala(t *testing.T) {
	cases := []struct {
		name    string
		literal string
	}{
		{"4", "4"},
		{"7", "7"},
		{"negativo", "-1"},
		{"decimal", "6.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			body := backlogFullBody()
			body["storyPoints"] = json.Number(tc.literal)

			rec := env.postStory(t, token, projectID, body)

			backlogRequireFieldRejected(t, rec, "storyPoints", backlogScale)
			backlogRequireNothingCreated(t, env.db)
		})
	}
}

// Escenario: Criterio vacío (003/US1)
// Con un criterio vacío o solo de espacios, la creación se rechaza indicando que cada criterio debe
// tener contenido, con el índice del criterio, y no se crea la historia ni el criterio válido que la
// acompaña (FR-007, FR-013).
func TestUS1_CriterioVacio(t *testing.T) {
	cases := []struct {
		name      string
		criterion string
	}{
		{"vacío", ""},
		{"solo de espacios", "   \t "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			body := backlogFullBody()
			body["acceptanceCriteria"] = []string{backlogFirstCriterion, tc.criterion}

			rec := env.postStory(t, token, projectID, body)

			backlogRequireFieldRejected(t, rec, "acceptanceCriteria[1]", "criterio", "contenido")
			backlogRequireNothingCreated(t, env.db)
		})
	}
}

// Escenario: Proyecto finalizado (003/US1)
// En un proyecto Finalizado la creación de un integrante se rechaza con 409 porque el backlog es de
// solo lectura, y no se crea nada (FR-048, RN-15).
func TestUS1_ProyectoFinalizado(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusFinished, brunoID)

	rec := env.postStory(t, brunoToken, projectID, backlogFullBody())

	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+backlogReadOnlyMessage+`"}`, rec.Body.String())
	backlogRequireNothingCreated(t, env.db)
}

// Escenario: No integrante (003/US1)
// Quien no es integrante del proyecto recibe 404 de proyecto inexistente, con el mismo código, tipo
// de contenido y cuerpo, byte a byte, que ante un projectId que no existe, y no se crea nada
// (FR-047, SC-006).
func TestUS1_NoIntegrante(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)

	foreignRec := env.postStory(t, outsiderToken, projectID, backlogFullBody())
	missingRec := env.postStory(t, outsiderToken, uuid.New(), backlogFullBody())

	assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
	assert.JSONEq(t, `{"error":"`+backlogProjectNotFound+`"}`, foreignRec.Body.String())
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	backlogRequireNothingCreated(t, env.db)
}

// Datos del caso normal de US3, escenario 1: el tercer criterio y los momentos fijos de creación y
// de última actualización con que se siembra la historia, para comprobar que el detalle devuelve
// exactamente esos valores en UTC con formato RFC 3339.
const backlogThirdCriterion = "El reporte respeta la zona horaria del sistema."

var (
	backlogDetailCreatedAt = time.Date(2026, 10, 20, 13, 5, 0, 0, time.UTC)
	backlogDetailUpdatedAt = time.Date(2026, 10, 20, 14, 30, 0, 0, time.UTC)
)

// backlogSprintAssignmentsDouble es el doble de testify del puerto SprintAssignments, para preparar
// una historia que está o estuvo en un sprint sin depender de specs/004-sprint-management.
type backlogSprintAssignmentsDouble struct {
	mock.Mock
}

// Find devuelve la asignación que la prueba configuró para la historia.
func (m *backlogSprintAssignmentsDouble) Find(ctx context.Context, storyID uuid.UUID) (domain.SprintAssignment, error) {
	args := m.Called(ctx, storyID)
	assignment, _ := args.Get(0).(domain.SprintAssignment)
	return assignment, args.Error(1)
}

// backlogSeedStory inserta directamente en la base la historia recibida y sus criterios, en el
// orden recibido y con la posición que trae cada uno, para preparar el "Dado" sin pasar por la API.
// Así una prueba puede sembrar los criterios desordenados y comprobar que el detalle los devuelve
// por posición. Sin identificador ni estado, la historia recibe un UUID nuevo y queda pending.
func backlogSeedStory(t *testing.T, db *gorm.DB, story domain.UserStory, criteria ...domain.AcceptanceCriterion) uuid.UUID {
	t.Helper()
	if story.ID == uuid.Nil {
		story.ID = uuid.New()
	}
	if story.Status == "" {
		story.Status = domain.StoryStatusPending
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	for _, criterion := range criteria {
		criterion.ID = uuid.New()
		criterion.StoryID = story.ID
		require.NoError(t, db.Create(&criterion).Error, "no se pudo sembrar el criterio %d", criterion.Position)
	}
	return story.ID
}

// backlogStoryPath es la ruta plana de una historia según contracts/openapi.yaml.
func backlogStoryPath(storyID string) string {
	return "/api/v1/stories/" + storyID
}

// getStory envía GET /api/v1/stories/{storyId} con el token como Bearer.
func (e backlogTestEnv) getStory(t *testing.T, token, storyID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, backlogStoryPath(storyID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// requireStoryDetail exige un 200 cuyo cuerpo sea un StoryDetail y lo devuelve decodificado.
func requireStoryDetail(t *testing.T, rec *httptest.ResponseRecorder) backlogStoryDetailBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body backlogStoryDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body
}

// backlogPoints devuelve un puntero al valor de Story Points indicado.
func backlogPoints(n int) *int {
	return &n
}

// backlogForeignAndMissing prepara una historia en un proyecto de Ana y devuelve dos respuestas de
// GET /api/v1/stories/{storyId}: la de Carla, que no es integrante, pidiendo esa historia, y la de
// Ana, integrante, pidiendo un identificador UUID que no existe.
func backlogForeignAndMissing(t *testing.T) (foreign, missing *httptest.ResponseRecorder) {
	t.Helper()
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedStory(t, env.db, domain.UserStory{
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Priority:    domain.StoryPriorityMedium,
		StoryPoints: backlogPoints(8),
		CreatedByID: anaID,
	}, domain.AcceptanceCriterion{Position: 0, Text: backlogFirstCriterion})

	foreign = env.getStory(t, carlaToken, storyID.String())
	missing = env.getStory(t, anaToken, uuid.New().String())
	return foreign, missing
}

// backlogRequireIdenticalNotFound exige que la respuesta sea el 404 de historia inexistente del
// contrato e idéntica byte a byte a la de referencia en estado, tipo de contenido y cuerpo (FR-047,
// SC-006).
func backlogRequireIdenticalNotFound(t *testing.T, got, reference *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusNotFound, got.Code, "cuerpo: %s", got.Body.String())
	assert.Equal(t, storyNotFoundBody, got.Body.String(), "el cuerpo debería ser el del contrato")
	assert.Equal(t, reference.Code, got.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, reference.Header().Get("Content-Type"), got.Header().Get("Content-Type"),
		"los tipos de contenido deberían ser idénticos")
	assert.Equal(t, reference.Body.Bytes(), got.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
}

// Escenario: Detalle completo (003/US3)
// Bruno, integrante que no es el propietario, consulta una historia con descripción, prioridad
// Media, 8 Story Points y tres criterios sembrados fuera de orden: obtiene todos sus datos, los
// criterios en el orden de su posición, el estado, la autora, los momentos de creación y de última
// actualización, que no estuvo en un sprint y que está lista para planificar (FR-039, RN-01, RN-02).
func TestUS3_DetalleCompleto(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	description := backlogStoryDescription
	storyID := backlogSeedStory(t, env.db, domain.UserStory{
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Description: &description,
		Priority:    domain.StoryPriorityMedium,
		StoryPoints: backlogPoints(8),
		CreatedByID: anaID,
		CreatedAt:   backlogDetailCreatedAt,
		UpdatedAt:   backlogDetailUpdatedAt,
	},
		domain.AcceptanceCriterion{Position: 2, Text: backlogThirdCriterion},
		domain.AcceptanceCriterion{Position: 0, Text: backlogFirstCriterion},
		domain.AcceptanceCriterion{Position: 1, Text: backlogSecondCriterion},
	)

	got := requireStoryDetail(t, env.getStory(t, brunoToken, storyID.String()))

	assert.Equal(t, storyID.String(), got.ID)
	assert.Equal(t, projectID.String(), got.ProjectID)
	assert.Equal(t, backlogStoryTitle, got.Title)
	require.NotNil(t, got.Description, "la descripción no debería ser null")
	assert.Equal(t, backlogStoryDescription, *got.Description)
	assert.Equal(t, "medium", got.Priority)
	assert.Equal(t, "pending", got.Status)
	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 8, *got.StoryPoints)
	assert.Equal(t, []string{backlogFirstCriterion, backlogSecondCriterion, backlogThirdCriterion}, got.AcceptanceCriteria,
		"los criterios deberían venir en el orden en que se cargaron")
	assert.Equal(t, anaID.String(), got.CreatedBy.ID)
	assert.Equal(t, "Ana Pérez", got.CreatedBy.FullName)
	assert.Equal(t, "2026-10-20T13:05:00Z", got.CreatedAt)
	assert.Equal(t, "2026-10-20T14:30:00Z", got.UpdatedAt)
	assert.True(t, got.Readiness.Ready, "la historia debería estar lista para planificar")
	assert.Equal(t, []string{}, got.Readiness.Missing)
	assert.False(t, got.EverAssignedToSprint)
}

// Escenario: Sin estimar ni criterios (003/US3)
// Una historia sin estimar y sin criterios se consulta sin error: storyPoints es null y no 0, la
// lista de criterios es [] y no null, y no está lista para planificar, con los dos motivos en orden
// (FR-029, FR-040).
func TestUS3_SinEstimarNiCriterios(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedStory(t, env.db, domain.UserStory{
		ProjectID:   projectID,
		Title:       "Revisar los permisos de los integrantes",
		Priority:    domain.StoryPriorityLow,
		CreatedByID: anaID,
	})

	rec := env.getStory(t, anaToken, storyID.String())

	got := requireStoryDetail(t, rec)
	raw := backlogRawBody(t, rec)
	value, present := raw["storyPoints"]
	assert.True(t, present, "el campo storyPoints debería estar presente")
	assert.Nil(t, value, "storyPoints debería ser null")
	assert.Equal(t, []any{}, raw["acceptanceCriteria"], "acceptanceCriteria debería ser [] y no null")
	assert.False(t, got.Readiness.Ready, "la historia no debería estar lista para planificar")
	assert.Equal(t, []string{"story_points", "acceptance_criteria"}, got.Readiness.Missing)
}

// Escenario: Asignada a un sprint (003/US3)
// Con el puerto de sprints reemplazado por un doble que informa que la historia está comprometida en
// un sprint abierto y estuvo asignada, el detalle informa everAssignedToSprint verdadero además de
// sus datos (FR-039).
func TestUS3_AsignadaASprint(t *testing.T) {
	sprints := &backlogSprintAssignmentsDouble{}
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) { p.sprints = sprints })
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedStory(t, env.db, domain.UserStory{
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Priority:    domain.StoryPriorityHigh,
		StoryPoints: backlogPoints(5),
		CreatedByID: anaID,
	}, domain.AcceptanceCriterion{Position: 0, Text: backlogFirstCriterion})
	sprints.On("Find", mock.Anything, storyID).
		Return(domain.SprintAssignment{CommittedToOpenSprint: true, EverAssigned: true}, nil)

	got := requireStoryDetail(t, env.getStory(t, anaToken, storyID.String()))

	assert.Equal(t, storyID.String(), got.ID)
	assert.Equal(t, backlogStoryTitle, got.Title)
	assert.True(t, got.EverAssignedToSprint, "el detalle debería informar que estuvo en un sprint")
	sprints.AssertExpectations(t)
}

// Escenario: Cero Story Points (003/US3)
// Una historia estimada en 0 Story Points con un criterio devuelve storyPoints: 0, distinguible de
// null, y está lista para planificar (FR-015, RN-07).
func TestUS3_CeroStoryPoints(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedStory(t, env.db, domain.UserStory{
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Priority:    domain.StoryPriorityHigh,
		StoryPoints: backlogPoints(0),
		CreatedByID: anaID,
	}, domain.AcceptanceCriterion{Position: 0, Text: backlogFirstCriterion})

	rec := env.getStory(t, anaToken, storyID.String())

	got := requireStoryDetail(t, rec)
	raw := backlogRawBody(t, rec)
	assert.Equal(t, float64(0), raw["storyPoints"], "storyPoints debería ser 0 y no null")
	assert.True(t, got.Readiness.Ready, "con 0 Story Points y un criterio la historia está lista")
	assert.Equal(t, []string{}, got.Readiness.Missing)
}

// Escenario: Historia ajena (003/US3)
// Carla, que no es integrante del proyecto, pide una historia que existe: recibe el 404 de historia
// inexistente, sin ningún dato ni el título, idéntico byte a byte al de un identificador que no
// existe (FR-047, SC-006).
func TestUS3_HistoriaAjena(t *testing.T) {
	foreign, missing := backlogForeignAndMissing(t)

	backlogRequireIdenticalNotFound(t, foreign, missing)
	assert.NotContains(t, foreign.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
}

// Escenario: Historia inexistente (003/US3)
// Un integrante pide un identificador de historia que no existe: recibe el 404 de historia
// inexistente, idéntico byte a byte al de la historia de un proyecto ajeno (FR-044, FR-047).
func TestUS3_HistoriaInexistente(t *testing.T) {
	foreign, missing := backlogForeignAndMissing(t)

	backlogRequireIdenticalNotFound(t, missing, foreign)
}

// Bruno crea una historia y Ana, propietaria, lo quita del proyecto con el endpoint de la 002: el
// detalle consultado por Ana sigue devolviendo la historia con Bruno como autor, y Bruno, que ya no
// es integrante, recibe el 404 de historia inexistente (FR-050, caso límite "Integrante quitado del
// proyecto"). No lleva etiqueta de escenario porque deriva de un caso límite y no de un escenario.
func TestAutoriaConservadaAlQuitarIntegrante(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	created := requireCreatedStory(t, env.postStory(t, brunoToken, projectID, backlogFullBody()))

	// Ana quita a Bruno del proyecto con DELETE /api/v1/projects/{projectId}/members/{userId}.
	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/projects/"+projectID.String()+"/members/"+brunoID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+anaToken)
	removal := httptest.NewRecorder()
	env.router.ServeHTTP(removal, req)
	require.Equal(t, http.StatusNoContent, removal.Code, "no se pudo quitar a Bruno: %s", removal.Body.String())

	// Para Ana la historia se conserva con la autoría de Bruno.
	got := requireStoryDetail(t, env.getStory(t, anaToken, created.ID))
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, brunoID.String(), got.CreatedBy.ID)
	assert.Equal(t, "Bruno Gómez", got.CreatedBy.FullName)

	// Para Bruno la historia se comporta como inexistente.
	rec := env.getStory(t, brunoToken, created.ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, storyNotFoundBody, rec.Body.String())
}

// Pruebas de integración de US2 (T030): GET /api/v1/projects/{projectId}/stories. Las historias se
// siembran directamente en la base, con su estado, su momento de creación y sus criterios, porque
// ningún endpoint de esta feature produce los estados in_progress y completed ni fija el created_at
// (plan.md, cómo se preparan los escenarios).

// backlogSeedBase es el instante de referencia de las historias sembradas; cada una se ubica a una
// cantidad de minutos de él para fijar el orden por fecha de creación.
var backlogSeedBase = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

// backlogListItemBody refleja StoryListItem del contrato. StoryPoints es un puntero para distinguir
// el 0 de "sin estimar".
type backlogListItemBody struct {
	ID                      string `json:"id"`
	Title                   string `json:"title"`
	Priority                string `json:"priority"`
	Status                  string `json:"status"`
	StoryPoints             *int   `json:"storyPoints"`
	AcceptanceCriteriaCount int    `json:"acceptanceCriteriaCount"`
	ReadyToPlan             bool   `json:"readyToPlan"`
}

// backlogListBody refleja StoryListResponse del contrato; EmptyReason es nil cuando hay historias.
type backlogListBody struct {
	Stories        []backlogListItemBody `json:"stories"`
	AppliedFilters struct {
		Status   []string `json:"status"`
		Priority []string `json:"priority"`
	} `json:"appliedFilters"`
	EmptyReason *string `json:"emptyReason"`
}

// backlogSeed describe una historia a sembrar: prioridad, estado, Story Points (nil es "sin
// estimar"), cantidad de criterios y minutos desde backlogSeedBase. Si ID es uuid.Nil se genera uno.
type backlogSeed struct {
	ID          uuid.UUID
	Title       string
	Priority    domain.StoryPriority
	Status      domain.StoryStatus
	StoryPoints *int
	Criteria    int
	Minute      int
}

// backlogIntPtr devuelve un puntero al entero indicado, para los Story Points sembrados.
func backlogIntPtr(v int) *int {
	return &v
}

// backlogSeedStories inserta las historias en user_stories, con el created_at fijado por la prueba,
// y sus criterios en acceptance_criteria, y devuelve los identificadores en el orden recibido.
func backlogSeedStories(t *testing.T, db *gorm.DB, projectID, creatorID uuid.UUID, seeds ...backlogSeed) []string {
	t.Helper()
	ids := make([]string, 0, len(seeds))
	for i, seed := range seeds {
		id := seed.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		title := seed.Title
		if title == "" {
			title = fmt.Sprintf("Historia sembrada %d", i+1)
		}
		status := seed.Status
		if status == "" {
			status = domain.StoryStatusPending
		}
		createdAt := backlogSeedBase.Add(time.Duration(seed.Minute) * time.Minute)
		story := domain.UserStory{
			ID:          id,
			ProjectID:   projectID,
			Title:       title,
			Priority:    seed.Priority,
			Status:      status,
			StoryPoints: seed.StoryPoints,
			CreatedByID: creatorID,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		}
		require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia %q", title)
		for p := 0; p < seed.Criteria; p++ {
			criterion := domain.AcceptanceCriterion{ID: uuid.New(), StoryID: id, Position: p, Text: fmt.Sprintf("Criterio %d", p+1)}
			require.NoError(t, db.Create(&criterion).Error, "no se pudo sembrar el criterio de %q", title)
		}
		ids = append(ids, id.String())
	}
	return ids
}

// getStories envía GET /api/v1/projects/{projectId}/stories con la consulta cruda indicada (sin el
// signo de pregunta) y el token como Bearer.
func (e backlogTestEnv) getStories(t *testing.T, token string, projectID uuid.UUID, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	target := backlogStoriesPath(projectID)
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// requireStoryList exige un 200 cuyo cuerpo sea un StoryListResponse con stories y los dos filtros
// aplicados como listas (nunca null) y emptyReason presente, y lo devuelve decodificado.
func requireStoryList(t *testing.T, rec *httptest.ResponseRecorder) backlogListBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	raw := backlogRawBody(t, rec)
	require.IsType(t, []any{}, raw["stories"], "stories debería ser una lista y no null: %s", rec.Body.String())
	applied, ok := raw["appliedFilters"].(map[string]any)
	require.True(t, ok, "appliedFilters debería ser un objeto: %s", rec.Body.String())
	require.IsType(t, []any{}, applied["status"], "appliedFilters.status debería ser una lista y no null")
	require.IsType(t, []any{}, applied["priority"], "appliedFilters.priority debería ser una lista y no null")
	_, present := raw["emptyReason"]
	require.True(t, present, "emptyReason debería estar presente aunque valga null")
	var body backlogListBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body
}

// backlogListIDs devuelve los identificadores del listado en el orden recibido.
func backlogListIDs(body backlogListBody) []string {
	ids := make([]string, 0, len(body.Stories))
	for _, story := range body.Stories {
		ids = append(ids, story.ID)
	}
	return ids
}

// backlogRequireEmptyReason exige el motivo de lista vacía indicado, o null si want es "".
func backlogRequireEmptyReason(t *testing.T, body backlogListBody, want string) {
	t.Helper()
	if want == "" {
		assert.Nil(t, body.EmptyReason, "emptyReason debería ser null cuando hay historias")
		return
	}
	require.NotNil(t, body.EmptyReason, "emptyReason debería ser %q y no null", want)
	assert.Equal(t, want, *body.EmptyReason)
}

// Escenario: Orden por prioridad y fecha (003/US2)
// Con historias Alta, Media y Baja creadas en distinto orden, el integrante ve primero las Altas,
// después las Medias y después las Bajas, y dentro de cada prioridad las más antiguas primero
// (FR-032, RN-17). Cada elemento trae los datos de FR-033, sin filtros aplicados y con emptyReason
// null.
func TestUS2_OrdenPorPrioridadYFecha(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress, brunoID)
	ids := backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Title: "Baja reciente", Priority: domain.StoryPriorityLow, Minute: 40},
		backlogSeed{Title: "Alta reciente", Priority: domain.StoryPriorityHigh, Minute: 30, StoryPoints: backlogIntPtr(5), Criteria: 2},
		backlogSeed{Title: "Media", Priority: domain.StoryPriorityMedium, Minute: 20, StoryPoints: backlogIntPtr(0), Criteria: 1},
		backlogSeed{Title: "Baja antigua", Priority: domain.StoryPriorityLow, Minute: 0, StoryPoints: backlogIntPtr(3)},
		backlogSeed{Title: "Alta antigua", Priority: domain.StoryPriorityHigh, Minute: 10},
	)

	body := requireStoryList(t, env.getStories(t, brunoToken, projectID, ""))

	// El orden es Alta antigua, Alta reciente, Media, Baja antigua, Baja reciente.
	assert.Equal(t, []string{ids[4], ids[1], ids[2], ids[3], ids[0]}, backlogListIDs(body))
	assert.Empty(t, body.AppliedFilters.Status)
	assert.Empty(t, body.AppliedFilters.Priority)
	backlogRequireEmptyReason(t, body, "")

	// Cada elemento informa sus datos, Story Points o null, cantidad de criterios y si está lista.
	require.Len(t, body.Stories, 5)
	byTitle := map[string]backlogListItemBody{}
	for _, story := range body.Stories {
		byTitle[story.Title] = story
	}
	ready := byTitle["Alta reciente"]
	assert.Equal(t, "high", ready.Priority)
	assert.Equal(t, "pending", ready.Status)
	require.NotNil(t, ready.StoryPoints)
	assert.Equal(t, 5, *ready.StoryPoints)
	assert.Equal(t, 2, ready.AcceptanceCriteriaCount)
	assert.True(t, ready.ReadyToPlan, "con Story Points y criterios la historia está lista")
	zero := byTitle["Media"]
	require.NotNil(t, zero.StoryPoints, "0 Story Points no debería informarse como null")
	assert.Equal(t, 0, *zero.StoryPoints)
	assert.Equal(t, 1, zero.AcceptanceCriteriaCount)
	assert.True(t, zero.ReadyToPlan, "con 0 Story Points y un criterio la historia está lista")
	noCriteria := byTitle["Baja antigua"]
	assert.Equal(t, 0, noCriteria.AcceptanceCriteriaCount)
	assert.False(t, noCriteria.ReadyToPlan, "sin criterios la historia no está lista")
	unestimated := byTitle["Alta antigua"]
	assert.Nil(t, unestimated.StoryPoints, "sin estimar debería informarse como null")
	assert.False(t, unestimated.ReadyToPlan)
}

// Escenario: Filtro por estado (003/US2)
// En un backlog con historias en los tres estados, sembrados en la base, filtrar por Pendiente
// devuelve solo las Pendientes, en el mismo orden de prioridad y fecha, y lo informa en
// appliedFilters (FR-034).
func TestUS2_FiltroPorEstado(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	ids := backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Priority: domain.StoryPriorityLow, Status: domain.StoryStatusPending, Minute: 0},
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusInProgress, Minute: 1},
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 2},
		backlogSeed{Priority: domain.StoryPriorityMedium, Status: domain.StoryStatusCompleted, Minute: 3},
		backlogSeed{Priority: domain.StoryPriorityMedium, Status: domain.StoryStatusPending, Minute: 4},
	)

	body := requireStoryList(t, env.getStories(t, token, projectID, "status=pending"))

	assert.Equal(t, []string{ids[2], ids[4], ids[0]}, backlogListIDs(body))
	for _, story := range body.Stories {
		assert.Equal(t, "pending", story.Status)
	}
	assert.Equal(t, []string{"pending"}, body.AppliedFilters.Status)
	assert.Empty(t, body.AppliedFilters.Priority)
	backlogRequireEmptyReason(t, body, "")
}

// Escenario: Filtros combinados (003/US2)
// Filtrar a la vez por prioridad Alta y por estado Pendiente devuelve solo las historias que cumplen
// ambas condiciones (FR-034).
func TestUS2_FiltrosCombinados(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	ids := backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 5},
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusCompleted, Minute: 1},
		backlogSeed{Priority: domain.StoryPriorityMedium, Status: domain.StoryStatusPending, Minute: 2},
		backlogSeed{Priority: domain.StoryPriorityLow, Status: domain.StoryStatusInProgress, Minute: 3},
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 0},
	)

	body := requireStoryList(t, env.getStories(t, token, projectID, "priority=high&status=pending"))

	assert.Equal(t, []string{ids[4], ids[0]}, backlogListIDs(body))
	assert.Equal(t, []string{"pending"}, body.AppliedFilters.Status)
	assert.Equal(t, []string{"high"}, body.AppliedFilters.Priority)
	backlogRequireEmptyReason(t, body, "")
}

// Escenario: Backlog vacío (003/US2)
// Un proyecto sin historias responde 200 con stories: [] y emptyReason backlog_empty, sin que sea un
// error (FR-035). Aunque la consulta lleve filtros, el motivo sigue siendo backlog_empty, porque el
// proyecto no tiene ninguna historia.
func TestUS2_BacklogVacio(t *testing.T) {
	cases := []struct {
		name         string
		rawQuery     string
		wantStatus   []string
		wantPriority []string
	}{
		{"sin filtros", "", []string{}, []string{}},
		{"con filtros", "status=pending&priority=high", []string{"pending"}, []string{"high"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusPlanned)

			body := requireStoryList(t, env.getStories(t, token, projectID, tc.rawQuery))

			assert.Empty(t, body.Stories)
			backlogRequireEmptyReason(t, body, "backlog_empty")
			assert.Equal(t, tc.wantStatus, body.AppliedFilters.Status)
			assert.Equal(t, tc.wantPriority, body.AppliedFilters.Priority)
		})
	}
}

// Escenario: Filtro sin coincidencias (003/US2)
// En un backlog que solo tiene historias de prioridad Baja, filtrar por Alta responde 200 con la
// lista vacía, emptyReason no_matches y el detalle de los filtros aplicados, distinguible del
// backlog vacío (FR-036).
func TestUS2_FiltroSinCoincidencias(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Priority: domain.StoryPriorityLow, Minute: 0},
		backlogSeed{Priority: domain.StoryPriorityLow, Minute: 1},
	)

	body := requireStoryList(t, env.getStories(t, token, projectID, "priority=high"))

	assert.Empty(t, body.Stories)
	backlogRequireEmptyReason(t, body, "no_matches")
	assert.Empty(t, body.AppliedFilters.Status)
	assert.Equal(t, []string{"high"}, body.AppliedFilters.Priority)
}

// Escenario: Orden estable ante empate (003/US2)
// Dos historias de la misma prioridad con el mismo created_at escrito en la base aparecen siempre en
// el mismo orden en dos consultas seguidas, desempatadas por id ascendente (FR-032). La de id mayor
// se inserta primero para que el orden de inserción no coincida con el esperado.
func TestUS2_OrdenEstableAnteEmpate(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	higherID := uuid.MustParse("ffffffff-0000-4000-8000-000000000001")
	lowerID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{ID: higherID, Priority: domain.StoryPriorityMedium, Minute: 3},
		backlogSeed{ID: lowerID, Priority: domain.StoryPriorityMedium, Minute: 7},
	)
	tie := backlogSeedBase.Add(15 * time.Minute)
	require.NoError(t, env.db.Exec("UPDATE user_stories SET created_at = ? WHERE id IN ?", tie, []uuid.UUID{higherID, lowerID}).Error,
		"no se pudo igualar el created_at de las historias")
	require.Equal(t, int64(2), backlogCountRows(t, env.db, &domain.UserStory{}, "created_at = ?", tie),
		"las dos historias deberían tener el mismo created_at")

	first := requireStoryList(t, env.getStories(t, token, projectID, ""))
	second := requireStoryList(t, env.getStories(t, token, projectID, ""))

	want := []string{lowerID.String(), higherID.String()}
	assert.Equal(t, want, backlogListIDs(first), "el empate se desempata por id ascendente")
	assert.Equal(t, want, backlogListIDs(second), "la segunda consulta debe devolver el mismo orden")
}

// Escenario: Filtro inválido (003/US2)
// Un estado o una prioridad que no existe se rechaza con 400 indicando los valores admitidos para
// ese filtro, también cuando llega junto a un valor válido (FR-037).
func TestUS2_FiltroInvalido(t *testing.T) {
	cases := []struct {
		name     string
		rawQuery string
		field    string
		admitted []string
	}{
		{"estado inexistente", "status=done", "status", []string{"pending", "in_progress", "completed"}},
		{"estado inválido junto a uno válido", "status=pending&status=archived", "status", []string{"pending", "in_progress", "completed"}},
		{"prioridad inexistente", "priority=urgent", "priority", []string{"high", "medium", "low"}},
		{"prioridad en español", "priority=alta", "priority", []string{"high", "medium", "low"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			backlogSeedStories(t, env.db, projectID, ownerID, backlogSeed{Priority: domain.StoryPriorityHigh})

			rec := env.getStories(t, token, projectID, tc.rawQuery)

			backlogRequireFieldRejected(t, rec, tc.field, tc.admitted...)
		})
	}
}

// Escenario: No integrante (003/US2)
// Quien no es integrante recibe 404 de proyecto inexistente, idéntico byte a byte al de un
// projectId que no existe, y la respuesta no revela ninguna historia (FR-047, SC-006).
func TestUS2_NoIntegrante(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	ids := backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Title: backlogStoryTitle, Priority: domain.StoryPriorityHigh})

	foreignRec := env.getStories(t, outsiderToken, projectID, "")
	missingRec := env.getStories(t, outsiderToken, uuid.New(), "")

	assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
	assert.JSONEq(t, `{"error":"`+backlogProjectNotFound+`"}`, foreignRec.Body.String())
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	assert.NotContains(t, foreignRec.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
	assert.NotContains(t, foreignRec.Body.String(), ids[0], "la respuesta no debería revelar el identificador")
}

// backlogPriorityRank es el rango de orden de cada prioridad según RN-17, declarado en la prueba
// para calcular el orden esperado sin depender del código de producción.
var backlogPriorityRank = map[domain.StoryPriority]int{
	domain.StoryPriorityHigh:   0,
	domain.StoryPriorityMedium: 1,
	domain.StoryPriorityLow:    2,
}

// TestStoryList_OrdenConCincuentaHistorias verifica el orden de FR-032 sobre un backlog de 50
// historias de las tres prioridades (SC-004). Las fechas se reparten en siete instantes para que
// haya empates de prioridad y fecha, que se desempatan por id. El orden esperado se calcula en la
// prueba ordenando los datos sembrados por rango, fecha e id.
func TestStoryList_OrdenConCincuentaHistorias(t *testing.T) {
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	priorities := []domain.StoryPriority{domain.StoryPriorityLow, domain.StoryPriorityHigh, domain.StoryPriorityMedium}
	seeds := make([]backlogSeed, 0, 50)
	for i := 0; i < 50; i++ {
		seeds = append(seeds, backlogSeed{Priority: priorities[i%3], Minute: (i * 5) % 7})
	}
	ids := backlogSeedStories(t, env.db, projectID, ownerID, seeds...)

	// Orden esperado: rango de prioridad, minuto de creación e id ascendentes.
	order := make([]int, len(seeds))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		sa, sb := seeds[order[a]], seeds[order[b]]
		if ra, rb := backlogPriorityRank[sa.Priority], backlogPriorityRank[sb.Priority]; ra != rb {
			return ra < rb
		}
		if sa.Minute != sb.Minute {
			return sa.Minute < sb.Minute
		}
		return ids[order[a]] < ids[order[b]]
	})
	want := make([]string, 0, len(order))
	for _, i := range order {
		want = append(want, ids[i])
	}

	body := requireStoryList(t, env.getStories(t, token, projectID, ""))

	assert.Equal(t, want, backlogListIDs(body))
}

// TestStoryList_SinHistoriasDeOtroProyecto verifica que el listado de un proyecto no incluye
// historias de otro, aunque quien consulta sea integrante de ambos (FR-038).
func TestStoryList_SinHistoriasDeOtroProyecto(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	ownProject := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	otherProject := backlogSeedProject(t, env.db, brunoID, projectsdomain.ProjectStatusInProgress, anaID)
	ownIDs := backlogSeedStories(t, env.db, ownProject, anaID,
		backlogSeed{Priority: domain.StoryPriorityHigh, Minute: 0},
		backlogSeed{Priority: domain.StoryPriorityLow, Minute: 1},
	)
	otherIDs := backlogSeedStories(t, env.db, otherProject, brunoID,
		backlogSeed{Priority: domain.StoryPriorityHigh, Minute: 0},
		backlogSeed{Priority: domain.StoryPriorityMedium, Minute: 1},
	)

	body := requireStoryList(t, env.getStories(t, anaToken, ownProject, ""))

	assert.Equal(t, ownIDs, backlogListIDs(body))
	for _, id := range otherIDs {
		assert.NotContains(t, backlogListIDs(body), id, "no debería aparecer una historia de otro proyecto")
	}
}

// TestStoryList_VariosValoresPorFiltro verifica que un mismo filtro admite varios valores, que se
// combinan con "o": dos estados devuelven las historias de ambos y ninguna completed, y dos
// prioridades devuelven las Altas y las Bajas sin las Medias (FR-034).
func TestStoryList_VariosValoresPorFiltro(t *testing.T) {
	seeds := []backlogSeed{
		{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 0},
		{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusCompleted, Minute: 1},
		{Priority: domain.StoryPriorityMedium, Status: domain.StoryStatusInProgress, Minute: 2},
		{Priority: domain.StoryPriorityMedium, Status: domain.StoryStatusPending, Minute: 3},
		{Priority: domain.StoryPriorityLow, Status: domain.StoryStatusInProgress, Minute: 4},
		{Priority: domain.StoryPriorityLow, Status: domain.StoryStatusCompleted, Minute: 5},
	}
	cases := []struct {
		name         string
		rawQuery     string
		wantIndexes  []int
		wantStatus   []string
		wantPriority []string
	}{
		{"pending e in_progress", "status=pending&status=in_progress", []int{0, 2, 3, 4},
			[]string{"pending", "in_progress"}, []string{}},
		{"high y low", "priority=high&priority=low", []int{0, 1, 4, 5},
			[]string{}, []string{"high", "low"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			ids := backlogSeedStories(t, env.db, projectID, ownerID, seeds...)

			body := requireStoryList(t, env.getStories(t, token, projectID, tc.rawQuery))

			want := make([]string, 0, len(tc.wantIndexes))
			for _, i := range tc.wantIndexes {
				want = append(want, ids[i])
			}
			assert.Equal(t, want, backlogListIDs(body))
			assert.ElementsMatch(t, tc.wantStatus, body.AppliedFilters.Status)
			assert.ElementsMatch(t, tc.wantPriority, body.AppliedFilters.Priority)
			backlogRequireEmptyReason(t, body, "")
		})
	}
}

// TestStoryList_EstadoRepetidoMilesDeVeces verifica que un listado con miles de status=pending
// responde 200, no 500, con solo las historias pendientes y el filtro aplicado una sola vez, porque
// la cláusula IN debe recibir cada valor admitido una vez y no un parámetro por repetición (T085,
// hallazgo de seguridad: filtros repetibles sin tope). Las repeticiones quedan por debajo del tope de
// 10000 parámetros de consulta de net/url, para que lleguen todas al manejador.
func TestStoryList_EstadoRepetidoMilesDeVeces(t *testing.T) {
	const repetitions = 9000
	env := newBacklogTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
	ids := backlogSeedStories(t, env.db, projectID, ownerID,
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 0},
		backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusCompleted, Minute: 1},
	)
	rawQuery := strings.TrimSuffix(strings.Repeat("status=pending&", repetitions), "&")

	body := requireStoryList(t, env.getStories(t, token, projectID, rawQuery))

	// Solo se lista la pendiente y el filtro informado contiene pending una única vez.
	assert.Equal(t, []string{ids[0]}, backlogListIDs(body))
	appliedCounts := map[string]int{}
	for _, status := range body.AppliedFilters.Status {
		appliedCounts[status]++
	}
	assert.Equal(t, map[string]int{"pending": 1}, appliedCounts, "el filtro aplicado debe informar pending una sola vez")
	assert.Empty(t, body.AppliedFilters.Priority)
}

// Sin etiqueta de escenario: un listado del backlog cuya query no se puede interpretar responde 400
// con un mensaje en el campo error y sin listado, en lugar de 200 con todas las historias porque el
// filtro de estado se descartó sin aviso (T104, hallazgo de seguridad: query mal formada que ignora
// los filtros sin aviso). Cada subtest es una forma de query que url.ParseQuery rechaza: más de
// 10000 parámetros, un escape inválido y el separador ";".
func TestStoryList_QueryMalFormada(t *testing.T) {
	cases := []struct {
		name     string
		rawQuery string
	}{
		{name: "mas de 10000 parametros status", rawQuery: strings.TrimSuffix(strings.Repeat("status=pending&", 10001), "&")},
		{name: "escape invalido", rawQuery: "status=%zz"},
		{name: "separador punto y coma", rawQuery: "status=pending;x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress)
			backlogSeedStories(t, env.db, projectID, ownerID,
				backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusPending, Minute: 0},
				backlogSeed{Priority: domain.StoryPriorityHigh, Status: domain.StoryStatusCompleted, Minute: 1},
			)

			rec := env.getStories(t, token, projectID, tc.rawQuery)

			// Se rechaza con 400 y un motivo, sin devolver ningún listado.
			require.Equal(t, http.StatusBadRequest, rec.Code, "la query mal formada no debe listar todas las historias: %s", rec.Body.String())
			raw := backlogRawBody(t, rec)
			message, _ := raw["error"].(string)
			assert.NotEmpty(t, strings.TrimSpace(message), "el 400 debe explicar el motivo en el campo error: %s", rec.Body.String())
			assert.NotContains(t, raw, "stories", "el 400 no debe incluir un listado")
		})
	}
}

// Pruebas de integración de US4 (T049): PUT /api/v1/stories/{storyId}/story-points. Las historias se
// siembran directamente en la base, con su estado, su estimación previa y un momento de última
// actualización fijo en el pasado, para comprobar que el valor vigente y ese momento cambian o se
// conservan según el escenario.

// Mensajes exactos del 409 que fija el contrato para la estimación de una historia completada y para
// devolver a "sin estimar" una historia comprometida en un sprint abierto.
const (
	backlogCompletedMessage = "La historia está completada y no se puede modificar ni reestimar."
	backlogCommittedMessage = "La historia está comprometida en un sprint abierto; primero hay que quitarla del sprint."
)

// backlogEstimateUpdatedAt es el momento de creación y de última actualización con que se siembran
// las historias de US4: queda en el pasado para que una estimación aceptada lo deba mover y una
// rechazada no.
var backlogEstimateUpdatedAt = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// putStoryPoints envía el cuerpo JSON literal a PUT /api/v1/stories/{storyId}/story-points con el
// token como Bearer. El cuerpo va como texto para poder enviar null, decimales y valores intermedios
// tal cual.
func (e backlogTestEnv) putStoryPoints(t *testing.T, token, storyID, rawBody string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, backlogStoryPath(storyID)+"/story-points", strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// backlogSeedEstimable siembra en el proyecto una historia de la autora con el estado, la estimación
// y la cantidad de criterios indicados (hasta tres), con el momento fijo de US4, y devuelve su
// identificador.
func backlogSeedEstimable(t *testing.T, db *gorm.DB, projectID, authorID uuid.UUID, status domain.StoryStatus, points *int, criteriaCount int) uuid.UUID {
	t.Helper()
	texts := []string{backlogFirstCriterion, backlogSecondCriterion, backlogThirdCriterion}
	criteria := make([]domain.AcceptanceCriterion, 0, criteriaCount)
	for i := 0; i < criteriaCount; i++ {
		criteria = append(criteria, domain.AcceptanceCriterion{Position: i, Text: texts[i]})
	}
	return backlogSeedStory(t, db, domain.UserStory{
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Priority:    domain.StoryPriorityHigh,
		Status:      status,
		StoryPoints: points,
		CreatedByID: authorID,
		CreatedAt:   backlogEstimateUpdatedAt,
		UpdatedAt:   backlogEstimateUpdatedAt,
	}, criteria...)
}

// backlogRequireEstimateUnchanged exige que la fila de la historia conserve la estimación indicada y
// el momento de última actualización con que se sembró (FR-019).
func backlogRequireEstimateUnchanged(t *testing.T, db *gorm.DB, storyID uuid.UUID, want *int) {
	t.Helper()
	stored := backlogStoredStory(t, db, storyID.String())
	assert.Equal(t, want, stored.StoryPoints, "la estimación vigente no debería cambiar")
	assert.True(t, backlogEstimateUpdatedAt.Equal(stored.UpdatedAt),
		"updated_at no debería cambiar: quedó en %s", stored.UpdatedAt)
}

// backlogRequireConflict exige un 409 con el cuerpo de error exacto del contrato.
func backlogRequireConflict(t *testing.T, rec *httptest.ResponseRecorder, message string) {
	t.Helper()
	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+message+`"}`, rec.Body.String())
}

// Escenario: Estimación inicial (003/US4)
// Bruno, integrante que no es el propietario, estima en 13 una historia sin estimar con dos
// criterios: la respuesta es el detalle con 13 Story Points y la historia lista para planificar, sin
// cambiar su estado, y la base guarda 13 (FR-016, RN-12).
func TestUS4_EstimacionInicial(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 2)

	got := requireStoryDetail(t, env.putStoryPoints(t, brunoToken, storyID.String(), `{"storyPoints":13}`))

	assert.Equal(t, storyID.String(), got.ID)
	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 13, *got.StoryPoints)
	assert.Equal(t, []string{backlogFirstCriterion, backlogSecondCriterion}, got.AcceptanceCriteria)
	assert.True(t, got.Readiness.Ready, "la historia debería estar lista para planificar")
	assert.Equal(t, []string{}, got.Readiness.Missing)
	assert.Equal(t, "pending", got.Status, "estimar no cambia el estado")
	stored := backlogStoredStory(t, env.db, storyID.String())
	require.NotNil(t, stored.StoryPoints, "story_points no debería ser NULL")
	assert.Equal(t, 13, *stored.StoryPoints)
}

// Escenario: Reestimación (003/US4)
// Una historia estimada en 5 se reestima en 8: el valor anterior se reemplaza por 8 y el momento de
// última actualización queda posterior al que tenía, en la respuesta y en la base (FR-017).
func TestUS4_Reestimacion(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 1)

	got := requireStoryDetail(t, env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":8}`))

	// La respuesta trae el valor nuevo y un updatedAt posterior, con el createdAt intacto.
	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 8, *got.StoryPoints)
	updatedAt, err := time.Parse(time.RFC3339, got.UpdatedAt)
	require.NoError(t, err, "updatedAt = %q no es una fecha y hora RFC 3339", got.UpdatedAt)
	assert.True(t, updatedAt.After(backlogEstimateUpdatedAt), "updatedAt = %s debería ser posterior al anterior", got.UpdatedAt)
	assert.Equal(t, "2026-10-01T12:00:00Z", got.CreatedAt, "el momento de creación no cambia")

	// La base guarda el valor nuevo y el updated_at posterior.
	stored := backlogStoredStory(t, env.db, storyID.String())
	require.NotNil(t, stored.StoryPoints)
	assert.Equal(t, 8, *stored.StoryPoints)
	assert.True(t, stored.UpdatedAt.After(backlogEstimateUpdatedAt), "updated_at debería ser posterior al anterior")
}

// Escenario: Volver a sin estimar (003/US4)
// Una historia estimada que no está comprometida en ningún sprint se devuelve a "sin estimar" con
// null: el cambio se acepta, storyPoints vale null y la historia deja de estar lista para planificar
// por la falta de Story Points (FR-018, RN-12).
func TestUS4_VolverASinEstimar(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)

	rec := env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":null}`)

	got := requireStoryDetail(t, rec)
	raw := backlogRawBody(t, rec)
	value, present := raw["storyPoints"]
	assert.True(t, present, "el campo storyPoints debería estar presente")
	assert.Nil(t, value, "storyPoints debería ser null")
	assert.False(t, got.Readiness.Ready, "la historia no debería estar lista para planificar")
	assert.Equal(t, []string{"story_points"}, got.Readiness.Missing)
	assert.Nil(t, backlogStoredStory(t, env.db, storyID.String()).StoryPoints, "story_points debería ser NULL")
}

// Escenario: Sin criterios (003/US4)
// Una historia sin criterios se estima en 3: la estimación se registra, pero la historia sigue sin
// estar lista para planificar porque le falta al menos un criterio (RN-12, FR-028).
func TestUS4_SinCriterios(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 0)

	got := requireStoryDetail(t, env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":3}`))

	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 3, *got.StoryPoints)
	assert.False(t, got.Readiness.Ready, "sin criterios la historia no está lista para planificar")
	assert.Equal(t, []string{"acceptance_criteria"}, got.Readiness.Missing)
	stored := backlogStoredStory(t, env.db, storyID.String())
	require.NotNil(t, stored.StoryPoints)
	assert.Equal(t, 3, *stored.StoryPoints)
}

// Escenario: Extremos de la escala (003/US4)
// Una historia sin estimar acepta los dos extremos de la escala, 0 y 100; el 0 se informa como 0 y
// no como null (FR-014, FR-015, RN-07).
func TestUS4_ExtremosDeLaEscala(t *testing.T) {
	for _, points := range []int{0, 100} {
		t.Run(fmt.Sprint(points), func(t *testing.T) {
			env := newBacklogTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

			rec := env.putStoryPoints(t, anaToken, storyID.String(), fmt.Sprintf(`{"storyPoints":%d}`, points))

			requireStoryDetail(t, rec)
			assert.Equal(t, float64(points), backlogRawBody(t, rec)["storyPoints"], "storyPoints debería ser %d y no null", points)
			stored := backlogStoredStory(t, env.db, storyID.String())
			require.NotNil(t, stored.StoryPoints, "story_points no debería ser NULL")
			assert.Equal(t, points, *stored.StoryPoints)
		})
	}
}

// Escenario: Comprometida no vuelve a sin estimar (003/US4)
// Con el puerto de sprints reemplazado por un doble que informa que la historia está comprometida en
// un sprint abierto, devolverla a "sin estimar" se rechaza con 409 indicando que primero hay que
// quitarla del sprint, y la estimación no cambia (FR-018, RN-18).
func TestUS4_ComprometidaNoVuelveASinEstimar(t *testing.T) {
	sprints := &backlogSprintAssignmentsDouble{}
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) { p.sprints = sprints })
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 1)
	sprints.On("Find", mock.Anything, storyID).
		Return(domain.SprintAssignment{CommittedToOpenSprint: true, EverAssigned: true}, nil)

	rec := env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":null}`)

	backlogRequireConflict(t, rec, backlogCommittedMessage)
	backlogRequireEstimateUnchanged(t, env.db, storyID, backlogPoints(5))
	sprints.AssertExpectations(t)
}

// Escenario: Valor fuera de escala (003/US4)
// Una historia estimada en 5 no acepta 50, 4 ni 6,5: cada intento se rechaza con 400 indicando la
// escala admitida, y el valor anterior y su momento de última actualización no cambian (FR-014,
// FR-019, RN-06).
func TestUS4_ValorFueraDeEscala(t *testing.T) {
	cases := []struct {
		name    string
		literal string
	}{
		{"50", "50"},
		{"4", "4"},
		{"6,5", "6.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 1)

			rec := env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":`+tc.literal+`}`)

			backlogRequireFieldRejected(t, rec, "storyPoints", backlogScale)
			backlogRequireEstimateUnchanged(t, env.db, storyID, backlogPoints(5))
		})
	}
}

// Escenario: Historia completada (003/US4)
// Una historia sembrada como completed no se reestima: el intento se rechaza con 409 porque una
// historia completada no se reestima, y la estimación no cambia (FR-030, RN-14).
func TestUS4_HistoriaCompletada(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusCompleted, backlogPoints(5), 1)

	rec := env.putStoryPoints(t, anaToken, storyID.String(), `{"storyPoints":8}`)

	backlogRequireConflict(t, rec, backlogCompletedMessage)
	backlogRequireEstimateUnchanged(t, env.db, storyID, backlogPoints(5))
}

// Escenario: Proyecto finalizado (003/US4)
// En un proyecto Finalizado, Bruno, integrante, intenta estimar una historia de su backlog: se
// rechaza con 409 porque el backlog es de solo lectura, y la historia sigue sin estimar (FR-048,
// RN-15).
func TestUS4_ProyectoFinalizado(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusFinished, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.putStoryPoints(t, brunoToken, storyID.String(), `{"storyPoints":8}`)

	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireEstimateUnchanged(t, env.db, storyID, nil)
}

// Pruebas de integración de T091 (hallazgo de seguridad detectado en 003/US4: regla de solo lectura
// del proyecto Finalizado verificada fuera de la escritura). Reproducen de forma determinista la
// carrera en la que el proyecto pasa a finished después de que se resolvió el acceso y se comprobó su
// estado, pero antes de escribir: un envoltorio del puerto correspondiente delega en la
// implementación real y, en cuanto esta responde que el proyecto no está Finalizado, lo marca como
// finished en la base. Así la comprobación previa ve el estado viejo y solo una verificación hecha en
// la propia escritura puede rechazar el cambio.

// backlogFinishProject marca el proyecto como finished directamente en la base, con una conexión
// propia y fuera de cualquier transacción de la solicitud, como lo haría la 002 en otra solicitud.
func backlogFinishProject(ctx context.Context, db *gorm.DB, projectID uuid.UUID) error {
	err := db.WithContext(ctx).
		Model(&projectsdomain.Project{}).
		Where("id = ?", projectID).
		UpdateColumn("status", projectsdomain.ProjectStatusFinished).Error
	if err != nil {
		return fmt.Errorf("marcar el proyecto como finished: %w", err)
	}
	return nil
}

// backlogFinishingStories envuelve el repositorio real de historias: FindAccessible resuelve el
// acceso con la implementación real, que informa el proyecto como no Finalizado, y recién después
// marca el proyecto como finished. Las demás operaciones, incluida UpdateStoryPoints, son las reales.
type backlogFinishingStories struct {
	domain.StoryRepository
	db *gorm.DB
}

// FindAccessible delega en el repositorio real y, si el acceso se resolvió, finaliza el proyecto.
func (r backlogFinishingStories) FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*domain.StoryAccess, error) {
	access, err := r.StoryRepository.FindAccessible(ctx, storyID, userID)
	if err != nil {
		return access, err
	}
	if err := backlogFinishProject(ctx, r.db, access.ProjectID); err != nil {
		return nil, err
	}
	return access, nil
}

// backlogFinishingProjects envuelve el puerto real de proyectos: IsFinished consulta el estado con la
// implementación real y, si el proyecto todavía no está Finalizado, lo marca como finished antes de
// devolver ese estado viejo al servicio.
type backlogFinishingProjects struct {
	domain.ProjectDirectory
	db *gorm.DB
}

// IsFinished delega en el puerto real y finaliza el proyecto cuando la respuesta es "no Finalizado".
func (d backlogFinishingProjects) IsFinished(ctx context.Context, projectID uuid.UUID) (bool, error) {
	finished, err := d.ProjectDirectory.IsFinished(ctx, projectID)
	if err != nil || finished {
		return finished, err
	}
	if err := backlogFinishProject(ctx, d.db, projectID); err != nil {
		return false, err
	}
	return false, nil
}

// backlogRequireProjectFinished exige que el proyecto haya quedado finished en la base, para
// confirmar que el envoltorio cambió el estado durante la solicitud y que la prueba reprodujo la
// carrera.
func backlogRequireProjectFinished(t *testing.T, db *gorm.DB, projectID uuid.UUID) {
	t.Helper()
	var project projectsdomain.Project
	require.NoError(t, db.Select("status").Where("id = ?", projectID).Take(&project).Error,
		"no se pudo leer el estado del proyecto")
	require.Equal(t, projectsdomain.ProjectStatusFinished, project.Status,
		"el proyecto debería haber pasado a finished durante la solicitud")
}

// TestStoryEstimate_ProyectoFinalizadoTrasResolverAcceso verifica que una estimación se rechaza con
// el 409 de solo lectura cuando el proyecto pasa a finished después de que RequireStoryAccess
// resolvió el acceso con el proyecto en curso y antes de guardar los Story Points, y que la historia
// sigue sin estimar con su momento de última actualización intacto (T091, FR-048, RN-15).
func TestStoryEstimate_ProyectoFinalizadoTrasResolverAcceso(t *testing.T) {
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) {
		p.stories = backlogFinishingStories{StoryRepository: p.stories, db: p.db}
	})
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.putStoryPoints(t, brunoToken, storyID.String(), `{"storyPoints":8}`)

	backlogRequireProjectFinished(t, env.db, projectID)
	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireEstimateUnchanged(t, env.db, storyID, nil)
}

// TestStoryCreate_ProyectoFinalizadoTrasResolverAcceso verifica que una creación se rechaza con el
// 409 de solo lectura cuando el proyecto pasa a finished después de que RequireMembership resolvió el
// acceso y el servicio comprobó que el proyecto no estaba Finalizado, pero antes de insertar la
// historia, y que no se crea ninguna historia ni ningún criterio (T091, FR-048, RN-15).
func TestStoryCreate_ProyectoFinalizadoTrasResolverAcceso(t *testing.T) {
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) {
		p.projects = backlogFinishingProjects{ProjectDirectory: p.projects, db: p.db}
	})
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)

	rec := env.postStory(t, brunoToken, projectID, backlogFullBody())

	backlogRequireProjectFinished(t, env.db, projectID)
	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireNothingCreated(t, env.db)
}

// TestStoryDelete_ProyectoFinalizadoTrasResolverAcceso verifica que una eliminación se rechaza con el
// 409 de solo lectura cuando el proyecto pasa a finished después de que RequireStoryAccess resolvió
// el acceso con el proyecto en curso y antes de borrar, y que la historia, sin historial y por lo
// tanto eliminable, sigue en la base con sus criterios intactos (T093, FR-048, RN-15).
func TestStoryDelete_ProyectoFinalizadoTrasResolverAcceso(t *testing.T) {
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) {
		p.stories = backlogFinishingStories{StoryRepository: p.stories, db: p.db}
	})
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 2)

	rec := env.deleteStory(t, brunoToken, storyID.String())

	backlogRequireProjectFinished(t, env.db, projectID)
	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending,
		[]string{backlogFirstCriterion, backlogSecondCriterion})
}

// Pruebas de integración de US5 (T058): PATCH /api/v1/stories/{storyId}. Las historias se siembran
// con backlogSeedEstimable, con prioridad Alta, el título del caso normal, sin descripción y con el
// momento fijo de US4 como creación y última actualización, para comprobar qué cambia y qué se
// conserva. Los estados in_progress y completed se siembran directamente en la base (FR-027) y la
// historia comprometida se prepara con el doble del puerto de sprints.

// Datos nuevos con que se refina la historia en el caso normal de US5.
const (
	backlogRefinedTitle       = "Exportar a PDF el reporte de velocidad del equipo"
	backlogRefinedDescription = "Incluye los sprints cerrados del último trimestre."
)

// patchStory envía el cuerpo JSON literal a PATCH /api/v1/stories/{storyId} con el token como Bearer.
func (e backlogTestEnv) patchStory(t *testing.T, token, storyID, rawBody string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, backlogStoryPath(storyID), strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// backlogCriteriaTexts devuelve los textos de los criterios guardados de la historia, en el orden de
// su posición, y exige que las posiciones sean 0..n-1.
func backlogCriteriaTexts(t *testing.T, db *gorm.DB, storyID string) []string {
	t.Helper()
	criteria := backlogStoredCriteria(t, db, storyID)
	texts := make([]string, 0, len(criteria))
	for i, criterion := range criteria {
		assert.Equal(t, i, criterion.Position, "las posiciones de los criterios deberían ser 0..n-1")
		texts = append(texts, criterion.Text)
	}
	return texts
}

// backlogRequireStoryUntouched exige que la fila sembrada con backlogSeedEstimable conserve el título,
// la descripción vacía, la prioridad Alta, el estado indicado, los criterios indicados y el momento de
// última actualización con que se sembró: ningún cambio, ni siquiera parcial, se aplicó (FR-023).
func backlogRequireStoryUntouched(t *testing.T, db *gorm.DB, storyID uuid.UUID, status domain.StoryStatus, criteria []string) {
	t.Helper()
	stored := backlogStoredStory(t, db, storyID.String())
	assert.Equal(t, backlogStoryTitle, stored.Title, "el título no debería cambiar")
	assert.Nil(t, stored.Description, "la descripción no debería cambiar")
	assert.Equal(t, domain.StoryPriorityHigh, stored.Priority, "la prioridad no debería cambiar")
	assert.Equal(t, status, stored.Status, "el estado no debería cambiar")
	assert.True(t, backlogEstimateUpdatedAt.Equal(stored.UpdatedAt),
		"updated_at no debería cambiar: quedó en %s", stored.UpdatedAt)
	assert.Equal(t, criteria, backlogCriteriaTexts(t, db, storyID.String()), "los criterios no deberían cambiar")
}

// Escenario: Refinamiento completo (003/US5)
// Ana cambia el título, la descripción, la prioridad y los criterios de una historia Pendiente: la
// respuesta y la base reflejan los valores nuevos, y el identificador, el estado, el proyecto, la
// autora y el momento de creación no cambian, mientras que el de última actualización avanza (FR-020,
// FR-022, FR-024).
func TestUS5_RefinamientoCompleto(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)
	body := validationPatchJSON(t, map[string]any{
		"title":              backlogRefinedTitle,
		"description":        backlogRefinedDescription,
		"priority":           "medium",
		"acceptanceCriteria": []string{backlogThirdCriterion, backlogFirstCriterion},
	})

	got := requireStoryDetail(t, env.patchStory(t, anaToken, storyID.String(), body))

	// La respuesta trae los valores nuevos y conserva identidad, estado, autora y creación.
	assert.Equal(t, storyID.String(), got.ID, "el identificador no cambia")
	assert.Equal(t, projectID.String(), got.ProjectID, "el proyecto no cambia")
	assert.Equal(t, backlogRefinedTitle, got.Title)
	require.NotNil(t, got.Description, "la descripción no debería ser null")
	assert.Equal(t, backlogRefinedDescription, *got.Description)
	assert.Equal(t, "medium", got.Priority)
	assert.Equal(t, []string{backlogThirdCriterion, backlogFirstCriterion}, got.AcceptanceCriteria)
	assert.Equal(t, "pending", got.Status, "el estado no cambia")
	require.NotNil(t, got.StoryPoints, "storyPoints no debería ser null")
	assert.Equal(t, 5, *got.StoryPoints, "la estimación no cambia")
	assert.Equal(t, anaID.String(), got.CreatedBy.ID, "la autora no cambia")
	assert.Equal(t, "2026-10-01T12:00:00Z", got.CreatedAt, "el momento de creación no cambia")
	updatedAt, err := time.Parse(time.RFC3339, got.UpdatedAt)
	require.NoError(t, err, "updatedAt = %q no es una fecha y hora RFC 3339", got.UpdatedAt)
	assert.True(t, updatedAt.After(backlogEstimateUpdatedAt), "updatedAt = %s debería ser posterior al anterior", got.UpdatedAt)

	// La base guarda los valores nuevos y conserva las columnas que el backlog no modifica.
	stored := backlogStoredStory(t, env.db, storyID.String())
	assert.Equal(t, backlogRefinedTitle, stored.Title)
	require.NotNil(t, stored.Description)
	assert.Equal(t, backlogRefinedDescription, *stored.Description)
	assert.Equal(t, domain.StoryPriorityMedium, stored.Priority)
	assert.Equal(t, domain.StoryStatusPending, stored.Status)
	assert.Equal(t, projectID, stored.ProjectID)
	assert.Equal(t, anaID, stored.CreatedByID)
	assert.True(t, backlogEstimateUpdatedAt.Equal(stored.CreatedAt), "created_at no debería cambiar: quedó en %s", stored.CreatedAt)
	assert.Equal(t, []string{backlogThirdCriterion, backlogFirstCriterion}, backlogCriteriaTexts(t, env.db, storyID.String()))
}

// Escenario: Historia de otro integrante (003/US5)
// Bruno, integrante que no es el autor, cambia la prioridad de una historia creada por Ana: el
// cambio se acepta y la autora sigue siendo Ana (FR-020, RN-01).
func TestUS5_HistoriaDeOtroIntegrante(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	got := requireStoryDetail(t, env.patchStory(t, brunoToken, storyID.String(), `{"priority":"low"}`))

	assert.Equal(t, "low", got.Priority)
	assert.Equal(t, anaID.String(), got.CreatedBy.ID, "la autora sigue siendo Ana")
	stored := backlogStoredStory(t, env.db, storyID.String())
	assert.Equal(t, domain.StoryPriorityLow, stored.Priority)
	assert.Equal(t, anaID, stored.CreatedByID)
}

// Escenario: Reorden de criterios (003/US5)
// Una historia con tres criterios recibe los mismos tres en otro orden: queda con ese orden, sin
// criterios duplicados, en la respuesta y en las posiciones guardadas (FR-022).
func TestUS5_ReordenDeCriterios(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(3), 3)
	reordered := []string{backlogThirdCriterion, backlogFirstCriterion, backlogSecondCriterion}

	got := requireStoryDetail(t, env.patchStory(t, anaToken, storyID.String(),
		validationPatchJSON(t, map[string]any{"acceptanceCriteria": reordered})))

	assert.Equal(t, reordered, got.AcceptanceCriteria)
	assert.Equal(t, reordered, backlogCriteriaTexts(t, env.db, storyID.String()))
}

// Escenario: Vaciar criterios (003/US5)
// Una historia estimada con dos criterios, no comprometida en ningún sprint, recibe una lista vacía:
// queda sin criterios, la respuesta trae [] y deja de estar lista para planificar por la falta de
// criterios (FR-022, FR-028).
func TestUS5_VaciarCriterios(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)

	rec := env.patchStory(t, anaToken, storyID.String(), `{"acceptanceCriteria":[]}`)

	got := requireStoryDetail(t, rec)
	assert.Equal(t, []any{}, backlogRawBody(t, rec)["acceptanceCriteria"], "acceptanceCriteria debería ser [] y no null")
	assert.False(t, got.Readiness.Ready, "sin criterios la historia no está lista para planificar")
	assert.Equal(t, []string{"acceptance_criteria"}, got.Readiness.Missing)
	assert.Zero(t, backlogCountRows(t, env.db, &domain.AcceptanceCriterion{}, "story_id = ?", storyID),
		"no deberían quedar criterios guardados")
}

// Escenario: Historia en progreso (003/US5)
// Una historia sembrada como in_progress acepta la corrección de su descripción, porque solo el
// estado Completada la cierra a modificaciones, y su estado sigue siendo in_progress (FR-030).
func TestUS5_HistoriaEnProgreso(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusInProgress, backlogPoints(5), 1)

	got := requireStoryDetail(t, env.patchStory(t, anaToken, storyID.String(),
		validationPatchJSON(t, map[string]any{"description": backlogRefinedDescription})))

	require.NotNil(t, got.Description, "la descripción no debería ser null")
	assert.Equal(t, backlogRefinedDescription, *got.Description)
	assert.Equal(t, "in_progress", got.Status, "el estado no cambia")
	stored := backlogStoredStory(t, env.db, storyID.String())
	require.NotNil(t, stored.Description)
	assert.Equal(t, backlogRefinedDescription, *stored.Description)
	assert.Equal(t, domain.StoryStatusInProgress, stored.Status)
}

// Escenario: Comprometida no queda sin criterios (003/US5)
// Con el puerto de sprints reemplazado por un doble que informa que la historia está comprometida en
// un sprint abierto, dejarla sin criterios se rechaza con 409 indicando que primero hay que quitarla
// del sprint, y los criterios no cambian (FR-022, RN-18).
func TestUS5_ComprometidaNoQuedaSinCriterios(t *testing.T) {
	sprints := &backlogSprintAssignmentsDouble{}
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) { p.sprints = sprints })
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)
	sprints.On("Find", mock.Anything, storyID).
		Return(domain.SprintAssignment{CommittedToOpenSprint: true, EverAssigned: true}, nil)

	rec := env.patchStory(t, anaToken, storyID.String(), `{"acceptanceCriteria":[]}`)

	backlogRequireConflict(t, rec, backlogCommittedMessage)
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending,
		[]string{backlogFirstCriterion, backlogSecondCriterion})
	sprints.AssertExpectations(t)
}

// Escenario: Entrada inválida sin cambio parcial (003/US5)
// Cada dato inválido llega junto a un cambio válido: la modificación se rechaza con 400 indicando el
// campo y el motivo, y ni el dato inválido ni el cambio válido que lo acompaña se aplican (FR-021,
// FR-023, RN-03 a RN-05, RN-08).
func TestUS5_EntradaInvalidaSinCambioParcial(t *testing.T) {
	cases := []struct {
		name  string
		body  map[string]any
		field string
		parts []string
	}{
		{"título vacío", map[string]any{"title": "", "priority": "low"}, "title", []string{"obligatorio"}},
		{"título de 151", map[string]any{"title": strings.Repeat("ñ", 151), "priority": "low"}, "title", []string{"150"}},
		{"descripción de 2001", map[string]any{"description": strings.Repeat("d", 2001), "title": backlogRefinedTitle},
			"description", []string{"2000"}},
		{"prioridad inexistente", map[string]any{"priority": "urgent", "title": backlogRefinedTitle},
			"priority", []string{"Alta", "Media", "Baja"}},
		{"criterio de 501", map[string]any{
			"acceptanceCriteria": []string{backlogThirdCriterion, strings.Repeat("c", 501)},
			"priority":           "low",
		}, "acceptanceCriteria[1]", []string{"500"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)

			rec := env.patchStory(t, anaToken, storyID.String(), validationPatchJSON(t, tc.body))

			backlogRequireFieldRejected(t, rec, tc.field, tc.parts...)
			backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending,
				[]string{backlogFirstCriterion, backlogSecondCriterion})
		})
	}
}

// Escenario: Historia completada (003/US5)
// Una historia sembrada como completed no se modifica: el intento se rechaza con 409 porque una
// historia completada no se modifica, y la historia no cambia (FR-030, RN-14).
func TestUS5_HistoriaCompletada(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusCompleted, backlogPoints(5), 1)

	rec := env.patchStory(t, anaToken, storyID.String(),
		validationPatchJSON(t, map[string]any{"title": backlogRefinedTitle, "priority": "low"}))

	backlogRequireConflict(t, rec, backlogCompletedMessage)
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusCompleted, []string{backlogFirstCriterion})
}

// Escenario: Proyecto finalizado (003/US5)
// En un proyecto Finalizado, Bruno, integrante, intenta refinar una historia de su backlog: se
// rechaza con 409 porque el backlog es de solo lectura, y la historia no cambia (FR-048, RN-15).
func TestUS5_ProyectoFinalizado(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusFinished, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.patchStory(t, brunoToken, storyID.String(), `{"priority":"low"}`)

	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
}

// Escenario: Historia ajena (003/US5)
// Carla, que no es integrante del proyecto, intenta refinar una historia que existe: recibe el 404 de
// historia inexistente, idéntico byte a byte al de un identificador que no existe, y la historia no
// cambia (FR-047, SC-006).
func TestUS5_HistoriaAjena(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)
	body := validationPatchJSON(t, map[string]any{"title": backlogRefinedTitle})

	foreign := env.patchStory(t, carlaToken, storyID.String(), body)
	missing := env.patchStory(t, anaToken, uuid.New().String(), body)

	backlogRequireIdenticalNotFound(t, foreign, missing)
	assert.NotContains(t, foreign.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
}

// Escenario: Cambio de estado no disponible (003/US5)
// Una historia Pendiente que no pertenece a ningún sprint no se puede pasar a En progreso ni a
// Completada desde el backlog: el PATCH con status se rechaza con 400 señalando ese campo, y la
// historia sigue pending y sin cambios (FR-027, RN-10).
func TestUS5_CambioDeEstadoNoDisponible(t *testing.T) {
	for _, status := range []string{"in_progress", "completed"} {
		t.Run(status, func(t *testing.T) {
			env := newBacklogTestEnv(t)
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 1)

			rec := env.patchStory(t, anaToken, storyID.String(), `{"status":"`+status+`"}`)

			backlogRequireFieldRejected(t, rec, "status", "no se cambia desde el backlog")
			backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
		})
	}
}

// Pruebas de integración de US6 (T067): DELETE /api/v1/stories/{storyId}. Las historias se siembran
// con backlogSeedEstimable, y las que tienen historial de sprint, esfuerzo, defectos o sesiones de
// Planning Poker se preparan reemplazando el puerto correspondiente por un doble de testify, porque
// ninguna de esas features existe todavía (plan.md, cómo se preparan los escenarios). En los rechazos
// se verifica que la historia, sus criterios y su momento de última actualización queden intactos.

// backlogDeletionBlockedMessage es el mensaje exacto del 409 de eliminación bloqueada por historial.
const backlogDeletionBlockedMessage = "La historia tiene historial asociado y no se puede eliminar."

// backlogDeletionReasonMessages son los mensajes exactos de cada motivo de bloqueo según la tabla de
// data-model.md sección 7.
var backlogDeletionReasonMessages = map[string]string{
	"sprint_history":      "Estuvo asignada a un sprint.",
	"effort":              "Tiene esfuerzo registrado.",
	"defects":             "Tiene defectos asociados.",
	"estimation_sessions": "Tiene sesiones de estimación por Planning Poker.",
}

// backlogHistoryRecordsDouble es el doble de testify de los puertos EffortRecords, DefectRecords y
// EstimationSessions: cada prueba usa una instancia por puerto y configura solo el método que ese
// puerto expone, para preparar una historia con esfuerzo, defectos o sesiones de estimación sin
// depender de las features 005 a 007.
type backlogHistoryRecordsDouble struct {
	mock.Mock
}

// HasEffort devuelve lo que la prueba configuró para el esfuerzo de la historia.
func (m *backlogHistoryRecordsDouble) HasEffort(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// HasDefects devuelve lo que la prueba configuró para los defectos de la historia.
func (m *backlogHistoryRecordsDouble) HasDefects(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// HasSessions devuelve lo que la prueba configuró para las sesiones de estimación de la historia.
func (m *backlogHistoryRecordsDouble) HasSessions(ctx context.Context, storyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, storyID)
	return args.Bool(0), args.Error(1)
}

// deleteStory envía DELETE /api/v1/stories/{storyId} con el token como Bearer.
func (e backlogTestEnv) deleteStory(t *testing.T, token, storyID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, backlogStoryPath(storyID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// backlogRequireDeleted exige un 204 sin cuerpo y que no quede en la base ni la historia ni ninguno
// de sus criterios: el borrado es definitivo (FR-041, FR-044; research.md sección 12).
func backlogRequireDeleted(t *testing.T, db *gorm.DB, rec *httptest.ResponseRecorder, storyID string) {
	t.Helper()
	require.Equal(t, http.StatusNoContent, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Empty(t, rec.Body.String(), "el 204 no debería tener cuerpo")
	assert.Zero(t, backlogCountRows(t, db, &domain.UserStory{}, "id = ?", storyID),
		"la historia no debería seguir en user_stories")
	assert.Zero(t, backlogCountRows(t, db, &domain.AcceptanceCriterion{}, "story_id = ?", storyID),
		"no deberían quedar filas de la historia en acceptance_criteria")
}

// backlogRequireDeletionBlocked exige el 409 de DeletionBlockedResponse con exactamente los motivos
// indicados, en ese orden y con sus mensajes del contrato (FR-043).
func backlogRequireDeletionBlocked(t *testing.T, rec *httptest.ResponseRecorder, codes ...string) {
	t.Helper()
	reasons := make([]map[string]string, 0, len(codes))
	for _, code := range codes {
		reasons = append(reasons, map[string]string{"code": code, "message": backlogDeletionReasonMessages[code]})
	}
	want, err := json.Marshal(map[string]any{"error": backlogDeletionBlockedMessage, "reasons": reasons})
	require.NoError(t, err, "no se pudo serializar el cuerpo esperado")
	assert.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, string(want), rec.Body.String())
}

// Escenario: Eliminación exitosa (003/US6)
// Ana elimina una historia recién creada que nunca estuvo en un sprint y no tiene esfuerzo ni
// defectos: la respuesta es 204, la historia y sus criterios desaparecen de la base y el listado ya
// no la muestra, mientras que otra historia del mismo backlog sigue en él (FR-041, FR-044).
func TestUS6_EliminacionExitosa(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	created := requireCreatedStory(t, env.postStory(t, anaToken, projectID, backlogFullBody()))
	otherID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.deleteStory(t, anaToken, created.ID)

	backlogRequireDeleted(t, env.db, rec, created.ID)
	list := requireStoryList(t, env.getStories(t, anaToken, projectID, ""))
	assert.Equal(t, []string{otherID.String()}, backlogListIDs(list),
		"el listado debería conservar solo la otra historia")
	assert.Len(t, backlogStoredCriteria(t, env.db, otherID.String()), 1,
		"los criterios de la otra historia no deberían tocarse")
}

// Escenario: Lista para planificar se elimina (003/US6)
// Una historia estimada en 5 y con dos criterios, que nunca se asignó a un sprint, se elimina: estar
// lista para planificar no es haberse planificado (FR-041, RN-16).
func TestUS6_ListaParaPlanificarSeElimina(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)

	rec := env.deleteStory(t, anaToken, storyID.String())

	backlogRequireDeleted(t, env.db, rec, storyID.String())
}

// Escenario: Integrante no propietario (003/US6)
// Bruno, integrante que no es el propietario ni el autor, elimina una historia creada por Ana sin
// historial: la eliminación se acepta (FR-042, RN-01, RN-02).
func TestUS6_IntegranteNoPropietario(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.deleteStory(t, brunoToken, storyID.String())

	backlogRequireDeleted(t, env.db, rec, storyID.String())
}

// Escenario: Quitada de un sprint (003/US6)
// Con el puerto de sprints reemplazado por un doble que informa que la historia estuvo en un sprint
// pero ya no está comprometida en uno abierto, la eliminación se rechaza con 409 y el motivo
// sprint_history, y la historia queda intacta (FR-041, FR-043, RN-16).
func TestUS6_QuitadaDeUnSprint(t *testing.T) {
	sprints := &backlogSprintAssignmentsDouble{}
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) { p.sprints = sprints })
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 1)
	sprints.On("Find", mock.Anything, storyID).
		Return(domain.SprintAssignment{CommittedToOpenSprint: false, EverAssigned: true}, nil)

	rec := env.deleteStory(t, anaToken, storyID.String())

	backlogRequireDeletionBlocked(t, rec, "sprint_history")
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
	sprints.AssertExpectations(t)
}

// Escenario: Única historia (003/US6)
// En un backlog con una sola historia, Ana la elimina: el listado responde 200 sin historias y con
// emptyReason backlog_empty, como backlog vacío y no como error (FR-035, FR-044).
func TestUS6_UnicaHistoria(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	backlogRequireDeleted(t, env.db, env.deleteStory(t, anaToken, storyID.String()), storyID.String())

	list := requireStoryList(t, env.getStories(t, anaToken, projectID, ""))
	assert.Empty(t, list.Stories, "el backlog debería quedar sin historias")
	backlogRequireEmptyReason(t, list, "backlog_empty")
}

// Escenario: Esfuerzo o defectos (003/US6)
// Con el puerto de esfuerzo, el de defectos o ambos reemplazados por dobles que informan historial,
// la eliminación se rechaza con 409 indicando cada motivo que aplica, en el orden del contrato, y la
// historia no se modifica (FR-043, RN-16).
func TestUS6_EsfuerzoODefectos(t *testing.T) {
	cases := []struct {
		name       string
		hasEffort  bool
		hasDefects bool
		codes      []string
	}{
		{"esfuerzo registrado", true, false, []string{"effort"}},
		{"defectos asociados", false, true, []string{"defects"}},
		{"esfuerzo y defectos", true, true, []string{"effort", "defects"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effort := &backlogHistoryRecordsDouble{}
			defects := &backlogHistoryRecordsDouble{}
			env := newBacklogTestEnv(t, func(p *backlogTestPorts) {
				p.effort = effort
				p.defects = defects
			})
			anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(3), 2)
			effort.On("HasEffort", mock.Anything, storyID).Return(tc.hasEffort, nil)
			defects.On("HasDefects", mock.Anything, storyID).Return(tc.hasDefects, nil)

			rec := env.deleteStory(t, anaToken, storyID.String())

			backlogRequireDeletionBlocked(t, rec, tc.codes...)
			backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending,
				[]string{backlogFirstCriterion, backlogSecondCriterion})
			effort.AssertExpectations(t)
			defects.AssertExpectations(t)
		})
	}
}

// Escenario: Sesiones de estimación (003/US6)
// Una historia que nunca estuvo en un sprint, con el puerto de Planning Poker reemplazado por un doble
// que informa sesiones de estimación, no se elimina: 409 con el motivo estimation_sessions y la
// historia intacta (FR-043, RN-16, RC-10).
func TestUS6_SesionesDeEstimacion(t *testing.T) {
	sessions := &backlogHistoryRecordsDouble{}
	env := newBacklogTestEnv(t, func(p *backlogTestPorts) { p.sessions = sessions })
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(8), 1)
	sessions.On("HasSessions", mock.Anything, storyID).Return(true, nil)

	rec := env.deleteStory(t, anaToken, storyID.String())

	backlogRequireDeletionBlocked(t, rec, "estimation_sessions")
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
	sessions.AssertExpectations(t)
}

// Escenario: Proyecto finalizado (003/US6)
// En un proyecto Finalizado, Bruno, integrante, intenta eliminar una historia sin historial: se
// rechaza con 409 porque el backlog es de solo lectura, y la historia sigue en la base sin cambios
// (FR-048, RN-15).
func TestUS6_ProyectoFinalizado(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusFinished, brunoID)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	rec := env.deleteStory(t, brunoToken, storyID.String())

	backlogRequireConflict(t, rec, backlogReadOnlyMessage)
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
}

// Escenario: Historia ajena (003/US6)
// Carla, que no es integrante del proyecto, intenta eliminar una historia que existe: recibe el 404 de
// historia inexistente, idéntico byte a byte al de un identificador que no existe, y la historia no
// se toca (FR-047, SC-006).
func TestUS6_HistoriaAjena(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	foreign := env.deleteStory(t, carlaToken, storyID.String())
	missing := env.deleteStory(t, anaToken, uuid.New().String())

	backlogRequireIdenticalNotFound(t, foreign, missing)
	assert.NotContains(t, foreign.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
	backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
}

// Escenario: Ya eliminada (003/US6)
// Ana elimina una historia y vuelve a intentarlo: el segundo intento recibe el 404 de historia
// inexistente, idéntico al de un identificador que nunca existió (FR-044, FR-047).
func TestUS6_YaEliminada(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)
	backlogRequireDeleted(t, env.db, env.deleteStory(t, anaToken, storyID.String()), storyID.String())

	again := env.deleteStory(t, anaToken, storyID.String())
	missing := env.deleteStory(t, anaToken, uuid.New().String())

	backlogRequireIdenticalNotFound(t, again, missing)
}

// Pruebas de caracterización de la fase Polish (T078). No llevan etiqueta 003/USn porque derivan de
// los criterios de éxito SC-005 y SC-006 y del Principio VI, no de un escenario: recorren las seis
// rutas del backlog ya protegidas por el middleware de sesión, RequireMembership y
// RequireStoryAccess, y fijan su comportamiento para que una regresión en la composición se detecte.

// backlogEndpointCall describe una de las seis acciones del backlog: su nombre para el subtest, su
// método, su ruta y el cuerpo válido que se envía, que produciría un cambio si llegara al manejador.
type backlogEndpointCall struct {
	name   string
	method string
	path   string
	body   string
}

// backlogSend envía la solicitud con el valor indicado en la cabecera Authorization; si el valor es
// vacío, la solicitud viaja sin esa cabecera.
func (e backlogTestEnv) backlogSend(t *testing.T, call backlogEndpointCall, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(call.method, call.path, strings.NewReader(call.body))
	if call.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// backlogCollectionCalls devuelve las dos acciones de colección sobre el proyecto indicado: crear y
// listar historias.
func backlogCollectionCalls(t *testing.T, projectID uuid.UUID) []backlogEndpointCall {
	t.Helper()
	payload, err := json.Marshal(backlogFullBody())
	require.NoError(t, err, "no se pudo serializar la solicitud")
	return []backlogEndpointCall{
		{name: "POST stories", method: http.MethodPost, path: backlogStoriesPath(projectID), body: string(payload)},
		{name: "GET stories", method: http.MethodGet, path: backlogStoriesPath(projectID)},
	}
}

// backlogItemCalls devuelve las cuatro acciones de ítem sobre la historia indicada: detalle,
// refinamiento, eliminación y estimación.
func backlogItemCalls(t *testing.T, storyID string) []backlogEndpointCall {
	t.Helper()
	return []backlogEndpointCall{
		{name: "GET story", method: http.MethodGet, path: backlogStoryPath(storyID)},
		{name: "PATCH story", method: http.MethodPatch, path: backlogStoryPath(storyID),
			body: validationPatchJSON(t, map[string]any{"title": backlogRefinedTitle})},
		{name: "DELETE story", method: http.MethodDelete, path: backlogStoryPath(storyID)},
		{name: "PUT story-points", method: http.MethodPut, path: backlogStoryPath(storyID) + "/story-points",
			body: `{"storyPoints":13}`},
	}
}

// backlogTamperToken altera un carácter en medio de la firma del JWT, de modo que el token conserva
// su forma pero su firma deja de ser válida.
func backlogTamperToken(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "el token de prueba debería tener tres segmentos")
	signature := []byte(parts[2])
	middle := len(signature) / 2
	if signature[middle] == 'A' {
		signature[middle] = 'B'
	} else {
		signature[middle] = 'A'
	}
	parts[2] = string(signature)
	return strings.Join(parts, ".")
}

// backlogRequireNoPasswordHash exige que la respuesta no exponga el hash de contraseña en ninguna de
// sus dos grafías (Principio VI).
func backlogRequireNoPasswordHash(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.NotContains(t, rec.Body.String(), "password_hash", "la respuesta no debería exponer password_hash")
	assert.NotContains(t, rec.Body.String(), "passwordHash", "la respuesta no debería exponer passwordHash")
}

// Sin cabecera de autorización y con un token de firma adulterada, cada una de las seis acciones del
// backlog responde el 401 de la 001 y no aplica ningún cambio: no se crea ninguna historia y la
// sembrada conserva sus datos, sus criterios y su momento de última actualización (SC-005, FR-045).
func TestStoryEndpoints_SinSesion(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)
	calls := append(backlogCollectionCalls(t, projectID), backlogItemCalls(t, storyID.String())...)
	variants := []struct {
		name          string
		authorization string
	}{
		{name: "sin cabecera", authorization: ""},
		{name: "token adulterado", authorization: "Bearer " + backlogTamperToken(t, anaToken)},
	}

	// Cada acción se prueba con las dos variantes y, después de cada solicitud, se comprueba que la
	// base sigue exactamente como se sembró.
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			for _, variant := range variants {
				t.Run(variant.name, func(t *testing.T) {
					rec := env.backlogSend(t, call, variant.authorization)

					assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
					assert.Equal(t, storyAccessSessionBody, rec.Body.String(), "el cuerpo debería ser el 401 de la 001")
					assert.NotContains(t, rec.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
					backlogRequireNoPasswordHash(t, rec)
					assert.Equal(t, int64(1), backlogCountRows(t, env.db, &domain.UserStory{}, ""),
						"no debería crearse ni eliminarse ninguna historia")
					backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
				})
			}
		})
	}
}

// Para cada acción, la respuesta a quien no es integrante es idéntica byte a byte a la de un
// identificador inexistente: en las cuatro acciones de ítem, el 404 de historia ajena frente al de
// historia inexistente; en las dos de colección, el 404 de proyecto ajeno frente al de proyecto
// inexistente. Ninguna respuesta revela datos ni aplica cambios (SC-006, FR-047).
func TestStoryEndpoints_NotFoundIndistinguible(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
	storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, nil, 1)

	// Acciones de ítem: Carla pide la historia de Ana y Ana pide un identificador que no existe.
	foreignItems := backlogItemCalls(t, storyID.String())
	missingItems := backlogItemCalls(t, uuid.New().String())
	for i := range foreignItems {
		t.Run(foreignItems[i].name, func(t *testing.T) {
			foreign := env.backlogSend(t, foreignItems[i], "Bearer "+carlaToken)
			missing := env.backlogSend(t, missingItems[i], "Bearer "+anaToken)

			backlogRequireIdenticalNotFound(t, foreign, missing)
			assert.NotContains(t, foreign.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
			backlogRequireNoPasswordHash(t, foreign)
			backlogRequireNoPasswordHash(t, missing)
			backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
		})
	}

	// Acciones de colección: Carla pide el proyecto de Ana y un proyecto que no existe.
	foreignCollection := backlogCollectionCalls(t, projectID)
	missingCollection := backlogCollectionCalls(t, uuid.New())
	for i := range foreignCollection {
		t.Run(foreignCollection[i].name, func(t *testing.T) {
			foreign := env.backlogSend(t, foreignCollection[i], "Bearer "+carlaToken)
			missing := env.backlogSend(t, missingCollection[i], "Bearer "+carlaToken)

			assert.Equal(t, http.StatusNotFound, foreign.Code, "cuerpo: %s", foreign.Body.String())
			assert.JSONEq(t, `{"error":"`+backlogProjectNotFound+`"}`, foreign.Body.String())
			assert.Equal(t, missing.Code, foreign.Code, "los códigos de estado deberían ser idénticos")
			assert.Equal(t, missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"),
				"los tipos de contenido deberían ser idénticos")
			assert.Equal(t, missing.Body.Bytes(), foreign.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
			assert.NotContains(t, foreign.Body.String(), backlogStoryTitle, "la respuesta no debería revelar el título")
			backlogRequireNoPasswordHash(t, foreign)
			backlogRequireNoPasswordHash(t, missing)
			assert.Equal(t, int64(1), backlogCountRows(t, env.db, &domain.UserStory{}, ""),
				"no debería crearse ninguna historia")
			backlogRequireStoryUntouched(t, env.db, storyID, domain.StoryStatusPending, []string{backlogFirstCriterion})
		})
	}
}

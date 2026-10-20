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
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
// POST /api/v1/projects/{projectId}/stories; las historias siguientes agregan las suyas en este
// archivo. Los ayudantes llevan el prefijo "backlog" para no chocar con los de validation_test.go.

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

// backlogTestPorts son los cuatro puertos hacia features que todavía no existen. Por defecto son las
// implementaciones de backlog/repository; una prueba los reemplaza por dobles para preparar
// historias comprometidas o con historial (plan.md, cómo se preparan los escenarios).
type backlogTestPorts struct {
	sprints  domain.SprintAssignments
	effort   domain.EffortRecords
	defects  domain.DefectRecords
	sessions domain.EstimationSessions
}

// backlogTestEnv reúne el router compuesto como en cmd/app, la base de test y el servicio de
// autenticación con el que se crean las cuentas y las sesiones de cada escenario.
type backlogTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	auth   *authservice.AuthService
}

// newBacklogTestEnv abre la base de test vacía y compone, igual que cmd/app, el límite global del
// cuerpo, la cadena real de la 001 (para el middleware de sesión), el módulo de proyectos de la 002
// con su subgrupo /:projectId y las rutas del backlog colgadas de ese subgrupo, sin CORS ni Swagger.
// Cada override recibe los puertos por defecto y puede reemplazar cualquiera por un doble.
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
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)

	// Módulo de proyectos de la 002, que devuelve el subgrupo /api/v1/projects/:projectId con
	// RequireMembership.
	projectRepository := projectsrepository.NewProjectRepository(db)
	projectMembers := projectsrepository.NewProjectMemberRepository(db)
	projects := projectsservice.NewProjectService(
		projectRepository,
		projectMembers,
		projectsrepository.NewBacklogSummaryProvider(),
		projectsrepository.NewSprintDirectory(),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(), cfg.Location, time.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// Módulo del backlog con los puertos por defecto o los dobles que pida la prueba.
	ports := backlogTestPorts{
		sprints:  repository.NewSprintAssignments(),
		effort:   repository.NewEffortRecords(),
		defects:  repository.NewDefectRecords(),
		sessions: repository.NewEstimationSessions(),
	}
	for _, override := range overrides {
		override(&ports)
	}
	stories := service.NewStoryService(
		repository.NewStoryRepository(db),
		ports.sprints,
		ports.effort,
		ports.defects,
		ports.sessions,
		repository.NewProjectDirectory(db),
	)
	RegisterRoutes(projectGroup, NewHandler(stories))
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

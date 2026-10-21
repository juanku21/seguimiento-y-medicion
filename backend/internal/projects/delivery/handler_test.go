package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
)

// Pruebas de integración de punta a punta del módulo de proyectos: atraviesan el router de Gin, el
// middleware de sesión de la 001, el servicio y el repositorio contra la base smye_test (Principios
// II y V). Las de US1 (T019) cubren POST /api/v1/projects; las historias siguientes agregan las
// suyas en este archivo.

// projectsPath es la ruta de creación y listado de proyectos según contracts/openapi.yaml.
const projectsPath = "/api/v1/projects"

// Textos exactos que fija el contrato para el 409 de nombre repetido, el 401 de sesión inválida y
// el 400 de entrada inválida.
const (
	projectNameTakenMessage   = "Ya existe un proyecto propio con ese nombre."
	projectSessionMessage     = "Se requiere iniciar sesión."
	projectInvalidFieldsError = "Hay campos inválidos en la solicitud."
)

// testUserPassword es la contraseña de las cuentas que crean las pruebas; cumple las reglas de la 001.
const testUserPassword = "Proyecto2026"

// projectDetailBody refleja ProjectDetail del contrato; se declara en la prueba para no depender de
// los DTO de producción al leer la respuesta.
type projectDetailBody struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	Status             string  `json:"status"`
	StartDate          string  `json:"startDate"`
	PlannedEndDate     string  `json:"plannedEndDate"`
	ActualEndDate      *string `json:"actualEndDate"`
	HoursPerStoryPoint float64 `json:"hoursPerStoryPoint"`
	ReadOnly           bool    `json:"readOnly"`
	Owner              struct {
		ID       string `json:"id"`
		FullName string `json:"fullName"`
	} `json:"owner"`
	Members []struct {
		UserID   string `json:"userId"`
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		IsOwner  bool   `json:"isOwner"`
	} `json:"members"`
	HasActiveSprint bool `json:"hasActiveSprint"`
	ActiveSprint    *struct {
		ID string `json:"id"`
	} `json:"activeSprint"`
	Backlog struct {
		CountsByState        map[string]int `json:"countsByState"`
		EstimatedStoryPoints int            `json:"estimatedStoryPoints"`
		EstimatedHours       float64        `json:"estimatedHours"`
	} `json:"backlog"`
}

// projectValidationBody refleja ValidationErrorResponse del contrato.
type projectValidationBody struct {
	Error  string `json:"error"`
	Fields []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"fields"`
}

// projectErrorBody refleja ErrorResponse del contrato.
type projectErrorBody struct {
	Error string `json:"error"`
}

// projectsTestEnv reúne el router compuesto como en cmd/app, la base de test, el servicio de
// autenticación con el que se crean las cuentas y las sesiones de cada escenario, y el subgrupo
// /api/v1/projects/:projectId que devuelve RegisterRoutes junto con su subgrupo de administración.
// location es la zona horaria del sistema cargada de .env.test, con la que las pruebas de US6
// calculan la fecha de finalización real esperada.
type projectsTestEnv struct {
	router       *gin.Engine
	db           *gorm.DB
	auth         *authservice.AuthService
	projectGroup *gin.RouterGroup
	adminGroup   *gin.RouterGroup
	location     *time.Location
}

// newProjectsTestEnv abre la base de test vacía y compone la cadena real de la 001 (para el
// middleware de sesión) y del módulo de proyectos, con el servicio de proyectos, el de integrantes y
// el de cambio de estado, igual que cmd/app: con el límite global del cuerpo, sin CORS ni Swagger.
func newProjectsTestEnv(t *testing.T) projectsTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	projectRepository := repository.NewProjectRepository(db)
	projectMembers := repository.NewProjectMemberRepository(db)
	projects := service.NewProjectService(
		projectRepository,
		projectMembers,
		repository.NewBacklogSummaryProvider(db),
		repository.NewSprintDirectory(),
	)
	members := service.NewMemberService(projectRepository, projectMembers, repository.NewUserDirectory(db))
	status := service.NewStatusService(projectRepository, repository.NewSprintDirectory(), cfg.Location, time.Now)
	router := gin.New()
	router.Use(httpserver.LimitRequestBody())
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)
	projectGroup, adminGroup := RegisterRoutes(router, authHandler.RequireSession, NewHandler(projects, members, status), projectMembers, projectRepository)
	return projectsTestEnv{router: router, db: db, auth: auth, projectGroup: projectGroup, adminGroup: adminGroup, location: cfg.Location}
}

// signUp registra una cuenta con el servicio de la 001, inicia su sesión y devuelve el identificador
// de la cuenta y el token de la sesión.
func (e projectsTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	user, err := e.auth.Register(ctx, fullName, email, testUserPassword)
	require.NoError(t, err, "no se pudo registrar la cuenta de prueba %s", email)
	login, err := e.auth.Login(ctx, email, testUserPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de prueba %s", email)
	return user.ID, login.Token
}

// postProject envía el cuerpo a POST /api/v1/projects con el token recibido como Bearer; con token
// vacío la solicitud va sin cabecera Authorization.
func (e projectsTestEnv) postProject(t *testing.T, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err, "no se pudo serializar la solicitud")
	req := httptest.NewRequest(http.MethodPost, projectsPath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// validProjectInputBody devuelve el cuerpo del escenario normal de US1; cada prueba modifica o quita
// solo el campo que le interesa. El factor va como json.Number para enviar su texto literal.
func validProjectInputBody() map[string]any {
	return map[string]any{
		"name":               "Portal de Clientes",
		"description":        "Rediseño del portal de autogestión.",
		"startDate":          "2026-10-01",
		"plannedEndDate":     "2026-12-20",
		"hoursPerStoryPoint": json.Number("6.5"),
	}
}

// requireCreatedProject exige un 201 cuyo cuerpo sea un ProjectDetail con id UUID y lo devuelve
// decodificado.
func requireCreatedProject(t *testing.T, rec *httptest.ResponseRecorder) projectDetailBody {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 201 no es JSON válido: %s", rec.Body.String())
	_, err := uuid.Parse(body.ID)
	require.NoError(t, err, "id = %q no es un UUID", body.ID)
	return body
}

// requireFieldRejected exige un 400 con ValidationErrorResponse que señale el campo indicado y
// cuyo mensaje mencione cada una de las piezas esperadas, sin distinguir mayúsculas.
func requireFieldRejected(t *testing.T, rec *httptest.ResponseRecorder, field string, parts ...string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectValidationBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 400 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, projectInvalidFieldsError, body.Error)
	for _, f := range body.Fields {
		if f.Field != field {
			continue
		}
		msg := strings.ToLower(f.Message)
		for _, part := range parts {
			assert.Contains(t, msg, strings.ToLower(part), "el mensaje de %s debería mencionar %q", field, part)
		}
		return
	}
	t.Errorf("la respuesta no señala el campo %q: %s", field, rec.Body.String())
}

// countProjectRows cuenta las filas de projects, opcionalmente filtradas.
func countProjectRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	tx := db.Model(&domain.Project{})
	if query != "" {
		tx = tx.Where(query, args...)
	}
	var n int64
	require.NoError(t, tx.Count(&n).Error, "no se pudieron contar los proyectos")
	return n
}

// countMemberRows cuenta las filas de project_members, opcionalmente filtradas.
func countMemberRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	tx := db.Model(&domain.ProjectMember{})
	if query != "" {
		tx = tx.Where(query, args...)
	}
	var n int64
	require.NoError(t, tx.Count(&n).Error, "no se pudieron contar las membresías")
	return n
}

// requireNothingCreated exige que no exista ningún proyecto ni ninguna membresía (FR-010).
func requireNothingCreated(t *testing.T, db *gorm.DB) {
	t.Helper()
	assert.Zero(t, countProjectRows(t, db, ""), "no debería haberse creado ningún proyecto")
	assert.Zero(t, countMemberRows(t, db, ""), "no debería haberse creado ninguna membresía")
}

// findStoredProject lee de projects la fila con el identificador devuelto por la API.
func findStoredProject(t *testing.T, db *gorm.DB, id string) domain.Project {
	t.Helper()
	var project domain.Project
	require.NoError(t, db.First(&project, "id = ?", id).Error, "no se encontró el proyecto %s en projects", id)
	return project
}

// assertNoPasswordHash comprueba sobre el cuerpo crudo de una respuesta exitosa que no aparezca el
// hash de la contraseña de ninguna cuenta, ni en snake_case ni en camelCase, aunque la respuesta
// incluya integrantes o al propietario (Principio VI).
func assertNoPasswordHash(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := rec.Body.String()
	assert.NotContains(t, body, "password_hash", "la respuesta no debería exponer el hash de la contraseña")
	assert.NotContains(t, body, "passwordHash", "la respuesta no debería exponer el hash de la contraseña")
}

// seedOwnedProject inserta directamente en la base un proyecto del propietario indicado con su
// membresía, para preparar el "Dado" de un escenario sin pasar por la API que se está probando.
func seedOwnedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, name string) {
	t.Helper()
	project := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         name,
		NameNormalized:               strings.ToLower(strings.TrimSpace(name)),
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       domain.ProjectStatusPlanned,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto %q", name)
	member := domain.ProjectMember{ID: uuid.New(), ProjectID: project.ID, UserID: ownerID}
	require.NoError(t, db.Create(&member).Error, "no se pudo sembrar la membresía del propietario")
}

// Escenario: Creación exitosa (002/US1)
// Un usuario sin proyectos crea "Portal de Clientes" del 2026-10-01 al 2026-12-20 con factor 6,5: el
// proyecto queda en planned, sin fecha de finalización real y con el creador como propietario y
// único integrante, tanto en la respuesta como en la base (FR-007, FR-008, RN-01).
func TestUS1_CreacionExitosa(t *testing.T) {
	env := newProjectsTestEnv(t)
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")

	rec := env.postProject(t, token, validProjectInputBody())
	got := requireCreatedProject(t, rec)
	assertNoPasswordHash(t, rec)

	// La respuesta es la vista completa del proyecto recién creado.
	assert.Equal(t, "Portal de Clientes", got.Name)
	if assert.NotNil(t, got.Description) {
		assert.Equal(t, "Rediseño del portal de autogestión.", *got.Description)
	}
	assert.Equal(t, string(domain.ProjectStatusPlanned), got.Status)
	assert.Equal(t, "2026-10-01", got.StartDate)
	assert.Equal(t, "2026-12-20", got.PlannedEndDate)
	assert.Nil(t, got.ActualEndDate, "un proyecto recién creado no tiene fecha de finalización real")
	assert.InDelta(t, 6.5, got.HoursPerStoryPoint, 1e-9)
	assert.False(t, got.ReadOnly)
	assert.Equal(t, userID.String(), got.Owner.ID)
	assert.Equal(t, "Ana Pérez", got.Owner.FullName, "el propietario se muestra con el nombre con que se registró")
	if assert.Len(t, got.Members, 1, "el creador debe ser el único integrante") {
		assert.Equal(t, userID.String(), got.Members[0].UserID)
		assert.True(t, got.Members[0].IsOwner, "el único integrante debe estar marcado propietario")

		// Quien consulta es el propietario, así que ve el nombre y también el correo del integrante
		// (FR-030).
		assert.Equal(t, "Ana Pérez", got.Members[0].FullName)
		assert.Equal(t, "ana@example.com", got.Members[0].Email)
	}
	assert.False(t, got.HasActiveSprint)
	assert.Nil(t, got.ActiveSprint)
	assert.Empty(t, got.Backlog.CountsByState)
	assert.Zero(t, got.Backlog.EstimatedStoryPoints)
	assert.Zero(t, got.Backlog.EstimatedHours)

	// En la base queda un único proyecto con los datos persistidos y una única membresía del creador.
	stored := findStoredProject(t, env.db, got.ID)
	assert.Equal(t, userID, stored.OwnerID)
	assert.Equal(t, "Portal de Clientes", stored.Name)
	assert.Equal(t, "portal de clientes", stored.NameNormalized)
	assert.Equal(t, domain.ProjectStatusPlanned, stored.Status)
	assert.Nil(t, stored.ActualEndDate)
	assert.Equal(t, 650, stored.HoursPerStoryPointHundredths)
	assert.Equal(t, int64(1), countProjectRows(t, env.db, ""))
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ?", got.ID))
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", got.ID, userID))
}

// Escenario: Sin descripción (002/US1)
// La descripción es opcional: sin el campo, o con un texto solo de espacios, el proyecto se crea
// igual y la descripción queda nula (FR-003).
func TestUS1_SinDescripcion(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(body map[string]any)
	}{
		{"campo ausente", func(body map[string]any) { delete(body, "description") }},
		{"solo espacios", func(body map[string]any) { body["description"] = "   " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			body := validProjectInputBody()
			tc.mutate(body)

			got := requireCreatedProject(t, env.postProject(t, token, body))

			assert.Nil(t, got.Description, "la descripción debería volver nula")
			assert.Nil(t, findStoredProject(t, env.db, got.ID).Description, "la descripción debería guardarse nula")
		})
	}
}

// Escenario: Mismo nombre, otro propietario (002/US1)
// Otro usuario ya tiene "Portal de Clientes"; este usuario crea el suyo con el mismo nombre y se
// acepta, porque la unicidad del nombre es solo entre los proyectos de un mismo propietario (FR-004).
func TestUS1_MismoNombreOtroPropietario(t *testing.T) {
	env := newProjectsTestEnv(t)
	otherID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	seedOwnedProject(t, env.db, otherID, "Portal de Clientes")
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")

	got := requireCreatedProject(t, env.postProject(t, token, validProjectInputBody()))

	assert.Equal(t, userID.String(), got.Owner.ID)
	assert.Equal(t, int64(2), countProjectRows(t, env.db, "name_normalized = ?", "portal de clientes"))
	assert.Equal(t, int64(1), countProjectRows(t, env.db, "owner_id = ?", userID))
}

// Escenario: Fechas iguales (002/US1)
// Una fecha de finalización prevista igual a la de inicio es válida (FR-005, RN-06).
func TestUS1_FechasIguales(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	body := validProjectInputBody()
	body["startDate"] = "2026-10-01"
	body["plannedEndDate"] = "2026-10-01"

	got := requireCreatedProject(t, env.postProject(t, token, body))

	assert.Equal(t, "2026-10-01", got.StartDate)
	assert.Equal(t, "2026-10-01", got.PlannedEndDate)
}

// Escenario: Límites del factor (002/US1)
// El mismo usuario crea un proyecto con factor 0,01 y luego otro con factor 40: ambos se crean
// porque son el mínimo y el máximo admitidos, y se guardan como 1 y 4000 centésimas (FR-006, RN-07).
func TestUS1_LimitesDelFactor(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	cases := []struct {
		name       string
		factor     json.Number
		want       float64
		hundredths int
	}{
		{"mínimo 0,01", "0.01", 0.01, 1},
		{"máximo 40", "40", 40, 4000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := validProjectInputBody()
			body["name"] = "Proyecto con factor " + string(tc.factor)
			body["hoursPerStoryPoint"] = tc.factor

			got := requireCreatedProject(t, env.postProject(t, token, body))

			assert.InDelta(t, tc.want, got.HoursPerStoryPoint, 1e-9)
			assert.Equal(t, tc.hundredths, findStoredProject(t, env.db, got.ID).HoursPerStoryPointHundredths)
		})
	}
	assert.Equal(t, int64(2), countProjectRows(t, env.db, ""), "deberían existir los dos proyectos")
}

// Escenario: Límites del nombre (002/US1)
// Un nombre de exactamente 100 caracteres se acepta y uno de 101 se rechaza indicando el campo, sin
// crear nada (FR-002, RN-04).
func TestUS1_LimitesDelNombre(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")

	t.Run("100 caracteres válido", func(t *testing.T) {
		name := strings.Repeat("a", 100)
		body := validProjectInputBody()
		body["name"] = name

		got := requireCreatedProject(t, env.postProject(t, token, body))

		assert.Equal(t, name, got.Name)
	})

	t.Run("101 caracteres rechazado", func(t *testing.T) {
		body := validProjectInputBody()
		body["name"] = strings.Repeat("b", 101)

		requireFieldRejected(t, env.postProject(t, token, body), "name", "100")
		assert.Zero(t, countProjectRows(t, env.db, "name_normalized = ?", strings.Repeat("b", 101)))
	})
}

// Escenario: Campos faltantes (002/US1)
// Sin nombre, sin fecha de inicio, sin fecha de finalización prevista o sin factor, la creación se
// rechaza con 400 indicando qué campo corregir y no se crea ningún proyecto ni membresía (FR-010).
func TestUS1_CamposFaltantes(t *testing.T) {
	cases := []struct {
		name  string
		field string
	}{
		{"sin nombre", "name"},
		{"sin fecha de inicio", "startDate"},
		{"sin fecha de finalización prevista", "plannedEndDate"},
		{"sin factor", "hoursPerStoryPoint"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			body := validProjectInputBody()
			delete(body, tc.field)

			requireFieldRejected(t, env.postProject(t, token, body), tc.field)
			requireNothingCreated(t, env.db)
		})
	}
}

// Escenario: Factor inválido (002/US1)
// Un factor 0, negativo, de 40,01 o con tres decimales (6,555) se rechaza sin redondear, indicando
// el rango y la precisión admitidos, y no se crea nada (FR-006, RN-07).
func TestUS1_FactorInvalido(t *testing.T) {
	cases := []struct {
		name   string
		factor json.Number
	}{
		{"0", "0"},
		{"negativo", "-1"},
		{"40,01", "40.01"},
		{"6,555", "6.555"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			body := validProjectInputBody()
			body["hoursPerStoryPoint"] = tc.factor

			requireFieldRejected(t, env.postProject(t, token, body), "hoursPerStoryPoint", "40", "2 decimales")
			requireNothingCreated(t, env.db)
		})
	}
}

// Escenario: Fecha prevista anterior (002/US1)
// Una fecha de finalización prevista anterior a la de inicio se rechaza indicando que no puede ser
// anterior, y no se crea nada (FR-005, RN-06).
func TestUS1_FechaPrevistaAnterior(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	body := validProjectInputBody()
	body["startDate"] = "2026-10-01"
	body["plannedEndDate"] = "2026-09-30"

	requireFieldRejected(t, env.postProject(t, token, body), "plannedEndDate", "anterior")
	requireNothingCreated(t, env.db)
}

// Escenario: Nombre repetido (002/US1)
// El usuario ya tiene "Portal de Clientes" e intenta crear "  portal de clientes  ": se rechaza con
// 409 por nombre repetido, comparando sin mayúsculas ni espacios extremos, y no se crea un segundo
// proyecto (FR-004, RN-05).
func TestUS1_NombreRepetido(t *testing.T) {
	env := newProjectsTestEnv(t)
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	seedOwnedProject(t, env.db, userID, "Portal de Clientes")
	body := validProjectInputBody()
	body["name"] = "  portal de clientes  "

	rec := env.postProject(t, token, body)

	require.Equal(t, http.StatusConflict, rec.Code, "cuerpo: %s", rec.Body.String())
	var got projectErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, projectNameTakenMessage, got.Error)
	assert.Equal(t, int64(1), countProjectRows(t, env.db, "owner_id = ?", userID))
	assert.Equal(t, int64(1), countMemberRows(t, env.db, ""))
}

// Escenario: Sin sesión (002/US1)
// Sin sesión válida (sin cabecera o con un token adulterado), la creación se rechaza con 401
// pidiendo iniciar sesión y no se crea nada (FR-046).
func TestUS1_SinSesion(t *testing.T) {
	cases := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			_, valid := env.signUp(t, "Ana Pérez", "ana@example.com")

			rec := env.postProject(t, tc.token(valid), validProjectInputBody())

			require.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
			var got projectErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "el cuerpo del 401 no es JSON válido: %s", rec.Body.String())
			assert.Equal(t, projectSessionMessage, got.Error)
			requireNothingCreated(t, env.db)
		})
	}
}

// projectListMessage es la indicación en español que acompaña al listado vacío (FR-034).
const projectListMessage = "Todavía no participás en ningún proyecto."

// projectListItemBody refleja ProjectListItem del contrato.
type projectListItemBody struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	StartDate      string `json:"startDate"`
	PlannedEndDate string `json:"plannedEndDate"`
	MemberCount    int    `json:"memberCount"`
	IsOwner        bool   `json:"isOwner"`
}

// projectListBody refleja ProjectListResponse del contrato. RawProjects guarda además el texto
// crudo de projects, para distinguir una lista vacía de un campo nulo o ausente.
type projectListBody struct {
	Projects    []projectListItemBody `json:"projects"`
	RawProjects json.RawMessage       `json:"-"`
	Message     *string               `json:"message"`
}

// listedProjectSeed describe un proyecto a sembrar para los escenarios del listado: nombre, estado,
// fechas como texto AAAA-MM-DD e integrantes adicionales al propietario.
type listedProjectSeed struct {
	name           string
	status         domain.ProjectStatus
	startDate      string
	plannedEndDate string
	extraMembers   []uuid.UUID
}

// get envía un GET a la ruta indicada con el token como Bearer; con token vacío la solicitud va sin
// cabecera Authorization.
func (e projectsTestEnv) get(t *testing.T, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// requireProjectList exige un 200 cuyo cuerpo sea un ProjectListResponse y lo devuelve decodificado.
func requireProjectList(t *testing.T, rec *httptest.ResponseRecorder) projectListBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectListBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw), "el cuerpo del 200 no es un objeto JSON: %s", rec.Body.String())
	body.RawProjects = raw["projects"]
	return body
}

// seedListedProject inserta directamente en la base un proyecto del propietario con los datos del
// listado y las membresías del propietario y de cada integrante adicional, y devuelve su
// identificador, para preparar el "Dado" sin pasar por la API que se está probando.
func seedListedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, seed listedProjectSeed) uuid.UUID {
	t.Helper()
	start, err := time.Parse(time.DateOnly, seed.startDate)
	require.NoError(t, err, "fecha de inicio de prueba inválida")
	plannedEnd, err := time.Parse(time.DateOnly, seed.plannedEndDate)
	require.NoError(t, err, "fecha prevista de prueba inválida")
	project := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         seed.name,
		NameNormalized:               strings.ToLower(strings.TrimSpace(seed.name)),
		StartDate:                    start,
		PlannedEndDate:               plannedEnd,
		HoursPerStoryPointHundredths: 650,
		Status:                       seed.status,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto %q", seed.name)
	for _, userID := range append([]uuid.UUID{ownerID}, seed.extraMembers...) {
		member := domain.ProjectMember{ID: uuid.New(), ProjectID: project.ID, UserID: userID}
		require.NoError(t, db.Create(&member).Error, "no se pudo sembrar la membresía de %s", userID)
	}
	return project.ID
}

// Escenario: Listado completo (002/US2)
// Un usuario propietario de dos proyectos e integrante de un tercero ve los tres, cada uno con
// nombre, estado, fechas, cantidad de integrantes y si es su propietario, y no ve el proyecto ajeno
// del que no participa (FR-031 a FR-033). El orden es fecha de inicio descendente y, a igual fecha,
// nombre ascendente (FR-035): "Backoffice" y "Portal de Clientes" empiezan el mismo día y se
// siembran en orden inverso al esperado, para que el orden no salga del orden de inserción.
func TestUS2_ListadoCompleto(t *testing.T) {
	env := newProjectsTestEnv(t)
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	otherID, _ := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	thirdID, _ := env.signUp(t, "Carla Gómez", "carla@example.com")

	appID := seedListedProject(t, env.db, userID, listedProjectSeed{
		name: "App Móvil", status: domain.ProjectStatusPlanned,
		startDate: "2026-09-15", plannedEndDate: "2026-11-30",
	})
	portalID := seedListedProject(t, env.db, userID, listedProjectSeed{
		name: "Portal de Clientes", status: domain.ProjectStatusInProgress,
		startDate: "2026-10-01", plannedEndDate: "2026-12-20", extraMembers: []uuid.UUID{otherID, thirdID},
	})
	backofficeID := seedListedProject(t, env.db, otherID, listedProjectSeed{
		name: "Backoffice", status: domain.ProjectStatusFinished,
		startDate: "2026-10-01", plannedEndDate: "2026-10-31", extraMembers: []uuid.UUID{userID},
	})
	seedListedProject(t, env.db, thirdID, listedProjectSeed{
		name: "Proyecto Ajeno", status: domain.ProjectStatusPlanned,
		startDate: "2026-11-01", plannedEndDate: "2026-12-01", extraMembers: []uuid.UUID{otherID},
	})

	got := requireProjectList(t, env.get(t, token, projectsPath))

	// Aparecen exactamente los tres proyectos del usuario, en el orden de FR-035 y con todos sus
	// datos; el proyecto ajeno no aparece.
	want := []projectListItemBody{
		{ID: backofficeID.String(), Name: "Backoffice", Status: "finished", StartDate: "2026-10-01", PlannedEndDate: "2026-10-31", MemberCount: 2, IsOwner: false},
		{ID: portalID.String(), Name: "Portal de Clientes", Status: "in_progress", StartDate: "2026-10-01", PlannedEndDate: "2026-12-20", MemberCount: 3, IsOwner: true},
		{ID: appID.String(), Name: "App Móvil", Status: "planned", StartDate: "2026-09-15", PlannedEndDate: "2026-11-30", MemberCount: 1, IsOwner: true},
	}
	assert.Equal(t, want, got.Projects)
	assert.Nil(t, got.Message, "con proyectos en la lista el mensaje debería ser nulo")
}

// Escenario: Listado vacío (002/US2)
// Un usuario recién registrado que no participa en ningún proyecto recibe 200 con una lista vacía,
// no nula, y la indicación de que todavía no participa en ninguno: no es un error (FR-034). Que
// existan proyectos de otras personas no cambia el resultado (FR-033).
func TestUS2_ListadoVacio(t *testing.T) {
	env := newProjectsTestEnv(t)
	otherID, _ := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	seedOwnedProject(t, env.db, otherID, "Portal de Clientes")
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")

	got := requireProjectList(t, env.get(t, token, projectsPath))

	assert.JSONEq(t, `[]`, string(got.RawProjects), "projects debería ser una lista vacía, no nula ni ausente")
	assert.Empty(t, got.Projects)
	if assert.NotNil(t, got.Message, "la lista vacía debería venir con su indicación") {
		assert.Equal(t, projectListMessage, *got.Message)
	}
}

// Escenario: Integrante quitado no ve el proyecto (002/US2)
// Un usuario que era integrante de un proyecto y fue quitado ya no lo ve en su listado, pero sigue
// viendo el resto de sus proyectos. La baja se prepara borrando su membresía en la base, porque el
// endpoint de baja pertenece a US3.
func TestUS2_IntegranteQuitadoNoVeElProyecto(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, _ := env.signUp(t, "Bruno Díaz", "bruno@example.com")
	userID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	removedFrom := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "Portal de Clientes", status: domain.ProjectStatusInProgress,
		startDate: "2026-10-01", plannedEndDate: "2026-12-20", extraMembers: []uuid.UUID{userID},
	})
	stillIn := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "App Móvil", status: domain.ProjectStatusPlanned,
		startDate: "2026-09-15", plannedEndDate: "2026-11-30", extraMembers: []uuid.UUID{userID},
	})
	require.NoError(t, env.db.Where("project_id = ? AND user_id = ?", removedFrom, userID).
		Delete(&domain.ProjectMember{}).Error, "no se pudo quitar la membresía de prueba")

	got := requireProjectList(t, env.get(t, token, projectsPath))

	if assert.Len(t, got.Projects, 1, "solo debería quedar el proyecto del que sigue siendo integrante") {
		assert.Equal(t, stillIn.String(), got.Projects[0].ID)
	}
	for _, p := range got.Projects {
		assert.NotEqual(t, removedFrom.String(), p.ID, "el proyecto del que fue quitado no debería aparecer")
	}
}

// Escenario: Proyecto con un integrante (002/US2)
// Un proyecto creado por la API, cuyo único integrante es su propietario, aparece en el listado del
// propietario con memberCount 1 e isOwner verdadero (RN-01).
func TestUS2_ProyectoConUnIntegrante(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	created := requireCreatedProject(t, env.postProject(t, token, validProjectInputBody()))

	got := requireProjectList(t, env.get(t, token, projectsPath))

	want := []projectListItemBody{
		{ID: created.ID, Name: "Portal de Clientes", Status: "planned", StartDate: "2026-10-01", PlannedEndDate: "2026-12-20", MemberCount: 1, IsOwner: true},
	}
	assert.Equal(t, want, got.Projects)
}

// Escenario: Proyecto ajeno es inexistente (002/US2)
// Quien no es integrante de un proyecto existente, creado por su propietario con la API, no lo ve en
// su listado y, al consultarlo con su identificador, recibe la misma respuesta 404 que con un
// identificador que no existe (FR-033, FR-049, SC-005). La consulta se hace sobre el manejador
// protegido de prueba de middleware_test.go, montado en el router completo detrás del middleware de
// sesión y de RequireMembership, porque la vista de estado pertenece a US4.
func TestUS2_ProyectoAjenoEsInexistente(t *testing.T) {
	env := newProjectsTestEnv(t)
	probe := &membershipProbe{}
	env.router.GET(membershipProbeRoute,
		authdelivery.NewHandler(env.auth).RequireSession,
		RequireMembership(repository.NewProjectMemberRepository(env.db)),
		probe.handle,
	)
	_, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
	foreign := requireCreatedProject(t, env.postProject(t, ownerToken, validProjectInputBody()))

	// El proyecto ajeno no aparece en el listado de quien no participa.
	list := requireProjectList(t, env.get(t, outsiderToken, projectsPath))
	assert.Empty(t, list.Projects, "el listado no debería incluir el proyecto ajeno")

	// La consulta del proyecto ajeno y la de un identificador inexistente son el mismo 404, byte a
	// byte, y ninguna alcanza el manejador protegido.
	foreignRec := env.get(t, outsiderToken, "/api/v1/solo-prueba-membresia/"+foreign.ID)
	missingRec := env.get(t, outsiderToken, "/api/v1/solo-prueba-membresia/"+uuid.New().String())
	assert.Equal(t, http.StatusNotFound, foreignRec.Code, "cuerpo: %s", foreignRec.Body.String())
	assert.JSONEq(t, `{"error":"`+projectNotFoundMessage+`"}`, foreignRec.Body.String())
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Header().Get("Content-Type"), foreignRec.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

// Escenario: Sin sesión (002/US2)
// Sin sesión válida (sin cabecera o con un token adulterado), el listado se rechaza con 401
// pidiendo iniciar sesión, aunque la cuenta tenga proyectos (FR-046).
func TestUS2_SinSesion(t *testing.T) {
	cases := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			userID, valid := env.signUp(t, "Ana Pérez", "ana@example.com")
			seedOwnedProject(t, env.db, userID, "Portal de Clientes")

			rec := env.get(t, tc.token(valid), projectsPath)

			require.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.JSONEq(t, `{"error":"`+projectSessionMessage+`"}`, rec.Body.String())
		})
	}
}

// Textos exactos que fija el contrato para las respuestas de error del alta y la baja de
// integrantes.
const (
	memberEmailNotRegisteredMessage = "No existe un usuario registrado con ese correo electrónico."
	memberAlreadyMemberMessage      = "La persona ya forma parte del proyecto."
	memberNotOwnerMessage           = "Solo el propietario del proyecto puede realizar esta acción."
	memberOwnerCannotLeaveMessage   = "El propietario no puede quitarse del proyecto."
	memberNotMemberMessage          = "La persona no forma parte del proyecto."
	projectReadOnlyMessage          = "El proyecto está finalizado y es de solo lectura."
)

// memberViewBody refleja ProjectMemberView del contrato. Email es un puntero para distinguir el
// campo ausente, que es como se oculta a quien no es propietario (FR-030).
type memberViewBody struct {
	UserID   string  `json:"userId"`
	FullName string  `json:"fullName"`
	Email    *string `json:"email"`
	IsOwner  bool    `json:"isOwner"`
	JoinedAt string  `json:"joinedAt"`
}

// membersPath es la ruta de alta de integrantes del proyecto según contracts/openapi.yaml.
func membersPath(projectID uuid.UUID) string {
	return projectsPath + "/" + projectID.String() + "/members"
}

// memberPath es la ruta de baja de un integrante concreto según contracts/openapi.yaml.
func memberPath(projectID, userID uuid.UUID) string {
	return membersPath(projectID) + "/" + userID.String()
}

// projectDetailPath es la ruta de la vista de estado del proyecto, la única vía por la que la API
// expone la lista de integrantes (research.md sección 12).
func projectDetailPath(projectID uuid.UUID) string {
	return projectsPath + "/" + projectID.String()
}

// postMember envía el correo a POST /api/v1/projects/{projectId}/members con el token como Bearer.
func (e projectsTestEnv) postMember(t *testing.T, token string, projectID uuid.UUID, email string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"email": email})
	require.NoError(t, err, "no se pudo serializar la solicitud")
	req := httptest.NewRequest(http.MethodPost, membersPath(projectID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// deleteMember envía DELETE /api/v1/projects/{projectId}/members/{userId} con el token como Bearer.
func (e projectsTestEnv) deleteMember(t *testing.T, token string, projectID, userID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, memberPath(projectID, userID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// mountMembershipProbe monta en el router el manejador protegido de prueba de middleware_test.go
// detrás del middleware de sesión y de RequireMembership, para comprobar si una persona puede
// consultar el proyecto sin depender de la vista de estado de US4.
func (e projectsTestEnv) mountMembershipProbe() {
	probe := &membershipProbe{}
	e.router.GET(membershipProbeRoute,
		authdelivery.NewHandler(e.auth).RequireSession,
		RequireMembership(repository.NewProjectMemberRepository(e.db)),
		probe.handle,
	)
}

// accessStatus devuelve el código con que responde el manejador protegido de prueba cuando la
// persona del token consulta el proyecto: 200 si tiene acceso y 404 si no.
func (e projectsTestEnv) accessStatus(t *testing.T, token string, projectID uuid.UUID) int {
	t.Helper()
	return e.get(t, token, "/api/v1/solo-prueba-membresia/"+projectID.String()).Code
}

// seedProjectInStatus siembra un proyecto del propietario en el estado indicado, con las
// membresías del propietario y de los integrantes adicionales, y devuelve su identificador.
func seedProjectInStatus(t *testing.T, db *gorm.DB, ownerID uuid.UUID, status domain.ProjectStatus, extraMembers ...uuid.UUID) uuid.UUID {
	t.Helper()
	return seedListedProject(t, db, ownerID, listedProjectSeed{
		name: "Portal de Clientes", status: status,
		startDate: "2026-10-01", plannedEndDate: "2026-12-20", extraMembers: extraMembers,
	})
}

// requireErrorResponse exige el código indicado con un ErrorResponse cuyo mensaje sea el esperado.
func requireErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	require.Equal(t, status, rec.Code, "cuerpo: %s", rec.Body.String())
	var got projectErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), "el cuerpo no es JSON válido: %s", rec.Body.String())
	assert.Equal(t, message, got.Error)
}

// requireCreatedMember exige un 201 cuyo cuerpo sea un ProjectMemberView y lo devuelve decodificado.
func requireCreatedMember(t *testing.T, rec *httptest.ResponseRecorder) memberViewBody {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, "cuerpo: %s", rec.Body.String())
	var body memberViewBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 201 no es JSON válido: %s", rec.Body.String())
	return body
}

// memberRowsOf devuelve las filas de project_members del proyecto ordenadas por usuario, para
// comparar la lista de integrantes antes y después de una acción.
func memberRowsOf(t *testing.T, db *gorm.DB, projectID uuid.UUID) []domain.ProjectMember {
	t.Helper()
	var rows []domain.ProjectMember
	require.NoError(t, db.Where("project_id = ?", projectID).Order("user_id").Find(&rows).Error,
		"no se pudieron leer las membresías del proyecto")
	return rows
}

// requireMemberList exige un 200 de la vista de estado y devuelve su lista de integrantes como
// objetos crudos, para poder comprobar que el campo email está ausente y no solo vacío.
func requireMemberList(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body struct {
		Members []map[string]any `json:"members"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body.Members
}

// tableSnapshots toma, para cada tabla del esquema salvo project_members, la cantidad de filas y un
// resumen md5 de su contenido completo. Comparar dos instantáneas detecta cualquier fila borrada,
// agregada o modificada, incluida una referencia al autor puesta en nulo, en las tablas de hoy y en
// las que agreguen las features siguientes (RN-14, SC-011).
func tableSnapshots(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' AND table_name <> 'project_members'
		ORDER BY table_name`).Scan(&tables).Error, "no se pudieron listar las tablas")
	snapshots := make(map[string]string, len(tables))
	for _, table := range tables {
		var snapshot string
		query := fmt.Sprintf(`SELECT count(*) || ':' || coalesce(md5(string_agg(t::text, ',' ORDER BY t::text)), '') FROM %q t`, table)
		require.NoError(t, db.Raw(query).Scan(&snapshot).Error, "no se pudo resumir la tabla %s", table)
		snapshots[table] = snapshot
	}
	return snapshots
}

// Escenario: Alta exitosa (002/US3)
// El propietario agrega a Bruno, registrado con bruno@example.com: la respuesta es 201 con la vista
// del nuevo integrante, queda la membresía en la base y, desde ese momento, Bruno puede consultar el
// proyecto y lo ve en su listado (FR-019, FR-028).
func TestUS3_AltaExitosa(t *testing.T) {
	env := newProjectsTestEnv(t)
	env.mountMembershipProbe()
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)

	rec := env.postMember(t, ownerToken, projectID, "bruno@example.com")
	got := requireCreatedMember(t, rec)
	assertNoPasswordHash(t, rec)

	// La respuesta describe a Bruno; quien la recibe es el propietario, así que incluye el correo.
	assert.Equal(t, brunoID.String(), got.UserID)
	assert.Equal(t, "Bruno Gómez", got.FullName)
	if assert.NotNil(t, got.Email, "el propietario debería ver el correo del nuevo integrante") {
		assert.Equal(t, "bruno@example.com", *got.Email)
	}
	assert.False(t, got.IsOwner)
	_, err := time.Parse(time.RFC3339, got.JoinedAt)
	assert.NoError(t, err, "joinedAt = %q no es una fecha y hora RFC 3339", got.JoinedAt)
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", projectID, brunoID))

	// Bruno ya puede consultar el proyecto y lo ve en su listado, sin ser su propietario.
	assert.Equal(t, http.StatusOK, env.accessStatus(t, brunoToken, projectID), "Bruno debería tener acceso al proyecto")
	list := requireProjectList(t, env.get(t, brunoToken, projectsPath))
	if assert.Len(t, list.Projects, 1, "Bruno debería ver el proyecto en su listado") {
		assert.Equal(t, projectID.String(), list.Projects[0].ID)
		assert.False(t, list.Projects[0].IsOwner)
		assert.Equal(t, 2, list.Projects[0].MemberCount)
	}
}

// Escenario: Correo con espacios y mayúsculas (002/US3)
// El propietario escribe "  Bruno@Example.COM  " y el sistema encuentra la misma cuenta registrada
// con bruno@example.com, porque el correo se compara sin espacios extremos y sin distinguir
// mayúsculas (FR-020, RN-05).
func TestUS3_CorreoConEspaciosYMayusculas(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)

	got := requireCreatedMember(t, env.postMember(t, ownerToken, projectID, "  Bruno@Example.COM  "))

	assert.Equal(t, brunoID.String(), got.UserID)
	if assert.NotNil(t, got.Email) {
		assert.Equal(t, "bruno@example.com", *got.Email, "el correo devuelto es el normalizado de la cuenta")
	}
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", projectID, brunoID))
}

// Escenario: Nueva alta tras una baja (002/US3)
// Bruno fue quitado del proyecto y el propietario lo vuelve a agregar: recupera el acceso, con una
// fecha de incorporación nueva, y sigue apareciendo una sola vez en la lista de integrantes (FR-029).
// La baja previa se prepara borrando su membresía en la base, para aislar el escenario del endpoint
// de baja.
func TestUS3_NuevaAltaTrasBaja(t *testing.T) {
	env := newProjectsTestEnv(t)
	env.mountMembershipProbe()
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
	var previous domain.ProjectMember
	require.NoError(t, env.db.Where("project_id = ? AND user_id = ?", projectID, brunoID).Take(&previous).Error)
	require.NoError(t, env.db.Where("project_id = ? AND user_id = ?", projectID, brunoID).
		Delete(&domain.ProjectMember{}).Error, "no se pudo quitar la membresía de prueba")
	require.Equal(t, http.StatusNotFound, env.accessStatus(t, brunoToken, projectID), "precondición: Bruno no tiene acceso")

	got := requireCreatedMember(t, env.postMember(t, ownerToken, projectID, "bruno@example.com"))

	// Bruno recupera el acceso y figura una sola vez entre los dos integrantes del proyecto.
	assert.Equal(t, brunoID.String(), got.UserID)
	assert.Equal(t, http.StatusOK, env.accessStatus(t, brunoToken, projectID), "Bruno debería recuperar el acceso")
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ? AND user_id = ?", projectID, brunoID))
	assert.Equal(t, int64(2), countMemberRows(t, env.db, "project_id = ?", projectID))

	// La nueva membresía es una fila nueva, con su propia fecha de incorporación.
	var current domain.ProjectMember
	require.NoError(t, env.db.Where("project_id = ? AND user_id = ?", projectID, brunoID).Take(&current).Error)
	assert.NotEqual(t, previous.ID, current.ID, "la nueva alta debería ser una membresía nueva")
	assert.True(t, current.CreatedAt.After(previous.CreatedAt), "la fecha de incorporación debería ser nueva")
}

// Escenario: Visibilidad del correo (002/US3)
// En un proyecto con propietario y dos integrantes más, el propietario ve de cada persona su nombre,
// su correo, la marca de propietario y la fecha de incorporación; cualquier otro integrante ve
// exactamente los mismos datos salvo los correos, que no aparecen ni como campo vacío (FR-030). La
// lista se consulta en la vista de estado, que es la única vía por la que la API la expone
// (research.md sección 12).
func TestUS3_VisibilidadDelCorreo(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	carlaID, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID, carlaID)

	ownerView := requireMemberList(t, env.get(t, ownerToken, projectDetailPath(projectID)))

	// El propietario ve a las tres personas con el correo de cada una.
	wantEmails := map[string]string{
		ownerID.String(): "ana@example.com",
		brunoID.String(): "bruno@example.com",
		carlaID.String(): "carla@example.com",
	}
	require.Len(t, ownerView, 3, "el propietario debería ver a los tres integrantes")
	for _, m := range ownerView {
		userID, _ := m["userId"].(string)
		assert.Equal(t, wantEmails[userID], m["email"], "correo de %s", userID)
	}

	// Sin los correos, la vista del propietario es la que debe recibir cualquier otro integrante.
	withoutEmails := make([]map[string]any, 0, len(ownerView))
	for _, m := range ownerView {
		copied := make(map[string]any, len(m))
		for k, v := range m {
			if k != "email" {
				copied[k] = v
			}
		}
		withoutEmails = append(withoutEmails, copied)
	}

	// Ningún integrante que no es propietario recibe el campo email, y el resto es idéntico.
	viewers := []struct {
		name  string
		token string
	}{{"Bruno", brunoToken}, {"Carla", carlaToken}}
	for _, viewer := range viewers {
		t.Run(viewer.name, func(t *testing.T) {
			view := requireMemberList(t, env.get(t, viewer.token, projectDetailPath(projectID)))
			for _, m := range view {
				assert.NotContains(t, m, "email", "un integrante que no es propietario no debería recibir correos")
			}
			assert.Equal(t, withoutEmails, view, "salvo los correos, los datos deberían ser los mismos que ve el propietario")
		})
	}
}

// Escenario: Solo el propietario como integrante (002/US3)
// En un proyecto recién creado, cuyo único integrante es su propietario, la lista de integrantes que
// consulta el propietario no está vacía: lo contiene a él, marcado como propietario (RN-01).
func TestUS3_SoloElPropietarioComoIntegrante(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	created := requireCreatedProject(t, env.postProject(t, ownerToken, validProjectInputBody()))
	projectID, err := uuid.Parse(created.ID)
	require.NoError(t, err)

	members := requireMemberList(t, env.get(t, ownerToken, projectDetailPath(projectID)))

	if assert.Len(t, members, 1, "la lista debería contener solo al propietario") {
		assert.Equal(t, ownerID.String(), members[0]["userId"])
		assert.Equal(t, "Ana Pérez", members[0]["fullName"])
		assert.Equal(t, true, members[0]["isOwner"], "el único integrante debería estar marcado propietario")
	}
}

// Escenario: La baja conserva el historial (002/US3)
// Al quitar a Bruno, la respuesta es 204 y lo único que cambia en toda la base es su fila de
// project_members: ninguna otra tabla pierde, agrega ni modifica filas, de modo que nada de lo que
// registró se pierde ni queda sin autor, y su cuenta sigue con su nombre (FR-027, RN-14, SC-011). El
// resumen abarca todas las tablas del esquema, así que alcanza también al esfuerzo, los votos y los
// defectos cuando las features 005 a 007 creen sus tablas. Bruno pierde el acceso de inmediato
// (FR-028) y los demás integrantes conservan el suyo.
func TestUS3_BajaConservaElHistorial(t *testing.T) {
	env := newProjectsTestEnv(t)
	env.mountMembershipProbe()
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	carlaID, carlaToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID, carlaID)
	before := tableSnapshots(t, env.db)
	var remaining []domain.ProjectMember
	for _, m := range memberRowsOf(t, env.db, projectID) {
		if m.UserID != brunoID {
			remaining = append(remaining, m)
		}
	}

	rec := env.deleteMember(t, ownerToken, projectID, brunoID)

	require.Equal(t, http.StatusNoContent, rec.Code, "cuerpo: %s", rec.Body.String())

	// Solo desaparece la membresía de Bruno; las demás filas de project_members quedan intactas.
	assert.Equal(t, remaining, memberRowsOf(t, env.db, projectID))

	// Ninguna otra tabla cambia y la cuenta de Bruno conserva su nombre.
	assert.Equal(t, before, tableSnapshots(t, env.db), "la baja no debería tocar ninguna tabla salvo project_members")
	var fullName string
	require.NoError(t, env.db.Raw("SELECT full_name FROM users WHERE id = ?", brunoID).Scan(&fullName).Error)
	assert.Equal(t, "Bruno Gómez", fullName)

	// Bruno pierde el acceso de inmediato; Carla y el propietario lo conservan.
	assert.Equal(t, http.StatusNotFound, env.accessStatus(t, brunoToken, projectID), "Bruno debería perder el acceso")
	assert.Equal(t, http.StatusOK, env.accessStatus(t, carlaToken, projectID))
	assert.Equal(t, http.StatusOK, env.accessStatus(t, ownerToken, projectID))
}

// Escenario: Correo sin cuenta (002/US3)
// El propietario intenta agregar nadie@example.com, que no corresponde a ninguna cuenta: la
// respuesta es 422, y no 404, informando que no existe un usuario registrado con ese correo, y no se
// crea ninguna membresía (FR-021, RC-03).
func TestUS3_CorreoSinCuenta(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)

	rec := env.postMember(t, ownerToken, projectID, "nadie@example.com")

	assert.NotEqual(t, http.StatusNotFound, rec.Code, "el 404 queda reservado para el proyecto inexistente")
	requireErrorResponse(t, rec, http.StatusUnprocessableEntity, memberEmailNotRegisteredMessage)
	assert.Equal(t, int64(1), countMemberRows(t, env.db, "project_id = ?", projectID))
}

// Escenario: Ya es integrante (002/US3)
// Bruno ya es integrante y el propietario intenta agregarlo otra vez: la respuesta es 409 informando
// que ya forma parte del proyecto, y la lista de integrantes no cambia (FR-022, RN-13).
func TestUS3_YaEsIntegrante(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
	before := memberRowsOf(t, env.db, projectID)

	rec := env.postMember(t, ownerToken, projectID, "bruno@example.com")

	requireErrorResponse(t, rec, http.StatusConflict, memberAlreadyMemberMessage)
	assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "la lista de integrantes no debería cambiar")
}

// Escenario: No es propietario (002/US3)
// Bruno es integrante pero no propietario: su intento de agregar a Carla y su intento de quitar a
// Daniel se rechazan con 403 por falta de permiso, y la lista de integrantes no cambia (FR-047,
// FR-050, SC-006).
func TestUS3_NoEsPropietario(t *testing.T) {
	cases := []struct {
		name string
		act  func(t *testing.T, env projectsTestEnv, token string, projectID, danielID uuid.UUID) *httptest.ResponseRecorder
	}{
		{"alta", func(t *testing.T, env projectsTestEnv, token string, projectID, _ uuid.UUID) *httptest.ResponseRecorder {
			return env.postMember(t, token, projectID, "carla@example.com")
		}},
		{"baja", func(t *testing.T, env projectsTestEnv, token string, projectID, danielID uuid.UUID) *httptest.ResponseRecorder {
			return env.deleteMember(t, token, projectID, danielID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			env.signUp(t, "Carla Díaz", "carla@example.com")
			danielID, _ := env.signUp(t, "Daniel Ruiz", "daniel@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID, danielID)
			before := memberRowsOf(t, env.db, projectID)

			rec := tc.act(t, env, brunoToken, projectID, danielID)

			requireErrorResponse(t, rec, http.StatusForbidden, memberNotOwnerMessage)
			assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "la acción no debería tener ningún efecto")
		})
	}
}

// Escenario: El propietario no puede quitarse (002/US3)
// El propietario intenta quitarse a sí mismo: la respuesta es 409 indicando que el propietario no
// puede quitarse del proyecto, y sigue siendo integrante (FR-025, RN-12).
func TestUS3_PropietarioNoPuedeQuitarse(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
	before := memberRowsOf(t, env.db, projectID)

	rec := env.deleteMember(t, ownerToken, projectID, ownerID)

	requireErrorResponse(t, rec, http.StatusConflict, memberOwnerCannotLeaveMessage)
	assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "el propietario debería seguir siendo integrante")
}

// Escenario: Proyecto finalizado (002/US3)
// Sobre un proyecto en estado Finalizado, el propietario intenta agregar a Carla y quitar a Bruno:
// las dos acciones se rechazan con 409 porque el proyecto es de solo lectura, y la lista de
// integrantes no cambia (FR-018, RN-11).
func TestUS3_ProyectoFinalizado(t *testing.T) {
	cases := []struct {
		name string
		act  func(t *testing.T, env projectsTestEnv, token string, projectID, brunoID uuid.UUID) *httptest.ResponseRecorder
	}{
		{"alta", func(t *testing.T, env projectsTestEnv, token string, projectID, _ uuid.UUID) *httptest.ResponseRecorder {
			return env.postMember(t, token, projectID, "carla@example.com")
		}},
		{"baja", func(t *testing.T, env projectsTestEnv, token string, projectID, brunoID uuid.UUID) *httptest.ResponseRecorder {
			return env.deleteMember(t, token, projectID, brunoID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			env.signUp(t, "Carla Díaz", "carla@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusFinished, brunoID)
			before := memberRowsOf(t, env.db, projectID)

			rec := tc.act(t, env, ownerToken, projectID, brunoID)

			requireErrorResponse(t, rec, http.StatusConflict, projectReadOnlyMessage)
			assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "la lista de integrantes no debería cambiar")
		})
	}
}

// Escenario: Baja de una persona que no es integrante (002/US3)
// El propietario intenta quitar a una persona que no forma parte del proyecto: la respuesta es 404
// informando que la persona no forma parte del proyecto, y la lista de integrantes no cambia
// (FR-026). Se cubren una cuenta registrada que no es integrante y un userId que no es un UUID, que
// recibe la misma respuesta.
func TestUS3_BajaDeQuienNoEsIntegrante(t *testing.T) {
	cases := []struct {
		name string
		act  func(t *testing.T, env projectsTestEnv, token string, projectID uuid.UUID) *httptest.ResponseRecorder
	}{
		{"cuenta registrada que no es integrante", func(t *testing.T, env projectsTestEnv, token string, projectID uuid.UUID) *httptest.ResponseRecorder {
			carlaID, _ := env.signUp(t, "Carla Díaz", "carla@example.com")
			return env.deleteMember(t, token, projectID, carlaID)
		}},
		{"userId que no es UUID", func(t *testing.T, env projectsTestEnv, token string, projectID uuid.UUID) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodDelete, membersPath(projectID)+"/no-es-un-uuid", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			env.router.ServeHTTP(rec, req)
			return rec
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
			before := memberRowsOf(t, env.db, projectID)

			rec := tc.act(t, env, ownerToken, projectID)

			requireErrorResponse(t, rec, http.StatusNotFound, memberNotMemberMessage)
			assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "la lista de integrantes no debería cambiar")
		})
	}
}

// Escenario: Alta y baja sin sesión (002/US3)
// Sin sesión válida (sin cabecera o con un token adulterado del propietario), el alta de Carla y la
// baja de Bruno, que con la sesión del propietario se aceptarían, se rechazan con 401 pidiendo
// iniciar sesión, y la lista de integrantes no cambia (FR-046, SC-004).
func TestUS3_SinSesion(t *testing.T) {
	actions := []struct {
		name string
		req  func(projectID, brunoID uuid.UUID) *http.Request
	}{
		{"alta", func(projectID, _ uuid.UUID) *http.Request {
			req := httptest.NewRequest(http.MethodPost, membersPath(projectID), strings.NewReader(`{"email":"carla@example.com"}`))
			req.Header.Set("Content-Type", "application/json")
			return req
		}},
		{"baja", func(projectID, brunoID uuid.UUID) *http.Request {
			return httptest.NewRequest(http.MethodDelete, memberPath(projectID, brunoID), nil)
		}},
	}
	tokens := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, action := range actions {
		for _, tk := range tokens {
			t.Run(action.name+"/"+tk.name, func(t *testing.T) {
				env := newProjectsTestEnv(t)
				ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
				brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
				env.signUp(t, "Carla Díaz", "carla@example.com")
				projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
				before := memberRowsOf(t, env.db, projectID)

				// Arma la solicitud y agrega la cabecera Authorization solo si hay token
				req := action.req(projectID, brunoID)
				if token := tk.token(ownerToken); token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				rec := httptest.NewRecorder()
				env.router.ServeHTTP(rec, req)

				requireErrorResponse(t, rec, http.StatusUnauthorized, projectSessionMessage)
				assert.Equal(t, before, memberRowsOf(t, env.db, projectID), "la lista de integrantes no debería cambiar")
			})
		}
	}
}

// fixedBacklog es un doble del puerto BacklogSummaryProvider que informa siempre el mismo resumen,
// para preparar el "Dado" de un backlog con historias sin sembrarlas en la base.
type fixedBacklog struct {
	summary domain.BacklogSummary
}

// Summarize devuelve el resumen fijado, sin importar el proyecto.
func (f fixedBacklog) Summarize(context.Context, uuid.UUID) (domain.BacklogSummary, error) {
	return f.summary, nil
}

// fixedSprint es un doble del puerto SprintDirectory que informa siempre el mismo sprint activo,
// para preparar el "Dado" de un sprint activo sin depender de la spec 004.
type fixedSprint struct {
	sprint *domain.ActiveSprint
}

// FindActiveSprint devuelve el sprint fijado, sin importar el proyecto.
func (f fixedSprint) FindActiveSprint(context.Context, uuid.UUID) (*domain.ActiveSprint, error) {
	return f.sprint, nil
}

// queryCounter es un Logger de GORM que no escribe nada y cuenta cada sentencia que GORM ejecuta.
// GORM llama a Trace una vez por sentencia, sea cual sea el nivel de log.
type queryCounter struct {
	count atomic.Int64
}

// LogMode, Info, Warn y Error completan gormlogger.Interface sin registrar nada.
func (q *queryCounter) LogMode(gormlogger.LogLevel) gormlogger.Interface { return q }
func (q *queryCounter) Info(context.Context, string, ...any)             {}
func (q *queryCounter) Warn(context.Context, string, ...any)             {}
func (q *queryCounter) Error(context.Context, string, ...any)            {}

// Trace suma una sentencia ejecutada.
func (q *queryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	q.count.Add(1)
}

// newStatusTestEnv compone el router como newProjectsTestEnv, pero con los puertos del backlog y de
// sprints recibidos en lugar de los de cmd/app; el servicio de cambio de estado usa el
// mismo puerto de sprints, para preparar el sprint activo de US6. Si recibe un contador, los
// repositorios que usan los servicios trabajan sobre una sesión de GORM con ese Logger, mientras que
// la sesión de la 001 y los middlewares de membresía y propiedad usan la conexión sin contar: así se
// cuentan solo las consultas con que el servicio arma la vista de estado (SC-003). Si backlog es nil,
// usa el proveedor real del resumen del backlog sobre esa misma sesión, para que su consulta también
// se cuente.
func newStatusTestEnv(t *testing.T, backlog domain.BacklogSummaryProvider, sprints domain.SprintDirectory, counter *queryCounter) projectsTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	serviceDB := db
	if counter != nil {
		serviceDB = db.Session(&gorm.Session{Logger: counter})
	}
	if backlog == nil {
		backlog = repository.NewBacklogSummaryProvider(serviceDB)
	}
	projectRepository := repository.NewProjectRepository(serviceDB)
	projectMembers := repository.NewProjectMemberRepository(serviceDB)
	projects := service.NewProjectService(projectRepository, projectMembers, backlog, sprints)
	members := service.NewMemberService(projectRepository, projectMembers, repository.NewUserDirectory(serviceDB))
	status := service.NewStatusService(projectRepository, sprints, cfg.Location, time.Now)
	router := gin.New()
	router.Use(httpserver.LimitRequestBody())
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)
	projectGroup, adminGroup := RegisterRoutes(router, authHandler.RequireSession, NewHandler(projects, members, status),
		repository.NewProjectMemberRepository(db), repository.NewProjectRepository(db))
	return projectsTestEnv{router: router, db: db, auth: auth, projectGroup: projectGroup, adminGroup: adminGroup, location: cfg.Location}
}

// requireProjectDetail exige un 200 de la vista de estado y devuelve el cuerpo decodificado como
// ProjectDetail y como objeto crudo, para distinguir un campo nulo de uno ausente.
func requireProjectDetail(t *testing.T, rec *httptest.ResponseRecorder) (projectDetailBody, map[string]json.RawMessage) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw), "el cuerpo del 200 no es un objeto JSON: %s", rec.Body.String())
	return body, raw
}

// setProjectStatus fija en la base el estado del proyecto, porque el cambio de estado por la API
// pertenece a US6.
func setProjectStatus(t *testing.T, db *gorm.DB, projectID string, status domain.ProjectStatus) {
	t.Helper()
	require.NoError(t, db.Model(&domain.Project{}).Where("id = ?", projectID).Update("status", status).Error,
		"no se pudo fijar el estado del proyecto")
}

// Escenario: Estado completo (002/US4)
// Un proyecto En curso con tres integrantes, un sprint activo y un backlog con historias en
// distintos estados: un integrante que no es el propietario obtiene en una sola respuesta todos los
// campos de FR-042, con el propietario identificado, el sprint activo, los contadores por estado y
// los Story Points estimados con su equivalente en horas. El sprint lo informa un doble del puerto,
// porque la spec 004 todavía no existe, y el backlog otro doble, que fija el resumen sin sembrar
// historias; el proveedor real se verifica en las pruebas de projects/repository.
func TestUS4_EstadoCompleto(t *testing.T) {
	sprintID := uuid.New()
	env := newStatusTestEnv(t,
		fixedBacklog{summary: domain.BacklogSummary{
			CountsByState:        map[string]int{"to_do": 2, "in_progress": 1, "done": 1},
			EstimatedStoryPoints: 16,
		}},
		fixedSprint{sprint: &domain.ActiveSprint{ID: sprintID, Name: "Sprint 4"}},
		nil,
	)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	carlaID, _ := env.signUp(t, "Carla Díaz", "carla@example.com")
	created := requireCreatedProject(t, env.postProject(t, ownerToken, validProjectInputBody()))
	projectID := uuid.MustParse(created.ID)
	requireCreatedMember(t, env.postMember(t, ownerToken, projectID, "bruno@example.com"))
	requireCreatedMember(t, env.postMember(t, ownerToken, projectID, "carla@example.com"))
	setProjectStatus(t, env.db, created.ID, domain.ProjectStatusInProgress)

	rec := env.get(t, brunoToken, projectDetailPath(projectID))
	got, raw := requireProjectDetail(t, rec)
	assertNoPasswordHash(t, rec)

	// Todos los campos de FR-042 llegan en la misma respuesta.
	for _, key := range []string{"id", "name", "description", "status", "startDate", "plannedEndDate", "actualEndDate",
		"hoursPerStoryPoint", "readOnly", "owner", "members", "hasActiveSprint", "activeSprint", "backlog"} {
		assert.Contains(t, raw, key, "la vista de estado debería incluir %q", key)
	}

	// Datos generales, estado, fechas y factor.
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "Portal de Clientes", got.Name)
	if assert.NotNil(t, got.Description) {
		assert.Equal(t, "Rediseño del portal de autogestión.", *got.Description)
	}
	assert.Equal(t, "in_progress", got.Status)
	assert.Equal(t, "2026-10-01", got.StartDate)
	assert.Equal(t, "2026-12-20", got.PlannedEndDate)
	assert.Nil(t, got.ActualEndDate, "un proyecto en curso no tiene fecha de finalización real")
	assert.InDelta(t, 6.5, got.HoursPerStoryPoint, 0.0001)
	assert.False(t, got.ReadOnly)

	// Propietario e integrantes, con el propietario identificado.
	assert.Equal(t, ownerID.String(), got.Owner.ID)
	assert.Equal(t, "Ana Pérez", got.Owner.FullName)
	owners := map[string]bool{}
	for _, m := range got.Members {
		owners[m.UserID] = m.IsOwner
	}
	assert.Equal(t, map[string]bool{ownerID.String(): true, brunoID.String(): false, carlaID.String(): false}, owners)

	// Sprint activo y resumen del backlog con sus horas (16 × 6,5 = 104).
	assert.True(t, got.HasActiveSprint)
	assert.JSONEq(t, `{"id":"`+sprintID.String()+`","name":"Sprint 4"}`, string(raw["activeSprint"]))
	assert.Equal(t, map[string]int{"to_do": 2, "in_progress": 1, "done": 1}, got.Backlog.CountsByState)
	assert.Equal(t, 16, got.Backlog.EstimatedStoryPoints)
	assert.InDelta(t, 104.0, got.Backlog.EstimatedHours, 0.0001)
}

// Escenario: Proyecto finalizado (002/US4)
// Un integrante que no es el propietario consulta un proyecto Finalizado: además de los demás datos
// recibe la fecha de finalización real y la indicación de solo lectura (FR-018, FR-039, FR-048). El
// proyecto se siembra finalizado en la base, con su fecha real, porque la transición es de US6.
func TestUS4_ProyectoFinalizado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusFinished, brunoID)
	actualEnd := time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)
	require.NoError(t, env.db.Model(&domain.Project{}).Where("id = ?", projectID).
		Update("actual_end_date", actualEnd).Error, "no se pudo fijar la fecha de finalización real")

	got, _ := requireProjectDetail(t, env.get(t, brunoToken, projectDetailPath(projectID)))

	assert.Equal(t, "finished", got.Status)
	assert.True(t, got.ReadOnly, "un proyecto finalizado debería indicarse de solo lectura")
	if assert.NotNil(t, got.ActualEndDate, "un proyecto finalizado debería traer su fecha de finalización real") {
		assert.Equal(t, "2026-12-18", *got.ActualEndDate)
	}
	assert.Equal(t, "Portal de Clientes", got.Name)
	assert.Equal(t, "2026-10-01", got.StartDate)
	assert.Equal(t, "2026-12-20", got.PlannedEndDate)
	assert.Len(t, got.Members, 2)
}

// Escenario: Sin sprint activo (002/US4)
// Un proyecto sin sprint activo responde 200, con hasActiveSprint en falso y activeSprint presente y
// nulo: la ausencia se informa de forma explícita y no como un error (FR-043). Se usa el adaptador
// provisorio de sprints, que es el que hoy compone cmd/app.
func TestUS4_SinSprintActivo(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)

	got, raw := requireProjectDetail(t, env.get(t, brunoToken, projectDetailPath(projectID)))

	assert.False(t, got.HasActiveSprint)
	if assert.Contains(t, raw, "activeSprint", "activeSprint debería estar presente aunque sea nulo") {
		assert.JSONEq(t, `null`, string(raw["activeSprint"]))
	}
	assert.NotContains(t, raw, "error", "la falta de sprint activo no es un error")
}

// Escenario: Proyecto recién creado (002/US4)
// El propietario de un proyecto recién creado por la API, sin backlog ni sprints, obtiene los datos
// generales con los contadores por estado vacíos, los Story Points y las horas estimadas en cero y la
// indicación de que no hay sprint activo (FR-043, FR-044).
func TestUS4_ProyectoReciénCreado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	created := requireCreatedProject(t, env.postProject(t, token, validProjectInputBody()))

	got, raw := requireProjectDetail(t, env.get(t, token, projectDetailPath(uuid.MustParse(created.ID))))

	assert.Equal(t, "Portal de Clientes", got.Name)
	assert.Equal(t, "planned", got.Status)
	assert.Equal(t, ownerID.String(), got.Owner.ID)
	var backlog map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["backlog"], &backlog), "backlog debería ser un objeto: %s", raw["backlog"])
	assert.JSONEq(t, `{}`, string(backlog["countsByState"]), "countsByState debería ser un objeto vacío, no nulo")
	assert.Equal(t, 0, got.Backlog.EstimatedStoryPoints)
	assert.Zero(t, got.Backlog.EstimatedHours)
	assert.False(t, got.HasActiveSprint)
	assert.JSONEq(t, `null`, string(raw["activeSprint"]))
}

// Escenario: Horas estimadas (002/US4)
// Con factor 6,5 y un backlog con historias estimadas en 3, 5 y 8 Story Points y una sin estimar, la
// vista informa 16 Story Points y 104 horas: la historia sin estimar se cuenta por estado pero no
// suma (FR-045, RN-08). La suma de Story Points la resuelve el puerto del backlog, así que el doble
// informa 16 y la prueba verifica la conversión a horas con el factor del proyecto.
func TestUS4_HorasEstimadas(t *testing.T) {
	env := newStatusTestEnv(t,
		fixedBacklog{summary: domain.BacklogSummary{
			CountsByState:        map[string]int{"to_do": 4},
			EstimatedStoryPoints: 3 + 5 + 8,
		}},
		repository.NewSprintDirectory(),
		nil,
	)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)

	got, _ := requireProjectDetail(t, env.get(t, brunoToken, projectDetailPath(projectID)))

	assert.InDelta(t, 6.5, got.HoursPerStoryPoint, 0.0001)
	assert.Equal(t, map[string]int{"to_do": 4}, got.Backlog.CountsByState)
	assert.Equal(t, 16, got.Backlog.EstimatedStoryPoints)
	assert.InDelta(t, 104.0, got.Backlog.EstimatedHours, 0.0001)
}

// Escenario: No integrante es inexistente (002/US4)
// Quien no es integrante consulta el estado de un proyecto existente y recibe el mismo 404 que con
// un identificador inexistente, byte a byte, sin ningún dato del proyecto, ni siquiera su nombre
// (FR-049, SC-005).
func TestUS4_NoIntegranteEsInexistente(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	created := requireCreatedProject(t, env.postProject(t, ownerToken, validProjectInputBody()))

	foreignRec := env.get(t, outsiderToken, projectDetailPath(uuid.MustParse(created.ID)))
	missingRec := env.get(t, outsiderToken, projectDetailPath(uuid.New()))

	requireErrorResponse(t, foreignRec, http.StatusNotFound, projectNotFoundMessage)
	assert.Equal(t, missingRec.Code, foreignRec.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missingRec.Body.Bytes(), foreignRec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	assert.NotContains(t, foreignRec.Body.String(), "Portal de Clientes", "la respuesta no debería revelar el nombre")
	assert.NotContains(t, foreignRec.Body.String(), created.ID, "la respuesta no debería revelar el identificador")
}

// Escenario: Sin sesión (002/US4)
// Sin sesión válida (sin cabecera o con un token adulterado del propietario), la consulta del estado
// se rechaza con 401 pidiendo iniciar sesión (FR-046, SC-004).
func TestUS4_SinSesion(t *testing.T) {
	cases := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, valid := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)

			rec := env.get(t, tc.token(valid), projectDetailPath(projectID))

			requireErrorResponse(t, rec, http.StatusUnauthorized, projectSessionMessage)
		})
	}
}

// Escenario: Vista de estado sin N+1 (002/US4)
// SC-003 pide la vista de estado en menos de 3 segundos; una aserción de tiempo de pared sería
// inestable, así que se verifica lo que el diseño exige para cumplirlo: el servicio arma la vista en
// a lo sumo 4 consultas (proyecto, integrantes con JOIN, resumen del backlog y sprint activo) y esa
// cantidad no crece al agregar integrantes. Se cuentan solo las consultas de los repositorios del
// servicio, sin la sesión de la 001 ni la verificación de membresía del middleware. Hoy se cuentan
// 3 (proyecto, integrantes y resumen del backlog), porque el directorio de sprints todavía no
// consulta la base: la cuarta llega con la spec 004.
func TestUS4_VistaDeEstadoSinNMasUno(t *testing.T) {
	counter := &queryCounter{}
	env := newStatusTestEnv(t, nil, repository.NewSprintDirectory(), counter)
	_, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	created := requireCreatedProject(t, env.postProject(t, ownerToken, validProjectInputBody()))
	projectID := uuid.MustParse(created.ID)

	// Cuenta las consultas de una única petición a la vista de estado.
	queriesOfView := func(t *testing.T) int64 {
		t.Helper()
		counter.count.Store(0)
		got, _ := requireProjectDetail(t, env.get(t, ownerToken, projectDetailPath(projectID)))
		require.NotEmpty(t, got.Members)
		return counter.count.Load()
	}

	// El contador tiene que registrar consultas: en cero, el Logger no estaría conectado y la
	// aserción del límite pasaría sin verificar nada.
	withOwnerOnly := queriesOfView(t)
	t.Logf("consultas del servicio con un integrante: %d", withOwnerOnly)
	require.Positive(t, withOwnerOnly, "el contador debería registrar las consultas de la vista")
	assert.LessOrEqual(t, withOwnerOnly, int64(4), "la vista de estado debería resolverse en a lo sumo 4 consultas")

	// Se suman cinco integrantes y la cantidad de consultas debe ser la misma.
	for i := range 5 {
		email := fmt.Sprintf("integrante%d@example.com", i)
		env.signUp(t, fmt.Sprintf("Integrante %d", i), email)
		requireCreatedMember(t, env.postMember(t, ownerToken, projectID, email))
	}
	withSixMembers := queriesOfView(t)
	assert.Equal(t, withOwnerOnly, withSixMembers, "la cantidad de consultas no debería crecer con los integrantes")
}

// putProject envía el cuerpo a PUT /api/v1/projects/{projectId} con el token recibido como Bearer.
func (e projectsTestEnv) putProject(t *testing.T, token string, projectID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err, "no se pudo serializar la solicitud")
	req := httptest.NewRequest(http.MethodPut, projectDetailPath(projectID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// updatedProjectInputBody devuelve un conjunto completo de valores nuevos y válidos para el proyecto
// sembrado con seedProjectInStatus; cada prueba modifica solo el campo que le interesa.
func updatedProjectInputBody() map[string]any {
	return map[string]any{
		"name":               "Portal de Clientes v2",
		"description":        "Rediseño del portal de autogestión, fase 2.",
		"startDate":          "2026-10-05",
		"plannedEndDate":     "2027-02-28",
		"hoursPerStoryPoint": json.Number("8"),
	}
}

// requireUpdatedProject exige un 200 cuyo cuerpo sea un ProjectDetail y lo devuelve decodificado.
func requireUpdatedProject(t *testing.T, rec *httptest.ResponseRecorder) projectDetailBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	return body
}

// Escenario: Modificación exitosa (002/US5)
// El propietario de un proyecto Planificado cambia nombre, descripción, fechas y factor por valores
// válidos: la respuesta es 200 con el proyecto actualizado, la base y la vista de estado reflejan los
// nuevos valores, y el estado y la fecha de finalización real no cambian (FR-012, FR-016). El cuerpo
// trae además un actualEndDate, que se ignora porque esa fecha no se edita por ninguna vía (FR-040).
func TestUS5_ModificacionExitosa(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
	body := updatedProjectInputBody()
	body["actualEndDate"] = "2026-11-30"

	rec := env.putProject(t, token, projectID, body)
	got := requireUpdatedProject(t, rec)
	assertNoPasswordHash(t, rec)

	// La respuesta trae los valores nuevos con el estado y la fecha real intactos.
	assert.Equal(t, projectID.String(), got.ID)
	assert.Equal(t, "Portal de Clientes v2", got.Name)
	if assert.NotNil(t, got.Description) {
		assert.Equal(t, "Rediseño del portal de autogestión, fase 2.", *got.Description)
	}
	assert.Equal(t, "2026-10-05", got.StartDate)
	assert.Equal(t, "2027-02-28", got.PlannedEndDate)
	assert.InDelta(t, 8.0, got.HoursPerStoryPoint, 1e-9)
	assert.Equal(t, "planned", got.Status)
	assert.Nil(t, got.ActualEndDate, "la fecha de finalización real no debería cambiar")
	assert.Equal(t, ownerID.String(), got.Owner.ID)

	// La base guarda los valores nuevos, con el nombre normalizado recalculado.
	stored := findStoredProject(t, env.db, projectID.String())
	assert.Equal(t, "Portal de Clientes v2", stored.Name)
	assert.Equal(t, "portal de clientes v2", stored.NameNormalized)
	assert.Equal(t, "2026-10-05", stored.StartDate.Format(time.DateOnly))
	assert.Equal(t, "2027-02-28", stored.PlannedEndDate.Format(time.DateOnly))
	assert.Equal(t, 800, stored.HoursPerStoryPointHundredths)
	assert.Equal(t, domain.ProjectStatusPlanned, stored.Status)
	assert.Nil(t, stored.ActualEndDate)

	// La vista de estado refleja los nuevos valores.
	view, _ := requireProjectDetail(t, env.get(t, token, projectDetailPath(projectID)))
	assert.Equal(t, "Portal de Clientes v2", view.Name)
	assert.Equal(t, "2027-02-28", view.PlannedEndDate)
	assert.InDelta(t, 8.0, view.HoursPerStoryPoint, 1e-9)
}

// Escenario: Proyecto en curso se modifica (002/US5)
// Sobre un proyecto En curso, el propietario corre la fecha de finalización prevista a una fecha
// posterior: el cambio se acepta porque solo el estado Finalizado bloquea las modificaciones, y el
// proyecto sigue En curso (FR-016, FR-018).
func TestUS5_ProyectoEnCursoSeModifica(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	body := validProjectInputBody()
	body["plannedEndDate"] = "2027-01-31"

	got := requireUpdatedProject(t, env.putProject(t, token, projectID, body))

	assert.Equal(t, "2027-01-31", got.PlannedEndDate)
	assert.Equal(t, "in_progress", got.Status)
	assert.Nil(t, got.ActualEndDate)
	stored := findStoredProject(t, env.db, projectID.String())
	assert.Equal(t, "2027-01-31", stored.PlannedEndDate.Format(time.DateOnly))
	assert.Equal(t, domain.ProjectStatusInProgress, stored.Status)
}

// Escenario: Mismo nombre que ya tenía (002/US5)
// El propietario renombra el proyecto con el mismo nombre que ya tenía, tal cual o con otra
// capitalización y espacios extremos: el cambio se acepta porque la unicidad del nombre excluye al
// propio proyecto (FR-014), y el resto de los datos se actualiza.
func TestUS5_MismoNombreQueYaTenia(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantName string
	}{
		{"idéntico", "Portal de Clientes", "Portal de Clientes"},
		{"otra capitalización y espacios", "  PORTAL de clientes  ", "PORTAL de clientes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
			body := updatedProjectInputBody()
			body["name"] = tc.input

			got := requireUpdatedProject(t, env.putProject(t, token, projectID, body))

			assert.Equal(t, tc.wantName, got.Name)
			assert.Equal(t, "2027-02-28", got.PlannedEndDate, "el resto de los datos debería actualizarse")
			assert.Equal(t, int64(1), countProjectRows(t, env.db, "owner_id = ?", ownerID))
		})
	}
}

// Escenario: Cambio de factor con sprints cerrados (002/US5)
// El propietario de un proyecto En curso cambia el factor de 6 a 8: el proyecto pasa a 8 y ninguna
// otra tabla cambia, de modo que los sprints cerrados conservan el factor vigente al cerrarse y sus
// métricas no se recalculan (FR-017, RN-15). Las tablas de sprints las agrega la spec 004; la
// instantánea de todas las tablas salvo projects y project_members las incluirá cuando existan.
func TestUS5_CambioDeFactorConSprintsCerrados(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	require.NoError(t, env.db.Model(&domain.Project{}).Where("id = ?", projectID).
		Update("hours_per_story_point_hundredths", 600).Error, "no se pudo fijar el factor 6")
	membersBefore := memberRowsOf(t, env.db, projectID)
	before := tableSnapshots(t, env.db)
	delete(before, "projects")
	body := validProjectInputBody()
	body["hoursPerStoryPoint"] = json.Number("8")

	got := requireUpdatedProject(t, env.putProject(t, token, projectID, body))

	assert.InDelta(t, 8.0, got.HoursPerStoryPoint, 1e-9)
	assert.Equal(t, 800, findStoredProject(t, env.db, projectID.String()).HoursPerStoryPointHundredths)
	after := tableSnapshots(t, env.db)
	delete(after, "projects")
	assert.Equal(t, before, after, "el cambio del factor no debería tocar ninguna otra tabla")
	assert.Equal(t, membersBefore, memberRowsOf(t, env.db, projectID), "las membresías no deberían cambiar")
}

// Escenario: Límites del factor (002/US5)
// El propietario cambia el factor al mínimo (0,01) y al máximo (40) admitidos: ambos cambios se
// aceptan y se guardan como 1 y 4000 centésimas (FR-006, FR-013, RN-07).
func TestUS5_LimitesDelFactor(t *testing.T) {
	cases := []struct {
		name       string
		factor     json.Number
		want       float64
		hundredths int
	}{
		{"mínimo 0,01", "0.01", 0.01, 1},
		{"máximo 40", "40", 40, 4000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
			body := validProjectInputBody()
			body["hoursPerStoryPoint"] = tc.factor

			got := requireUpdatedProject(t, env.putProject(t, token, projectID, body))

			assert.InDelta(t, tc.want, got.HoursPerStoryPoint, 1e-9)
			assert.Equal(t, tc.hundredths, findStoredProject(t, env.db, projectID.String()).HoursPerStoryPointHundredths)
		})
	}
}

// Escenario: Nombre de otro proyecto propio (002/US5)
// El propietario tiene "Portal de Clientes" y "Backoffice" e intenta renombrar el segundo como
// "  portal de clientes  ": la modificación se rechaza con 409 por nombre repetido y el proyecto
// queda exactamente como estaba, sin aplicar tampoco los demás cambios (FR-004, FR-014, FR-015).
func TestUS5_NombreDeOtroProyectoPropio(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
	projectID := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "Backoffice", status: domain.ProjectStatusPlanned,
		startDate: "2026-10-01", plannedEndDate: "2026-12-20",
	})
	before := findStoredProject(t, env.db, projectID.String())
	body := updatedProjectInputBody()
	body["name"] = "  portal de clientes  "

	rec := env.putProject(t, token, projectID, body)

	requireErrorResponse(t, rec, http.StatusConflict, projectNameTakenMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto debería quedar como estaba")
}

// Escenario: Entrada inválida sin cambio parcial (002/US5)
// El propietario envía un nombre vacío, una fecha prevista anterior a la de inicio, un factor fuera
// de rango o un factor con más de dos decimales, junto con otros campos válidos y distintos de los
// actuales: la modificación se rechaza con 400 indicando el campo y el motivo, y no se aplica ningún
// cambio, ni siquiera el de los campos válidos (FR-013, FR-015).
func TestUS5_EntradaInvalidaSinCambioParcial(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value any
		parts []string
	}{
		{"nombre vacío", "name", "   ", nil},
		{"fecha prevista anterior", "plannedEndDate", "2026-09-30", []string{"anterior"}},
		{"factor fuera de rango", "hoursPerStoryPoint", json.Number("40.01"), []string{"40"}},
		{"factor con más de dos decimales", "hoursPerStoryPoint", json.Number("6.555"), []string{"2 decimales"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
			before := findStoredProject(t, env.db, projectID.String())
			body := updatedProjectInputBody()
			body["startDate"] = "2026-10-01"
			body[tc.field] = tc.value

			requireFieldRejected(t, env.putProject(t, token, projectID, body), tc.field, tc.parts...)
			assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "no debería aplicarse ningún cambio parcial")
		})
	}
}

// Escenario: No es propietario (002/US5)
// Un integrante que no es el propietario intenta modificar el proyecto: la acción se rechaza con 403
// por falta de permiso y el proyecto no cambia (FR-047).
func TestUS5_NoEsPropietario(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress, brunoID)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.putProject(t, brunoToken, projectID, updatedProjectInputBody())

	requireErrorResponse(t, rec, http.StatusForbidden, memberNotOwnerMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
}

// Escenario: Proyecto finalizado (002/US5)
// El propietario de un proyecto Finalizado intenta modificar sus datos: la acción se rechaza con 409
// porque el proyecto es de solo lectura, y nada cambia (FR-018, RN-11).
func TestUS5_ProyectoFinalizado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusFinished)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.putProject(t, token, projectID, updatedProjectInputBody())

	requireErrorResponse(t, rec, http.StatusConflict, projectReadOnlyMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
}

// Escenario: Sin sesión (002/US5)
// Sin sesión válida (sin cabecera o con un token adulterado del propietario), la modificación de un
// proyecto Planificado, que con la sesión del propietario se aceptaría, se rechaza con 401 pidiendo
// iniciar sesión y el proyecto no cambia (FR-046, SC-004). Prueba de caracterización (T100).
func TestUS5_SinSesion(t *testing.T) {
	cases := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, valid := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
			before := findStoredProject(t, env.db, projectID.String())

			// Arma el PUT con un cuerpo válido y agrega la cabecera Authorization solo si hay token
			payload, err := json.Marshal(updatedProjectInputBody())
			require.NoError(t, err, "no se pudo serializar la solicitud")
			req := httptest.NewRequest(http.MethodPut, projectDetailPath(projectID), bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			if token := tc.token(valid); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			env.router.ServeHTTP(rec, req)

			requireErrorResponse(t, rec, http.StatusUnauthorized, projectSessionMessage)
			assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
		})
	}
}

// Textos exactos que fija el contrato para cada 409 del cambio de estado: salto de planned a
// finished, retroceso, transición al estado actual y finalización con un sprint activo. El de solo
// lectura es projectReadOnlyMessage, compartido con las demás acciones de administración.
const (
	statusSkipMessage         = "El proyecto debe pasar primero a En curso antes de finalizarse."
	statusBackwardMessage     = "Los estados del proyecto solo avanzan: no se puede volver a un estado anterior."
	statusSameMessage         = "El proyecto ya está En curso."
	statusActiveSprintMessage = "No se puede finalizar el proyecto mientras tenga un sprint activo. Cerralo primero."
)

// projectStatusPath es la ruta del cambio de estado del proyecto según contracts/openapi.yaml.
func projectStatusPath(projectID uuid.UUID) string {
	return projectDetailPath(projectID) + "/status"
}

// patchStatus envía el estado destino a PATCH /api/v1/projects/{projectId}/status con el token como
// Bearer, con el cuerpo StatusChangeRequest del contrato.
func (e projectsTestEnv) patchStatus(t *testing.T, token string, projectID uuid.UUID, status domain.ProjectStatus) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"status": string(status)})
	require.NoError(t, err, "no se pudo serializar la solicitud")
	req := httptest.NewRequest(http.MethodPatch, projectStatusPath(projectID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// dayInSystemZone devuelve la fecha de calendario (AAAA-MM-DD) que resulta de desplazar en days días
// el momento actual en la zona horaria del sistema, la misma con que el servicio fija la fecha de
// finalización real (FR-039).
func (e projectsTestEnv) dayInSystemZone(days int) string {
	return time.Now().In(e.location).AddDate(0, 0, days).Format(time.DateOnly)
}

// requireStatusChanged exige un 200 cuyo cuerpo sea el ProjectDetail del proyecto con el estado
// destino, y lo devuelve decodificado.
func requireStatusChanged(t *testing.T, rec *httptest.ResponseRecorder, want domain.ProjectStatus) projectDetailBody {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
	var body projectDetailBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 200 no es JSON válido: %s", rec.Body.String())
	require.Equal(t, string(want), body.Status, "el proyecto debería quedar en el estado destino")
	return body
}

// requireDateOfChange exige que la fecha de finalización real exista y sea la fecha del cambio en
// la zona del sistema. Se aceptan las fechas de antes y de después de la solicitud para que la
// prueba no falle si el cambio ocurre justo a la medianoche.
func requireDateOfChange(t *testing.T, got *string, before, after string) {
	t.Helper()
	require.NotNil(t, got, "la fecha de finalización real debería quedar registrada")
	assert.Contains(t, []string{before, after}, *got, "la fecha real debería ser la del cambio en la zona del sistema")
}

// storedActualEndDate devuelve la fecha de finalización real guardada en projects como AAAA-MM-DD,
// o nil si la columna es nula.
func storedActualEndDate(t *testing.T, db *gorm.DB, projectID uuid.UUID) *string {
	t.Helper()
	stored := findStoredProject(t, db, projectID.String())
	if stored.ActualEndDate == nil {
		return nil
	}
	date := stored.ActualEndDate.Format(time.DateOnly)
	return &date
}

// seedFinishedProject siembra un proyecto Finalizado del propietario, con su fecha de finalización
// real ya registrada y los integrantes adicionales, porque el "Dado" no debe pasar por la transición
// que se está probando.
func seedFinishedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, actualEndDate string, extraMembers ...uuid.UUID) uuid.UUID {
	t.Helper()
	projectID := seedProjectInStatus(t, db, ownerID, domain.ProjectStatusFinished, extraMembers...)
	actualEnd, err := time.Parse(time.DateOnly, actualEndDate)
	require.NoError(t, err, "fecha de finalización real de prueba inválida")
	require.NoError(t, db.Model(&domain.Project{}).Where("id = ?", projectID).
		Update("actual_end_date", actualEnd).Error, "no se pudo fijar la fecha de finalización real")
	return projectID
}

// Escenario: Recorrido completo (002/US6)
// El propietario pasa un proyecto Planificado a En curso y después a Finalizado: ambas transiciones
// responden 200 con el ProjectDetail actualizado; En curso todavía no tiene fecha real, y al
// finalizar queda registrada, en la respuesta, en la base y en la vista de estado, la fecha del
// cambio en la zona del sistema, con el proyecto de solo lectura (FR-037, FR-039, FR-018).
func TestUS6_RecorridoCompleto(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "Portal de Clientes", status: domain.ProjectStatusPlanned,
		startDate: env.dayInSystemZone(-10), plannedEndDate: env.dayInSystemZone(60),
	})

	// Primera transición: planned → in_progress, sin fecha real.
	startRec := env.patchStatus(t, token, projectID, domain.ProjectStatusInProgress)
	started := requireStatusChanged(t, startRec, domain.ProjectStatusInProgress)
	assertNoPasswordHash(t, startRec)
	assert.Equal(t, projectID.String(), started.ID)
	assert.Nil(t, started.ActualEndDate, "un proyecto En curso no tiene fecha de finalización real")
	assert.False(t, started.ReadOnly)
	assert.Equal(t, domain.ProjectStatusInProgress, findStoredProject(t, env.db, projectID.String()).Status)
	assert.Nil(t, storedActualEndDate(t, env.db, projectID))

	// Segunda transición: in_progress → finished, con la fecha del cambio registrada.
	before := env.dayInSystemZone(0)
	finishRec := env.patchStatus(t, token, projectID, domain.ProjectStatusFinished)
	finished := requireStatusChanged(t, finishRec, domain.ProjectStatusFinished)
	after := env.dayInSystemZone(0)
	assertNoPasswordHash(t, finishRec)
	requireDateOfChange(t, finished.ActualEndDate, before, after)
	assert.True(t, finished.ReadOnly, "un proyecto finalizado debería indicarse de solo lectura")
	assert.Equal(t, domain.ProjectStatusFinished, findStoredProject(t, env.db, projectID.String()).Status)
	requireDateOfChange(t, storedActualEndDate(t, env.db, projectID), before, after)

	// La vista de estado muestra la misma fecha real que la respuesta del cambio.
	view, _ := requireProjectDetail(t, env.get(t, token, projectDetailPath(projectID)))
	assert.Equal(t, "finished", view.Status)
	assert.Equal(t, finished.ActualEndDate, view.ActualEndDate)
}

// Escenario: Finalización anticipada (002/US6)
// El propietario finaliza un proyecto En curso cuya fecha prevista todavía no llegó: la transición
// se acepta con 200 y la fecha real queda anterior a la prevista, sin que eso sea un error; la fecha
// prevista no cambia (FR-039).
func TestUS6_FinalizacionAnticipada(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	plannedEnd := env.dayInSystemZone(30)
	projectID := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "Portal de Clientes", status: domain.ProjectStatusInProgress,
		startDate: env.dayInSystemZone(-30), plannedEndDate: plannedEnd,
	})

	before := env.dayInSystemZone(0)
	got := requireStatusChanged(t, env.patchStatus(t, token, projectID, domain.ProjectStatusFinished), domain.ProjectStatusFinished)
	after := env.dayInSystemZone(0)

	requireDateOfChange(t, got.ActualEndDate, before, after)
	assert.Less(t, *got.ActualEndDate, got.PlannedEndDate, "la fecha real debería quedar anterior a la prevista")
	assert.Equal(t, plannedEnd, got.PlannedEndDate, "la fecha prevista no debería cambiar")
	requireDateOfChange(t, storedActualEndDate(t, env.db, projectID), before, after)
}

// Escenario: Finalizado solo se consulta (002/US6)
// Sobre un proyecto Finalizado, un integrante que no es el propietario lo consulta completo, con la
// fecha real y la indicación de solo lectura, mientras que ninguna acción de administración del
// propietario se admite: el cambio de estado, la modificación y el alta y la baja de integrantes se
// rechazan con 409 de solo lectura y el proyecto no cambia (FR-018, FR-048).
func TestUS6_FinalizadoSoloSeConsulta(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	env.signUp(t, "Carla Ruiz", "carla@example.com")
	projectID := seedFinishedProject(t, env.db, ownerID, "2026-12-18", brunoID)

	// Un integrante consulta el proyecto completo.
	view, _ := requireProjectDetail(t, env.get(t, brunoToken, projectDetailPath(projectID)))
	assert.Equal(t, "finished", view.Status)
	assert.True(t, view.ReadOnly)
	if assert.NotNil(t, view.ActualEndDate) {
		assert.Equal(t, "2026-12-18", *view.ActualEndDate)
	}
	assert.Len(t, view.Members, 2)

	// Ninguna acción de administración del propietario se admite.
	actions := []struct {
		name string
		send func(t *testing.T) *httptest.ResponseRecorder
	}{
		{"cambio de estado", func(t *testing.T) *httptest.ResponseRecorder {
			return env.patchStatus(t, ownerToken, projectID, domain.ProjectStatusInProgress)
		}},
		{"modificación", func(t *testing.T) *httptest.ResponseRecorder {
			return env.putProject(t, ownerToken, projectID, updatedProjectInputBody())
		}},
		{"alta de integrante", func(t *testing.T) *httptest.ResponseRecorder {
			return env.postMember(t, ownerToken, projectID, "carla@example.com")
		}},
		{"baja de integrante", func(t *testing.T) *httptest.ResponseRecorder {
			return env.deleteMember(t, ownerToken, projectID, brunoID)
		}},
	}
	for _, action := range actions {
		t.Run(action.name, func(t *testing.T) {
			before := findStoredProject(t, env.db, projectID.String())
			membersBefore := memberRowsOf(t, env.db, projectID)

			requireErrorResponse(t, action.send(t), http.StatusConflict, projectReadOnlyMessage)
			assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
			assert.Equal(t, membersBefore, memberRowsOf(t, env.db, projectID), "los integrantes no deberían cambiar")
		})
	}
}

// Escenario: Finaliza el mismo día que empezó (002/US6)
// El propietario finaliza un proyecto que comenzó hoy: la transición se acepta con 200 y la fecha
// de finalización real es igual a la fecha de inicio (caso límite de FR-039).
func TestUS6_FinalizaElMismoDiaQueEmpezo(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	today := env.dayInSystemZone(0)
	projectID := seedListedProject(t, env.db, ownerID, listedProjectSeed{
		name: "Portal de Clientes", status: domain.ProjectStatusInProgress,
		startDate: today, plannedEndDate: env.dayInSystemZone(30),
	})

	got := requireStatusChanged(t, env.patchStatus(t, token, projectID, domain.ProjectStatusFinished), domain.ProjectStatusFinished)

	require.NotNil(t, got.ActualEndDate, "la fecha de finalización real debería quedar registrada")
	assert.Equal(t, got.StartDate, *got.ActualEndDate, "la fecha real debería ser igual a la de inicio")
	assert.Equal(t, today, got.StartDate)
	if stored := storedActualEndDate(t, env.db, projectID); assert.NotNil(t, stored) {
		assert.Equal(t, today, *stored)
	}
}

// Escenario: Arranque sin condiciones (002/US6)
// El propietario pasa a En curso un proyecto Planificado que ya tiene un sprint activo o un backlog
// cargado: la transición se acepta con 200 sin exigir ninguna condición sobre el backlog ni sobre
// los sprints, y el proyecto queda En curso sin fecha real (FR-041). El sprint y el backlog se
// preparan con dobles de los puertos.
func TestUS6_ArranqueSinCondiciones(t *testing.T) {
	cases := []struct {
		name    string
		backlog domain.BacklogSummaryProvider
		sprints domain.SprintDirectory
	}{
		{
			name:    "con sprint activo",
			backlog: nil,
			sprints: fixedSprint{sprint: &domain.ActiveSprint{ID: uuid.New(), Name: "Sprint 1"}},
		},
		{
			name: "con backlog cargado",
			backlog: fixedBacklog{summary: domain.BacklogSummary{
				CountsByState:        map[string]int{"to_do": 3, "in_progress": 1},
				EstimatedStoryPoints: 13,
			}},
			sprints: repository.NewSprintDirectory(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newStatusTestEnv(t, tc.backlog, tc.sprints, nil)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)

			got := requireStatusChanged(t, env.patchStatus(t, token, projectID, domain.ProjectStatusInProgress), domain.ProjectStatusInProgress)

			assert.Nil(t, got.ActualEndDate, "arrancar el proyecto no registra fecha real")
			assert.Equal(t, domain.ProjectStatusInProgress, findStoredProject(t, env.db, projectID.String()).Status)
			assert.Nil(t, storedActualEndDate(t, env.db, projectID))
		})
	}
}

// Escenario: Retroceso rechazado (002/US6)
// El propietario intenta devolver a Planificado un proyecto En curso: la transición se rechaza con
// 409 indicando que los estados solo avanzan, y el proyecto queda exactamente como estaba (FR-038).
func TestUS6_RetrocesoRechazado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.patchStatus(t, token, projectID, domain.ProjectStatusPlanned)

	requireErrorResponse(t, rec, http.StatusConflict, statusBackwardMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
}

// Escenario: Salto de estado rechazado (002/US6)
// El propietario intenta pasar un proyecto Planificado directamente a Finalizado: la transición se
// rechaza con 409 indicando que primero debe pasar por En curso, y el proyecto sigue Planificado sin
// fecha real (FR-038).
func TestUS6_SaltoDeEstadoRechazado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.patchStatus(t, token, projectID, domain.ProjectStatusFinished)

	requireErrorResponse(t, rec, http.StatusConflict, statusSkipMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
	assert.Nil(t, storedActualEndDate(t, env.db, projectID))
}

// Escenario: Mismo estado rechazado (002/US6)
// El propietario intenta pasar a En curso un proyecto que ya está En curso: la transición se
// rechaza con 409 por inválida y nada cambia (FR-038, SC-009).
func TestUS6_MismoEstadoRechazado(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.patchStatus(t, token, projectID, domain.ProjectStatusInProgress)

	requireErrorResponse(t, rec, http.StatusConflict, statusSameMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
}

// Escenario: Finalizado es solo lectura (002/US6)
// El propietario intenta cambiar el estado de un proyecto Finalizado hacia cualquiera de los tres
// estados: la acción se rechaza con 409 de solo lectura y la fecha de finalización real ya
// registrada no se altera (FR-018, FR-040, RN-10, SC-010).
func TestUS6_FinalizadoEsSoloLectura(t *testing.T) {
	targets := []domain.ProjectStatus{
		domain.ProjectStatusPlanned,
		domain.ProjectStatusInProgress,
		domain.ProjectStatusFinished,
	}
	for _, target := range targets {
		t.Run("hacia "+string(target), func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedFinishedProject(t, env.db, ownerID, "2026-12-18")
			before := findStoredProject(t, env.db, projectID.String())

			rec := env.patchStatus(t, token, projectID, target)

			requireErrorResponse(t, rec, http.StatusConflict, projectReadOnlyMessage)
			assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
			if stored := storedActualEndDate(t, env.db, projectID); assert.NotNil(t, stored) {
				assert.Equal(t, "2026-12-18", *stored, "la fecha de finalización real no debería alterarse")
			}
		})
	}
}

// Escenario: No es propietario (002/US6)
// Un integrante que no es el propietario intenta pasar a En curso un proyecto Planificado: la acción
// se rechaza con 403 por falta de permiso, aunque la transición sería válida, y el proyecto no
// cambia (FR-047).
func TestUS6_NoEsPropietario(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned, brunoID)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.patchStatus(t, brunoToken, projectID, domain.ProjectStatusInProgress)

	requireErrorResponse(t, rec, http.StatusForbidden, memberNotOwnerMessage)
	assert.Equal(t, before, findStoredProject(t, env.db, projectID.String()), "el proyecto no debería cambiar")
}

// Escenario: Finalizar con sprint activo (002/US6)
// El propietario intenta finalizar un proyecto En curso que tiene un sprint activo: la transición
// se rechaza con 409 indicando que primero debe cerrar el sprint, y el proyecto sigue En curso sin
// fecha de finalización real (FR-041, RN-16). El sprint activo se prepara con un doble del puerto
// SprintDirectory.
func TestUS6_FinalizarConSprintActivo(t *testing.T) {
	sprints := fixedSprint{sprint: &domain.ActiveSprint{ID: uuid.New(), Name: "Sprint 4"}}
	env := newStatusTestEnv(t, nil, sprints, nil)
	ownerID, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)
	before := findStoredProject(t, env.db, projectID.String())

	rec := env.patchStatus(t, token, projectID, domain.ProjectStatusFinished)

	requireErrorResponse(t, rec, http.StatusConflict, statusActiveSprintMessage)
	stored := findStoredProject(t, env.db, projectID.String())
	assert.Equal(t, before, stored, "el proyecto no debería cambiar")
	assert.Equal(t, domain.ProjectStatusInProgress, stored.Status, "el proyecto debería seguir En curso")
	assert.Nil(t, stored.ActualEndDate, "no debería registrarse fecha de finalización real")
}

// Escenario: Sin sesión (002/US6)
// Sin sesión válida (sin cabecera o con un token adulterado del propietario), el paso de un proyecto
// Planificado a En curso, que con la sesión del propietario se aceptaría, se rechaza con 401
// pidiendo iniciar sesión y el proyecto no cambia: sigue Planificado y sin fecha de finalización
// real (FR-046, SC-004). Prueba de caracterización (T100).
func TestUS6_SinSesion(t *testing.T) {
	cases := []struct {
		name  string
		token func(valid string) string
	}{
		{"sin cabecera Authorization", func(string) string { return "" }},
		{"token adulterado", func(valid string) string { return valid + "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newProjectsTestEnv(t)
			ownerID, valid := env.signUp(t, "Ana Pérez", "ana@example.com")
			projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusPlanned)
			before := findStoredProject(t, env.db, projectID.String())

			// Arma el PATCH con un estado destino válido y agrega la cabecera Authorization solo si hay token
			req := httptest.NewRequest(http.MethodPatch, projectStatusPath(projectID), strings.NewReader(`{"status":"in_progress"}`))
			req.Header.Set("Content-Type", "application/json")
			if token := tc.token(valid); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			env.router.ServeHTTP(rec, req)

			requireErrorResponse(t, rec, http.StatusUnauthorized, projectSessionMessage)
			stored := findStoredProject(t, env.db, projectID.String())
			assert.Equal(t, before, stored, "el proyecto no debería cambiar")
			assert.Equal(t, domain.ProjectStatusPlanned, stored.Status, "el proyecto debería seguir Planificado")
			assert.Nil(t, stored.ActualEndDate, "no debería registrarse fecha de finalización real")
		})
	}
}

// Pruebas de integración del límite global del cuerpo de las solicitudes (T094, hallazgo de
// seguridad detectado en la US3 de la 002): un JSON de más de 1 MiB se rechaza con 413 o 400 antes de
// que el servicio lo procese. El cuerpo es por lo demás válido: los campos esperados van completos y
// el exceso viaja en un campo que los DTO ignoran, de modo que sin el límite la solicitud llegaría al
// servicio y produciría su efecto.

// maxRequestBodyBytes es el tope del cuerpo de las solicitudes que fija el hallazgo: 1 MiB.
const maxRequestBodyBytes = 1 << 20

// oversizedPadding es el valor de relleno que, solo, ya supera el tope del cuerpo.
func oversizedPadding() string {
	return strings.Repeat("a", maxRequestBodyBytes+1)
}

// requireBodyTooLarge exige que la respuesta sea 413 o 400, los dos rechazos admitidos para un cuerpo
// que excede el tope.
func requireBodyTooLarge(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Contains(t, []int{http.StatusRequestEntityTooLarge, http.StatusBadRequest}, rec.Code,
		"un cuerpo de más de 1 MiB debería rechazarse con 413 o 400, se obtuvo %d", rec.Code)
}

// El propietario envía a POST /api/v1/projects/{projectId}/members el correo de una cuenta registrada
// dentro de un cuerpo de más de 1 MiB: la respuesta es 413 o 400 y la persona no queda como
// integrante, porque la solicitud no llega al servicio de integrantes.
func TestCreateMember_OversizedBody(t *testing.T) {
	env := newProjectsTestEnv(t)
	ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, _ := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := seedProjectInStatus(t, env.db, ownerID, domain.ProjectStatusInProgress)

	payload, err := json.Marshal(map[string]string{"email": "bruno@example.com", "padding": oversizedPadding()})
	require.NoError(t, err, "no se pudo serializar la solicitud")
	require.Greater(t, len(payload), maxRequestBodyBytes, "el cuerpo debería superar 1 MiB")
	req := httptest.NewRequest(http.MethodPost, membersPath(projectID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	requireBodyTooLarge(t, rec)
	assert.Zero(t, countMemberRows(t, env.db, "project_id = ? AND user_id = ?", projectID, brunoID),
		"Bruno no debería haberse agregado como integrante")
}

// Una persona con sesión envía a POST /api/v1/projects un proyecto válido dentro de un cuerpo de más
// de 1 MiB: la respuesta es 413 o 400 y no se crea ningún proyecto ni ninguna membresía.
func TestCreateProject_OversizedBody(t *testing.T) {
	env := newProjectsTestEnv(t)
	_, token := env.signUp(t, "Ana Pérez", "ana@example.com")
	body := validProjectInputBody()
	body["padding"] = oversizedPadding()

	rec := env.postProject(t, token, body)

	requireBodyTooLarge(t, rec)
	requireNothingCreated(t, env.db)
}

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

// projectsTestEnv reúne el router compuesto como en cmd/app, la base de test y el servicio de
// autenticación con el que se crean las cuentas y las sesiones de cada escenario.
type projectsTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	auth   *authservice.AuthService
}

// newProjectsTestEnv abre la base de test vacía y compone la cadena real de la 001 (para el
// middleware de sesión) y del módulo de proyectos, igual que cmd/app, sin CORS ni Swagger.
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
	projects := service.NewProjectService(
		repository.NewProjectRepository(db),
		repository.NewProjectMemberRepository(db),
		repository.NewBacklogSummaryProvider(),
		repository.NewSprintDirectory(),
	)
	router := gin.New()
	authHandler := authdelivery.NewHandler(auth)
	authdelivery.RegisterRoutes(router, authHandler)
	RegisterRoutes(router, authHandler.RequireSession, NewHandler(projects))
	return projectsTestEnv{router: router, db: db, auth: auth}
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

	got := requireCreatedProject(t, env.postProject(t, token, validProjectInputBody()))

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

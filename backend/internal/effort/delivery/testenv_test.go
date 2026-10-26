package delivery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	// time/tzdata embute la base de zonas horarias para que la zona inyectada se cargue también en
	// un contenedor que no la trae, igual que cmd/app.
	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	backlogdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	backlogservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/config"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/httpserver"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	sprintsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/delivery"
	sprintsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
	sprintsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Entorno de las pruebas de integración del registro de esfuerzo (T015). Compone el router como
// cmd/app —sesión de la 001, proyectos de la 002, backlog de la 003, sprints de la 004 y esfuerzo—
// sobre la base smye_test, con un único reloj falso que comparten el servicio de cambio de estado del
// proyecto, el de sprints y el de esfuerzo, una zona horaria inyectada y un plazo de gracia de 48
// horas salvo que la prueba indique otro. Así el cierre de un sprint, el plazo de gracia y el "hoy"
// se fijan sin esperar tiempo real. Los ayudantes llevan el prefijo "effort" para no chocar con los
// de validation_test.go y dto_test.go, que llevan los prefijos "validation" y "dto".

// effortPassword es la contraseña de las cuentas de prueba; cumple la política de la 001.
const effortPassword = "Effort2026"

// effortTimezone es la zona horaria inyectada como zona del sistema. Está detrás de UTC (UTC−3, sin
// horario de verano), así que a las 23:30 locales en UTC ya es el día siguiente.
const effortTimezone = "America/Argentina/Buenos_Aires"

// effortDefaultGrace es el plazo de gracia por defecto de EFFORT_GRACE_PERIOD_HOURS (FR-016).
const effortDefaultGrace = 48 * time.Hour

// Fechas de calendario de los proyectos de prueba y "hoy" según el reloj inicial.
const (
	effortProjectStart = "2026-10-01"
	effortProjectEnd   = "2026-12-20"
	effortToday        = "2026-10-21"
)

// effortCriterion es el criterio de aceptación de las historias de prueba, para que queden listas
// para planificar una vez estimadas.
const effortCriterion = "El alta valida el CUIT y guarda el cliente."

// effortClock es el reloj falso que comparten los servicios: devuelve siempre el mismo instante hasta
// que la prueba lo fija o lo avanza. Es seguro para uso concurrente porque lo leen los manejadores.
type effortClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now devuelve el instante vigente del reloj falso.
func (c *effortClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set fija el reloj falso en el instante indicado.
func (c *effortClock) Set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// Advance adelanta el reloj falso la duración indicada.
func (c *effortClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// effortTestEnv reúne el router de prueba, la base de test, el reloj falso, la zona inyectada y el
// plazo de gracia con que se compuso el servicio de esfuerzo.
type effortTestEnv struct {
	router   *gin.Engine
	db       *gorm.DB
	clock    *effortClock
	location *time.Location
	grace    time.Duration
}

// effortEnvOption modifica la composición del entorno antes de armar el router.
type effortEnvOption func(*effortTestEnv)

// withEffortGracePeriod compone el servicio de esfuerzo con otro plazo de gracia.
func withEffortGracePeriod(grace time.Duration) effortEnvOption {
	return func(e *effortTestEnv) {
		e.grace = grace
	}
}

// newEffortTestEnv abre la base de test vacía, carga la zona inyectada, fija el reloj en el miércoles
// 2026-10-21 a las 12:00 de esa zona y compone el router con newEffortRouter.
func newEffortTestEnv(t *testing.T, opts ...effortEnvOption) *effortTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	location, err := time.LoadLocation(effortTimezone)
	require.NoError(t, err, "no se pudo cargar la zona %s", effortTimezone)
	env := &effortTestEnv{
		db:       db,
		clock:    &effortClock{now: time.Date(2026, time.October, 21, 12, 0, 0, 0, location)},
		location: location,
		grace:    effortDefaultGrace,
	}
	for _, opt := range opts {
		opt(env)
	}
	env.router = newEffortRouter(db, cfg, env.clock, env.location, env.grace)
	return env
}

// newEffortRouter compone, igual que cmd/app, el límite del cuerpo, el rechazo de queries mal
// formadas, la cadena de autenticación de la 001, el módulo de proyectos de la 002 con su subgrupo
// /:projectId, el backlog de la 003 con su grupo de historia, los sprints de la 004 y el registro de
// esfuerzo, sin CORS ni Swagger. Los puertos de esfuerzo de la 003 y la 004 se componen como en
// cmd/app. El cambio de estado del proyecto, los sprints y el esfuerzo reciben el reloj falso y la
// zona inyectada; el esfuerzo, además, el plazo de gracia.
func newEffortRouter(db *gorm.DB, cfg config.Config, clock *effortClock, location *time.Location, grace time.Duration) *gin.Engine {
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

	// Módulo de proyectos de la 002, que devuelve el subgrupo /api/v1/projects/:projectId.
	projectRepository := projectsrepository.NewProjectRepository(db)
	projectMembers := projectsrepository.NewProjectMemberRepository(db)
	projects := projectsservice.NewProjectService(
		projectRepository,
		projectMembers,
		projectsrepository.NewBacklogSummaryProvider(db),
		projectsrepository.NewSprintDirectory(db),
	)
	members := projectsservice.NewMemberService(projectRepository, projectMembers, projectsrepository.NewUserDirectory(db))
	status := projectsservice.NewStatusService(projectRepository, projectsrepository.NewSprintDirectory(db), location, clock.Now)
	projectGroup, _ := projectsdelivery.RegisterRoutes(router, authHandler.RequireSession,
		projectsdelivery.NewHandler(projects, members, status), projectMembers, projectRepository)

	// Módulo del backlog de la 003, con el grupo plano de historia protegido por RequireStoryAccess.
	storyRepository := backlogrepository.NewStoryRepository(db)
	stories := backlogservice.NewStoryService(
		storyRepository,
		backlogrepository.NewSprintAssignments(db),
		backlogrepository.NewEffortRecords(db),
		backlogrepository.NewDefectRecords(),
		backlogrepository.NewEstimationSessions(db),
		backlogrepository.NewProjectDirectory(db),
	)
	backlogHandler := backlogdelivery.NewHandler(stories)
	backlogdelivery.RegisterRoutes(projectGroup, backlogHandler)
	backlogdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, storyRepository, backlogHandler)

	// Módulo de sprints de la 004 con el reloj falso, para iniciar, cerrar y avanzar historias en
	// instantes fijos.
	sprintRepository := sprintsrepository.NewSprintRepository(db)
	sprints := sprintsservice.NewSprintService(sprintRepository, sprintsrepository.NewSprintEffort(db), location, clock.Now)
	sprintsHandler := sprintsdelivery.NewHandler(sprints)
	sprintsdelivery.RegisterRoutes(projectGroup, sprintsHandler)
	sprintsdelivery.RegisterSprintRoutes(router, authHandler.RequireSession, sprintRepository, sprintsHandler)
	sprintsdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), sprintsHandler)

	// Módulo de esfuerzo con el reloj falso, la zona inyectada y el plazo de gracia: el alta cuelga
	// de la historia con el RequireStoryAccess de la 003, sobre la misma instancia del repositorio de
	// historias. La modificación y la eliminación cuelgan del registro con RequireEffortEntryAccess,
	// sobre la misma instancia del repositorio de esfuerzo que usa el servicio (US3, T044). La
	// consulta del esfuerzo del proyecto cuelga del subgrupo /:projectId de la 002 (US4, T056).
	effortRepository := repository.NewEffortRepository(db)
	efforts := service.NewEffortService(effortRepository, location, grace, clock.Now)
	effortHandler := NewHandler(efforts)
	RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), effortHandler)
	RegisterEntryRoutes(router, authHandler.RequireSession, effortRepository, effortHandler)
	RegisterProjectRoutes(projectGroup, effortHandler)
	return router
}

// at devuelve el instante de la fecha y la hora indicadas en la zona inyectada.
func (e *effortTestEnv) at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, e.location)
}

// do envía una petición al router con el cuerpo JSON opcional y el token como Bearer (si no es
// vacío), y devuelve la respuesta grabada.
func (e *effortTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// effortRequireStatus exige el código indicado y, si hay destino, decodifica el cuerpo JSON en él.
func effortRequireStatus(t *testing.T, rec *httptest.ResponseRecorder, code int, target any) {
	t.Helper()
	require.Equal(t, code, rec.Code, "cuerpo: %s", rec.Body.String())
	if target != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target), "el cuerpo no es JSON válido: %s", rec.Body.String())
	}
}

// effortParseID convierte el identificador de una respuesta en UUID o detiene la prueba.
func effortParseID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	require.NoError(t, err, "id = %q no es un UUID", raw)
	return id
}

// signUp registra una cuenta con POST /api/v1/auth/register de la 001, inicia su sesión con POST
// /api/v1/auth/login y devuelve el identificador de la cuenta y el token.
func (e *effortTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	var profile struct {
		ID string `json:"id"`
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/register", "",
		map[string]any{"fullName": fullName, "email": email, "password": effortPassword}), http.StatusCreated, &profile)
	var login struct {
		Token string `json:"token"`
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": effortPassword}), http.StatusOK, &login)
	return effortParseID(t, profile.ID), login.Token
}

// createProject crea con POST /api/v1/projects de la 002 un proyecto Planificado del titular del
// token, que empieza el 2026-10-01, y devuelve su identificador.
func (e *effortTestEnv) createProject(t *testing.T, ownerToken, name string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects", ownerToken, map[string]any{
		"name":               name,
		"startDate":          effortProjectStart,
		"plannedEndDate":     effortProjectEnd,
		"hoursPerStoryPoint": 6.5,
	}), http.StatusCreated, &created)
	return effortParseID(t, created.ID)
}

// setProjectStatus cambia el estado del proyecto con PATCH /api/v1/projects/{id}/status de la 002.
func (e *effortTestEnv) setProjectStatus(t *testing.T, ownerToken string, projectID uuid.UUID, status string) {
	t.Helper()
	effortRequireStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": status}), http.StatusOK, nil)
}

// createProjectInProgress crea el proyecto con la 002 y lo pasa a En curso.
func (e *effortTestEnv) createProjectInProgress(t *testing.T, ownerToken, name string) uuid.UUID {
	t.Helper()
	projectID := e.createProject(t, ownerToken, name)
	e.setProjectStatus(t, ownerToken, projectID, "in_progress")
	return projectID
}

// addMember suma al proyecto, con POST /api/v1/projects/{id}/members de la 002, la cuenta del
// correo indicado.
func (e *effortTestEnv) addMember(t *testing.T, ownerToken string, projectID uuid.UUID, email string) {
	t.Helper()
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/members", ownerToken,
		map[string]any{"email": email}), http.StatusCreated, nil)
}

// createStory crea con POST /api/v1/projects/{id}/stories de la 003 una historia "sin estimar" con
// un criterio de aceptación y devuelve su identificador.
func (e *effortTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, map[string]any{
		"title":              title,
		"priority":           "high",
		"storyPoints":        nil,
		"acceptanceCriteria": []string{effortCriterion},
	}), http.StatusCreated, &created)
	return effortParseID(t, created.ID)
}

// estimateStory registra los Story Points de la historia con PUT /api/v1/stories/{id}/story-points
// de la 003.
func (e *effortTestEnv) estimateStory(t *testing.T, token string, storyID uuid.UUID, points int) {
	t.Helper()
	effortRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", token,
		map[string]any{"storyPoints": points}), http.StatusOK, nil)
}

// createReadyStory crea con la 003 una historia y la estima en 5 Story Points, con lo que queda
// lista para planificar.
func (e *effortTestEnv) createReadyStory(t *testing.T, token string, projectID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	storyID := e.createStory(t, token, projectID, title)
	e.estimateStory(t, token, storyID, 5)
	return storyID
}

// createSprint crea con POST /api/v1/projects/{id}/sprints de la 004 un sprint Planificado con el
// período indicado y devuelve su identificador.
func (e *effortTestEnv) createSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/sprints", token, map[string]any{
		"name":      name,
		"goal":      "Clientes dados de alta desde el portal.",
		"startDate": start,
		"endDate":   end,
	}), http.StatusCreated, &created)
	return effortParseID(t, created.ID)
}

// assignStories compromete las historias en el sprint con POST /api/v1/sprints/{id}/stories de la
// 004.
func (e *effortTestEnv) assignStories(t *testing.T, token string, sprintID uuid.UUID, storyIDs ...uuid.UUID) {
	t.Helper()
	ids := make([]string, 0, len(storyIDs))
	for _, id := range storyIDs {
		ids = append(ids, id.String())
	}
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", token,
		map[string]any{"storyIds": ids}), http.StatusOK, nil)
}

// startSprint inicia el sprint con POST /api/v1/sprints/{id}/start de la 004, en el instante del
// reloj falso.
func (e *effortTestEnv) startSprint(t *testing.T, token string, sprintID uuid.UUID) {
	t.Helper()
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/start", token, nil),
		http.StatusOK, nil)
}

// closeSprintAt fija el reloj falso en el instante indicado y cierra el sprint con POST
// /api/v1/sprints/{id}/close de la 004, de modo que ese instante queda como su cierre real.
func (e *effortTestEnv) closeSprintAt(t *testing.T, token string, sprintID uuid.UUID, at time.Time) {
	t.Helper()
	e.clock.Set(at)
	effortRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/close", token, nil),
		http.StatusOK, nil)
}

// changeStoryStatus avanza la historia del sprint activo con PUT /api/v1/stories/{id}/status de la
// 004.
func (e *effortTestEnv) changeStoryStatus(t *testing.T, token string, storyID uuid.UUID, status string) {
	t.Helper()
	effortRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", token,
		map[string]any{"status": status}), http.StatusOK, nil)
}

// completeStory lleva la historia del sprint activo de Pendiente a En progreso y a Completada con la
// 004.
func (e *effortTestEnv) completeStory(t *testing.T, token string, storyID uuid.UUID) {
	t.Helper()
	e.changeStoryStatus(t, token, storyID, "in_progress")
	e.changeStoryStatus(t, token, storyID, "completed")
}

// activeSprintWith crea un sprint del 2026-10-12 al 2026-10-23, compromete en él las historias y lo
// inicia con la 004, y devuelve su identificador.
func (e *effortTestEnv) activeSprintWith(t *testing.T, token string, projectID uuid.UUID, storyIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	sprintID := e.createSprint(t, token, projectID, "Sprint 3", "2026-10-12", "2026-10-23")
	e.assignStories(t, token, sprintID, storyIDs...)
	e.startSprint(t, token, sprintID)
	return sprintID
}

// effortEntriesPath es la ruta del alta de esfuerzo sobre una historia según contracts/openapi.yaml.
func effortEntriesPath(storyID uuid.UUID) string {
	return "/api/v1/stories/" + storyID.String() + "/effort-entries"
}

// effortBody arma el cuerpo del alta con fecha, actividad y horas; cada prueba quita o modifica los
// campos que le interesan.
func effortBody(entryDate, activity string, hours any) map[string]any {
	return map[string]any{"entryDate": entryDate, "activity": activity, "hours": hours}
}

// postEffort envía POST /api/v1/stories/{storyId}/effort-entries con el cuerpo recibido.
func (e *effortTestEnv) postEffort(t *testing.T, token string, storyID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, effortEntriesPath(storyID), token, body)
}

// effortDate convierte una fecha AAAA-MM-DD a medianoche UTC, como se guarda entry_date.
func effortDate(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, value)
	require.NoError(t, err, "fecha de prueba inválida %q", value)
	return d
}

// seedEffortEntry inserta directamente en la base un registro de esfuerzo de la persona sobre la
// historia, para preparar el "Dado" de las horas ya cargadas sin depender del endpoint bajo prueba.
func (e *effortTestEnv) seedEffortEntry(t *testing.T, storyID, userID uuid.UUID, entryDate string, hundredths int, sprintID *uuid.UUID) {
	t.Helper()
	entry := domain.EffortEntry{
		ID:              uuid.New(),
		StoryID:         storyID,
		UserID:          userID,
		SprintID:        sprintID,
		EntryDate:       effortDate(t, entryDate),
		Activity:        "Horas cargadas previamente",
		HoursHundredths: hundredths,
	}
	require.NoError(t, e.db.Create(&entry).Error, "no se pudo sembrar el registro de esfuerzo")
}

// clearStoryPoints devuelve directamente en la base la historia a "sin estimar", para preparar los
// "Dado" de una historia sin estimar En progreso o Completada, que la 003 y la 004 no permiten
// alcanzar por su API (la 004 solo compromete historias estimadas y la 003 no quita la estimación de
// una historia comprometida ni completada).
func (e *effortTestEnv) clearStoryPoints(t *testing.T, storyID uuid.UUID) {
	t.Helper()
	require.NoError(t, e.db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).Update("story_points", nil).Error,
		"no se pudo quitar la estimación de la historia")
}

// effortEntryCount devuelve la cantidad de filas de effort_entries.
func (e *effortTestEnv) effortEntryCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, e.db.Model(&domain.EffortEntry{}).Count(&count).Error, "no se pudieron contar los registros")
	return count
}

// storyHundredths devuelve la suma de centésimas de hora registradas sobre la historia, leída en la
// base.
func (e *effortTestEnv) storyHundredths(t *testing.T, storyID uuid.UUID) int64 {
	t.Helper()
	var total int64
	require.NoError(t, e.db.Model(&domain.EffortEntry{}).
		Select("COALESCE(SUM(hours_hundredths), 0)").
		Where("story_id = ?", storyID).
		Scan(&total).Error, "no se pudo sumar el esfuerzo de la historia")
	return total
}

// userDateHundredths devuelve la suma de centésimas de hora de la persona en la fecha, en todos sus
// proyectos, leída en la base.
func (e *effortTestEnv) userDateHundredths(t *testing.T, userID uuid.UUID, entryDate string) int64 {
	t.Helper()
	var total int64
	require.NoError(t, e.db.Model(&domain.EffortEntry{}).
		Select("COALESCE(SUM(hours_hundredths), 0)").
		Where("user_id = ? AND entry_date = ?", userID, entryDate).
		Scan(&total).Error, "no se pudo sumar el esfuerzo de la persona en la fecha")
	return total
}

// storedEntry lee de la base el registro de esfuerzo con el identificador indicado.
func (e *effortTestEnv) storedEntry(t *testing.T, id string) domain.EffortEntry {
	t.Helper()
	var entry domain.EffortEntry
	require.NoError(t, e.db.Where("id = ?", id).Take(&entry).Error, "no se encontró el registro %s en la base", id)
	return entry
}

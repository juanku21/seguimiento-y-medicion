package delivery

import (
	"bytes"
	"encoding/json"
	"math"
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
	backlogrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	backlogservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
	defectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/delivery"
	defectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/repository"
	defectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/service"
	effortdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/delivery"
	effortrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/repository"
	effortservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/service"
	metricsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/repository"
	metricsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/service"
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

// Entorno de las pruebas de integración del cálculo de métricas (T010). Compone el router como
// cmd/app —sesión de la 001, proyectos de la 002, backlog de la 003, sprints de la 004, esfuerzo de la
// 006, defectos de la 007 y métricas— sobre la base smye_test, con un único reloj falso que comparten los servicios de
// cambio de estado del proyecto, de sprints, de esfuerzo y de métricas, y un plazo de gracia de 48
// horas. Así el cierre de un sprint y el plazo de gracia se fijan sin esperar tiempo real. Los
// ayudantes llevan el prefijo "metrics" para no chocar con los de otros archivos de prueba.

// metricsPassword es la contraseña de las cuentas de prueba; cumple la política de la 001.
const metricsPassword = "Metrics2026"

// metricsTimezone es la zona horaria inyectada como zona del sistema (UTC−3, sin horario de verano).
const metricsTimezone = "America/Argentina/Buenos_Aires"

// metricsGrace es el plazo de gracia de las pruebas: 48 horas, el valor por defecto de la 006.
const metricsGrace = 48 * time.Hour

// Fechas de calendario de los proyectos de prueba.
const (
	metricsProjectStart = "2026-10-01"
	metricsProjectEnd   = "2026-12-20"
)

// metricsCriterion es el criterio de aceptación de las historias de prueba, para que queden listas
// para planificar una vez estimadas.
const metricsCriterion = "El alta valida el CUIT y guarda el cliente."

// metricsClock es el reloj falso que comparten los servicios: devuelve siempre el mismo instante hasta
// que la prueba lo fija o lo avanza. Es seguro para uso concurrente porque lo leen los manejadores.
type metricsClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now devuelve el instante vigente del reloj falso.
func (c *metricsClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set fija el reloj falso en el instante indicado.
func (c *metricsClock) Set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// Advance adelanta el reloj falso la duración indicada.
func (c *metricsClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// metricsTestEnv reúne el router de prueba, la base de test, el reloj falso y la zona inyectada.
// effortDay cuenta los días de calendario ya usados por logHours, para no superar el tope de 24 horas
// diarias por persona de la 006.
type metricsTestEnv struct {
	router    *gin.Engine
	db        *gorm.DB
	clock     *metricsClock
	location  *time.Location
	effortDay int
}

// newMetricsTestEnv abre la base de test vacía, carga la zona inyectada, fija el reloj en el miércoles
// 2026-10-21 a las 12:00 de esa zona y compone el router con newMetricsRouter.
func newMetricsTestEnv(t *testing.T) *metricsTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	location, err := time.LoadLocation(metricsTimezone)
	require.NoError(t, err, "no se pudo cargar la zona %s", metricsTimezone)
	env := &metricsTestEnv{
		db:       db,
		clock:    &metricsClock{now: time.Date(2026, time.October, 21, 12, 0, 0, 0, location)},
		location: location,
	}
	env.router = newMetricsRouter(db, cfg, env.clock, env.location)
	return env
}

// newMetricsRouter compone, igual que cmd/app, el límite del cuerpo, el rechazo de queries mal
// formadas, la cadena de autenticación de la 001, el módulo de proyectos de la 002 con su subgrupo
// /:projectId, el backlog de la 003, los sprints de la 004, el esfuerzo de la 006 y las métricas, sin
// CORS ni Swagger. El cambio de estado del proyecto, los sprints, el esfuerzo y las métricas reciben
// el reloj falso; el esfuerzo y las métricas, además, el plazo de gracia de 48 horas.
func newMetricsRouter(db *gorm.DB, cfg config.Config, clock *metricsClock, location *time.Location) *gin.Engine {
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
		backlogrepository.NewDefectRecords(db),
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

	// Módulo de esfuerzo de la 006 con el reloj falso, la zona inyectada y el plazo de gracia.
	effortRepository := effortrepository.NewEffortRepository(db)
	efforts := effortservice.NewEffortService(effortRepository, location, metricsGrace, clock.Now)
	effortHandler := effortdelivery.NewHandler(efforts)
	effortdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), effortHandler)
	effortdelivery.RegisterEntryRoutes(router, authHandler.RequireSession, effortRepository, effortHandler)
	effortdelivery.RegisterProjectRoutes(projectGroup, effortHandler)

	// Módulo de defectos de la 007 con el reloj falso, para registrar, resolver, reabrir y descartar
	// defectos con sus endpoints reales.
	defectRepository := defectsrepository.NewDefectRepository(db)
	defectHandler := defectsdelivery.NewHandler(defectsservice.NewDefectService(defectRepository, clock.Now))
	defectsdelivery.RegisterProjectRoutes(projectGroup, defectHandler)
	defectsdelivery.RegisterDefectRoutes(router, authHandler.RequireSession, defectRepository, defectHandler)

	// Módulo de métricas con el reloj falso y el plazo de gracia, sobre el subgrupo /:projectId.
	metrics := metricsservice.NewMetricsService(metricsrepository.NewMetricsRepository(db), metricsGrace, clock.Now)
	RegisterRoutes(projectGroup, NewHandler(metrics))
	return router
}

// at devuelve el instante de la fecha y la hora indicadas en la zona inyectada.
func (e *metricsTestEnv) at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, e.location)
}

// do envía una petición al router con el cuerpo JSON opcional y el token como Bearer (si no es
// vacío), y devuelve la respuesta grabada.
func (e *metricsTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
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

// metricsRequireStatus exige el código indicado y, si hay destino, decodifica el cuerpo JSON en él.
func metricsRequireStatus(t *testing.T, rec *httptest.ResponseRecorder, code int, target any) {
	t.Helper()
	require.Equal(t, code, rec.Code, "cuerpo: %s", rec.Body.String())
	if target != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target), "el cuerpo no es JSON válido: %s", rec.Body.String())
	}
}

// metricsParseID convierte el identificador de una respuesta en UUID o detiene la prueba.
func metricsParseID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	require.NoError(t, err, "id = %q no es un UUID", raw)
	return id
}

// signUp registra una cuenta con POST /api/v1/auth/register de la 001, inicia su sesión con POST
// /api/v1/auth/login y devuelve el identificador de la cuenta y el token.
func (e *metricsTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	var profile struct {
		ID string `json:"id"`
	}
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/register", "",
		map[string]any{"fullName": fullName, "email": email, "password": metricsPassword}), http.StatusCreated, &profile)
	var login struct {
		Token string `json:"token"`
	}
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": metricsPassword}), http.StatusOK, &login)
	return metricsParseID(t, profile.ID), login.Token
}

// projectBody arma el cuerpo de alta y de modificación del proyecto con el factor indicado.
func projectBody(name string, factor any) map[string]any {
	return map[string]any{
		"name":               name,
		"startDate":          metricsProjectStart,
		"plannedEndDate":     metricsProjectEnd,
		"hoursPerStoryPoint": factor,
	}
}

// createProject crea con POST /api/v1/projects de la 002 un proyecto del titular del token con el
// factor de horas por Story Point indicado, lo pasa a En curso y devuelve su identificador.
func (e *metricsTestEnv) createProject(t *testing.T, ownerToken, name string, factor any) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects", ownerToken, projectBody(name, factor)),
		http.StatusCreated, &created)
	projectID := metricsParseID(t, created.ID)
	metricsRequireStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": "in_progress"}), http.StatusOK, nil)
	return projectID
}

// changeFactor cambia el factor de horas por Story Point del proyecto con PUT /api/v1/projects/{id}
// de la 002, que lo hace el propietario.
func (e *metricsTestEnv) changeFactor(t *testing.T, ownerToken string, projectID uuid.UUID, name string, factor any) {
	t.Helper()
	metricsRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/projects/"+projectID.String(), ownerToken,
		projectBody(name, factor)), http.StatusOK, nil)
}

// addMember suma al proyecto, con POST /api/v1/projects/{id}/members de la 002, la cuenta del
// correo indicado.
func (e *metricsTestEnv) addMember(t *testing.T, ownerToken string, projectID uuid.UUID, email string) {
	t.Helper()
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/members", ownerToken,
		map[string]any{"email": email}), http.StatusCreated, nil)
}

// createStory crea con POST /api/v1/projects/{id}/stories de la 003 una historia con los Story Points
// indicados, lista para planificar, y devuelve su identificador.
func (e *metricsTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, title string, points int) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, map[string]any{
		"title":              title,
		"priority":           "high",
		"storyPoints":        nil,
		"acceptanceCriteria": []string{metricsCriterion},
	}), http.StatusCreated, &created)
	storyID := metricsParseID(t, created.ID)
	e.estimateStory(t, token, storyID, points)
	return storyID
}

// estimateStory registra los Story Points de la historia con PUT /api/v1/stories/{id}/story-points
// de la 003; sirve tanto para la estimación inicial como para reestimarla después.
func (e *metricsTestEnv) estimateStory(t *testing.T, token string, storyID uuid.UUID, points int) {
	t.Helper()
	metricsRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", token,
		map[string]any{"storyPoints": points}), http.StatusOK, nil)
}

// createSprint crea con POST /api/v1/projects/{id}/sprints de la 004 un sprint Planificado con el
// período indicado y compromete en él las historias recibidas, si las hay.
func (e *metricsTestEnv) createSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string, storyIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/sprints", token, map[string]any{
		"name":      name,
		"goal":      "Clientes dados de alta desde el portal.",
		"startDate": start,
		"endDate":   end,
	}), http.StatusCreated, &created)
	sprintID := metricsParseID(t, created.ID)
	if len(storyIDs) > 0 {
		ids := make([]string, 0, len(storyIDs))
		for _, id := range storyIDs {
			ids = append(ids, id.String())
		}
		metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", token,
			map[string]any{"storyIds": ids}), http.StatusOK, nil)
	}
	return sprintID
}

// startSprint inicia el sprint con POST /api/v1/sprints/{id}/start de la 004, en el instante del
// reloj falso.
func (e *metricsTestEnv) startSprint(t *testing.T, token string, sprintID uuid.UUID) {
	t.Helper()
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/start", token, nil),
		http.StatusOK, nil)
}

// completeStory lleva la historia del sprint activo de Pendiente a En progreso y a Completada con
// PUT /api/v1/stories/{id}/status de la 004.
func (e *metricsTestEnv) completeStory(t *testing.T, token string, storyID uuid.UUID) {
	t.Helper()
	for _, status := range []string{"in_progress", "completed"} {
		metricsRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", token,
			map[string]any{"status": status}), http.StatusOK, nil)
	}
}

// closeSprintAt fija el reloj falso en el instante indicado y cierra el sprint con POST
// /api/v1/sprints/{id}/close de la 004, de modo que ese instante queda como su cierre real.
func (e *metricsTestEnv) closeSprintAt(t *testing.T, token string, sprintID uuid.UUID, at time.Time) {
	t.Helper()
	e.clock.Set(at)
	metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/close", token, nil),
		http.StatusOK, nil)
}

// logHours carga con POST /api/v1/stories/{id}/effort-entries de la 006 las horas indicadas sobre la
// historia. Las reparte en registros de hasta 24 horas, cada uno en un día de calendario distinto
// desde el inicio del proyecto, para respetar el tope diario; todas las fechas son anteriores al
// "hoy" inicial del reloj falso.
func (e *metricsTestEnv) logHours(t *testing.T, token string, storyID uuid.UUID, hours float64) {
	t.Helper()
	const maxEntryHundredths = 2400
	remaining := int(math.Round(hours * 100))
	start, err := time.Parse(time.DateOnly, metricsProjectStart)
	require.NoError(t, err, "fecha de inicio de prueba inválida")
	for remaining > 0 {
		chunk := min(remaining, maxEntryHundredths)
		date := start.AddDate(0, 0, e.effortDay).Format(time.DateOnly)
		e.effortDay++
		metricsRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/stories/"+storyID.String()+"/effort-entries", token, map[string]any{
			"entryDate": date,
			"activity":  "Horas cargadas para la prueba",
			"hours":     float64(chunk) / 100,
		}), http.StatusCreated, nil)
		remaining -= chunk
	}
}

// forceRow ejecuta directamente en smye_test la sentencia SQL indicada como dato de preparación: crea
// los datos rotos que ninguna API permite (una historia sin Story Points dentro de un sprint, un
// sprint Cerrado sin factor congelado o sin closed_at). Solo la usan las pruebas de SC-020.
func (e *metricsTestEnv) forceRow(t *testing.T, sql string, args ...any) {
	t.Helper()
	require.NoError(t, e.db.Exec(sql, args...).Error, "no se pudo forzar la fila de preparación")
}

// getSprintMetrics envía GET /api/v1/projects/{projectId}/metrics/sprints/{sprintId} con el token.
func (e *metricsTestEnv) getSprintMetrics(t *testing.T, token, projectID, sprintID string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, "/api/v1/projects/"+projectID+"/metrics/sprints/"+sprintID, token, nil)
}

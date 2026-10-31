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
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/delivery"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	projectsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/service"
	reportsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
	reportsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/repository"
	reportsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/service"
	sprintsdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/delivery"
	sprintsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
	sprintsservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Entorno de las pruebas de integración de la generación de reportes (T006). Compone el router como
// cmd/app —sesión de la 001, proyectos de la 002, backlog de la 003, sprints de la 004, esfuerzo de la
// 006, defectos de la 007, métricas de la 008 y reportes— sobre la base smye_test, con un único reloj
// falso que comparten todos los servicios que lo reciben y un plazo de gracia de 48 horas. Así el
// cierre de un sprint y el instante de generación del reporte se fijan sin esperar tiempo real. No se
// puede importar el entorno de métricas por ser un archivo _test.go, por eso se replica su estructura.

// reportsPassword es la contraseña de las cuentas de prueba; cumple la política de la 001.
const reportsPassword = "Reports2026"

// reportsTimezone es la zona horaria inyectada como zona del sistema (UTC−3, sin horario de verano).
const reportsTimezone = "America/Argentina/Buenos_Aires"

// reportsGrace es el plazo de gracia de las pruebas: 48 horas, el valor por defecto de la 006.
const reportsGrace = 48 * time.Hour

// Fechas de calendario de los proyectos de prueba.
const (
	reportsProjectStart = "2026-10-01"
	reportsProjectEnd   = "2026-12-20"
)

// reportsCriterion es el criterio de aceptación de las historias de prueba, para que queden listas
// para planificar una vez estimadas.
const reportsCriterion = "El alta valida el CUIT y guarda el cliente."

// reportsClock es el reloj falso que comparten los servicios: devuelve siempre el mismo instante hasta
// que la prueba lo fija o lo avanza. Es seguro para uso concurrente porque lo leen los manejadores.
type reportsClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now devuelve el instante vigente del reloj falso.
func (c *reportsClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set fija el reloj falso en el instante indicado.
func (c *reportsClock) Set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// Advance adelanta el reloj falso la duración indicada.
func (c *reportsClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// reportsCompose permite a una prueba reemplazar, o envolver, el repositorio de lectura o la fuente
// de métricas reales antes de que lleguen a ReportService (US4-6); recibe los reales y devuelve los
// que se usarán. Una función nula deja los reales.
type reportsCompose func(repo reportsdomain.ReportRepository, metrics reportsdomain.MetricsSource) (reportsdomain.ReportRepository, reportsdomain.MetricsSource)

// reportsTestEnv reúne el router de prueba, la base de test, el reloj falso y la zona inyectada.
// effortDay cuenta los días de calendario ya usados por logHours, para no superar el tope de 24 horas
// diarias por persona de la 006.
type reportsTestEnv struct {
	router    *gin.Engine
	db        *gorm.DB
	clock     *reportsClock
	location  *time.Location
	effortDay int
}

// newReportsTestEnv abre la base de test vacía y compone el router con los componentes reales.
func newReportsTestEnv(t *testing.T) *reportsTestEnv {
	t.Helper()
	return newReportsTestEnvWith(t, nil)
}

// newReportsTestEnvWith abre la base de test vacía, carga la zona inyectada, fija el reloj en el
// miércoles 2026-10-21 a las 12:00 de esa zona y compone el router; si compose no es nula, decide el
// repositorio de lectura y la fuente de métricas que recibe ReportService.
func newReportsTestEnvWith(t *testing.T, compose reportsCompose) *reportsTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	location, err := time.LoadLocation(reportsTimezone)
	require.NoError(t, err, "no se pudo cargar la zona %s", reportsTimezone)
	env := &reportsTestEnv{
		db:       db,
		clock:    &reportsClock{now: time.Date(2026, time.October, 21, 12, 0, 0, 0, location)},
		location: location,
	}
	env.router = newReportsRouter(db, cfg, env.clock, env.location, compose)
	return env
}

// newReportsRouter compone, igual que cmd/app, el límite del cuerpo, el rechazo de queries mal
// formadas, la cadena de autenticación, los módulos de proyectos, backlog, sprints, esfuerzo, defectos
// y métricas y, al final, el de reportes sobre el subgrupo /:projectId, sin CORS ni Swagger. Los
// servicios que dependen del tiempo reciben el reloj falso; el esfuerzo y las métricas, además, el
// plazo de gracia de 48 horas.
func newReportsRouter(db *gorm.DB, cfg config.Config, clock *reportsClock, location *time.Location, compose reportsCompose) *gin.Engine {
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
	efforts := effortservice.NewEffortService(effortRepository, location, reportsGrace, clock.Now)
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

	// Servicio de métricas de la 008 con el reloj falso y el plazo de gracia; es la misma instancia que
	// usa el reporte, como en cmd/app.
	metricsCalculator := metricsservice.NewMetricsService(metricsrepository.NewMetricsRepository(db), reportsGrace, clock.Now)

	// Módulo de reportes: el repositorio de lectura y la fuente de métricas pueden ser reemplazados
	// por la prueba; el registro de generaciones siempre es el real.
	var reportRepository reportsdomain.ReportRepository = reportsrepository.NewReportRepository(db)
	var metricsSource reportsdomain.MetricsSource = metricsCalculator
	if compose != nil {
		reportRepository, metricsSource = compose(reportRepository, metricsSource)
	}
	reports := reportsservice.NewReportService(
		reportRepository,
		reportsrepository.NewGenerationRepository(db),
		metricsSource,
		clock.Now,
	)
	RegisterRoutes(projectGroup, NewHandler(reports))
	return router
}

// at devuelve el instante de la fecha y la hora indicadas en la zona inyectada.
func (e *reportsTestEnv) at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, e.location)
}

// do envía una petición al router con el cuerpo JSON opcional y el token como Bearer (si no es
// vacío), y devuelve la respuesta grabada.
func (e *reportsTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
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

// requireStatus exige el código indicado y, si hay destino, decodifica el cuerpo JSON en él.
func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, code int, target any) {
	t.Helper()
	require.Equal(t, code, rec.Code, "cuerpo: %s", rec.Body.String())
	if target != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target), "el cuerpo no es JSON válido: %s", rec.Body.String())
	}
}

// parseID convierte el identificador de una respuesta en UUID o detiene la prueba.
func parseID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	require.NoError(t, err, "id = %q no es un UUID", raw)
	return id
}

// signUp registra una cuenta con POST /api/v1/auth/register de la 001, inicia su sesión con POST
// /api/v1/auth/login y devuelve el identificador de la cuenta y el token.
func (e *reportsTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	var profile struct {
		ID string `json:"id"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/register", "",
		map[string]any{"fullName": fullName, "email": email, "password": reportsPassword}), http.StatusCreated, &profile)
	var login struct {
		Token string `json:"token"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": reportsPassword}), http.StatusOK, &login)
	return parseID(t, profile.ID), login.Token
}

// createProject crea con POST /api/v1/projects de la 002 un proyecto del titular del token con el
// factor de horas por Story Point indicado, lo pasa a En curso y devuelve su identificador.
func (e *reportsTestEnv) createProject(t *testing.T, ownerToken, name string, factor any) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects", ownerToken, map[string]any{
		"name":               name,
		"startDate":          reportsProjectStart,
		"plannedEndDate":     reportsProjectEnd,
		"hoursPerStoryPoint": factor,
	}), http.StatusCreated, &created)
	projectID := parseID(t, created.ID)
	requireStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": "in_progress"}), http.StatusOK, nil)
	return projectID
}

// addMember suma al proyecto, con POST /api/v1/projects/{id}/members de la 002, la cuenta del
// correo indicado.
func (e *reportsTestEnv) addMember(t *testing.T, ownerToken string, projectID uuid.UUID, email string) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/members", ownerToken,
		map[string]any{"email": email}), http.StatusCreated, nil)
}

// removeMember quita al integrante del proyecto con DELETE /api/v1/projects/{id}/members/{userId}
// de la 002, que lo hace el propietario.
func (e *reportsTestEnv) removeMember(t *testing.T, ownerToken string, projectID, userID uuid.UUID) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/members/"+userID.String(), ownerToken, nil),
		http.StatusNoContent, nil)
}

// finishProject pasa el proyecto a Finalizado con PATCH /api/v1/projects/{id}/status de la 002.
func (e *reportsTestEnv) finishProject(t *testing.T, ownerToken string, projectID uuid.UUID) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": "finished"}), http.StatusOK, nil)
}

// createStory crea con POST /api/v1/projects/{id}/stories de la 003 una historia con la prioridad y
// los Story Points indicados, lista para planificar, y devuelve su identificador. Con points nulo
// (negativo) la historia queda sin estimar.
func (e *reportsTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, title, priority string, points int) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, map[string]any{
		"title":              title,
		"priority":           priority,
		"storyPoints":        nil,
		"acceptanceCriteria": []string{reportsCriterion},
	}), http.StatusCreated, &created)
	storyID := parseID(t, created.ID)
	if points >= 0 {
		e.estimateStory(t, token, storyID, points)
	}
	return storyID
}

// estimateStory registra los Story Points de la historia con PUT /api/v1/stories/{id}/story-points
// de la 003; sirve tanto para la estimación inicial como para reestimarla después.
func (e *reportsTestEnv) estimateStory(t *testing.T, token string, storyID uuid.UUID, points int) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", token,
		map[string]any{"storyPoints": points}), http.StatusOK, nil)
}

// renameStory cambia el título de la historia con PATCH /api/v1/stories/{id} de la 003.
func (e *reportsTestEnv) renameStory(t *testing.T, token string, storyID uuid.UUID, title string) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPatch, "/api/v1/stories/"+storyID.String(), token,
		map[string]any{"title": title}), http.StatusOK, nil)
}

// changePriority cambia la prioridad de la historia con PATCH /api/v1/stories/{id} de la 003.
func (e *reportsTestEnv) changePriority(t *testing.T, token string, storyID uuid.UUID, priority string) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPatch, "/api/v1/stories/"+storyID.String(), token,
		map[string]any{"priority": priority}), http.StatusOK, nil)
}

// createSprint crea con POST /api/v1/projects/{id}/sprints de la 004 un sprint Planificado con el
// período indicado y compromete en él las historias recibidas, si las hay.
func (e *reportsTestEnv) createSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string, storyIDs ...uuid.UUID) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/sprints", token, map[string]any{
		"name":      name,
		"goal":      "Clientes dados de alta desde el portal.",
		"startDate": start,
		"endDate":   end,
	}), http.StatusCreated, &created)
	sprintID := parseID(t, created.ID)
	if len(storyIDs) > 0 {
		ids := make([]string, 0, len(storyIDs))
		for _, id := range storyIDs {
			ids = append(ids, id.String())
		}
		requireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", token,
			map[string]any{"storyIds": ids}), http.StatusOK, nil)
	}
	return sprintID
}

// startSprint inicia el sprint con POST /api/v1/sprints/{id}/start de la 004, en el instante del
// reloj falso.
func (e *reportsTestEnv) startSprint(t *testing.T, token string, sprintID uuid.UUID) {
	t.Helper()
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/start", token, nil),
		http.StatusOK, nil)
}

// completeStory lleva la historia del sprint activo de Pendiente a En progreso y a Completada con
// PUT /api/v1/stories/{id}/status de la 004.
func (e *reportsTestEnv) completeStory(t *testing.T, token string, storyID uuid.UUID) {
	t.Helper()
	for _, status := range []string{"in_progress", "completed"} {
		requireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", token,
			map[string]any{"status": status}), http.StatusOK, nil)
	}
}

// closeSprintAt fija el reloj falso en el instante indicado y cierra el sprint con POST
// /api/v1/sprints/{id}/close de la 004, de modo que ese instante queda como su cierre real.
func (e *reportsTestEnv) closeSprintAt(t *testing.T, token string, sprintID uuid.UUID, at time.Time) {
	t.Helper()
	e.clock.Set(at)
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/close", token, nil),
		http.StatusOK, nil)
}

// logHours carga con POST /api/v1/stories/{id}/effort-entries de la 006 las horas indicadas sobre la
// historia. Las reparte en registros de hasta 24 horas, cada uno en un día de calendario distinto
// desde el inicio del proyecto, para respetar el tope diario; todas las fechas son anteriores al
// "hoy" inicial del reloj falso.
func (e *reportsTestEnv) logHours(t *testing.T, token string, storyID uuid.UUID, hours float64) {
	t.Helper()
	const maxEntryHundredths = 2400
	remaining := int(math.Round(hours * 100))
	start, err := time.Parse(time.DateOnly, reportsProjectStart)
	require.NoError(t, err, "fecha de inicio de prueba inválida")
	for remaining > 0 {
		chunk := min(remaining, maxEntryHundredths)
		date := start.AddDate(0, 0, e.effortDay).Format(time.DateOnly)
		e.effortDay++
		requireStatus(t, e.do(t, http.MethodPost, "/api/v1/stories/"+storyID.String()+"/effort-entries", token, map[string]any{
			"entryDate": date,
			"activity":  "Horas cargadas para la prueba",
			"hours":     float64(chunk) / 100,
		}), http.StatusCreated, nil)
		remaining -= chunk
	}
}

// createDefect registra con POST /api/v1/projects/{id}/defects de la 007 un defecto de la severidad
// indicada sobre la historia, detectado en el sprint dado, y devuelve su identificador.
func (e *reportsTestEnv) createDefect(t *testing.T, token string, projectID, storyID, detectedSprintID uuid.UUID, severity, description string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	requireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/defects", token, map[string]any{
		"description":      description,
		"severity":         severity,
		"storyId":          storyID.String(),
		"detectedSprintId": detectedSprintID.String(),
	}), http.StatusCreated, &created)
	return parseID(t, created.ID)
}

// changeDefectStatus cambia el estado del defecto con PUT /api/v1/defects/{id}/status de la 007. El
// sprint de resolución solo se envía si no es uuid.Nil (es obligatorio al resolver).
func (e *reportsTestEnv) changeDefectStatus(t *testing.T, token string, defectID uuid.UUID, status string, resolvedSprintID uuid.UUID) {
	t.Helper()
	body := map[string]any{"status": status}
	if resolvedSprintID != uuid.Nil {
		body["resolvedSprintId"] = resolvedSprintID.String()
	}
	requireStatus(t, e.do(t, http.MethodPut, "/api/v1/defects/"+defectID.String()+"/status", token, body),
		http.StatusOK, nil)
}

// forceRow ejecuta directamente en smye_test la sentencia SQL indicada como dato de preparación: crea
// los datos que ninguna API permite producir con el reloj de la prueba (sesiones de estimación con
// fechas arbitrarias, un sprint Cerrado sin factor congelado).
func (e *reportsTestEnv) forceRow(t *testing.T, sql string, args ...any) {
	t.Helper()
	require.NoError(t, e.db.Exec(sql, args...).Error, "no se pudo forzar la fila de preparación")
}

// seedPokerSession inserta con forceRow una sesión de Planning Poker de la historia en el estado
// indicado, con tantas rondas como se pidan (la última queda en votación si la sesión está abierta;
// las demás, reveladas), los Story Points acordados y las fechas de creación y de cierre. El
// facilitador y quien la abrió son el propietario del proyecto de la historia; en una sesión
// cancelada la causa es la inactividad. Devuelve el identificador de la sesión.
func (e *reportsTestEnv) seedPokerSession(t *testing.T, storyID uuid.UUID, status pokerdomain.SessionStatus, rounds int, agreed *int, createdAt time.Time, closedAt *time.Time) uuid.UUID {
	t.Helper()
	sessionID := uuid.New()
	var cancelReason *string
	if status == pokerdomain.SessionStatusCanceled {
		reason := string(pokerdomain.CancelReasonInactivity)
		cancelReason = &reason
	}
	lastActivity := createdAt
	if closedAt != nil {
		lastActivity = *closedAt
	}
	e.forceRow(t, `INSERT INTO estimation_sessions
		(id, story_id, facilitator_id, status, agreed_story_points, cancel_reason, opened_by_id, closed_by_id,
		 closed_at, last_activity_at, version, created_at, updated_at)
		SELECT ?, s.id, p.owner_id, ?, ?, ?, p.owner_id, CASE WHEN ?::timestamptz IS NULL THEN NULL ELSE p.owner_id END,
		 ?, ?, 0, ?, ?
		FROM user_stories s JOIN projects p ON p.id = s.project_id WHERE s.id = ?`,
		sessionID, string(status), agreed, cancelReason, closedAt, closedAt, lastActivity, createdAt, lastActivity, storyID)
	for number := 1; number <= rounds; number++ {
		roundStatus := pokerdomain.RoundStatusRevealed
		var revealedAt *time.Time
		if status == pokerdomain.SessionStatusOpen && number == rounds {
			roundStatus = pokerdomain.RoundStatusVoting
		} else {
			revealed := createdAt
			revealedAt = &revealed
		}
		e.forceRow(t, `INSERT INTO estimation_rounds (id, session_id, number, status, revealed_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New(), sessionID, number, string(roundStatus), revealedAt, createdAt, createdAt)
	}
	return sessionID
}

// generateSprintReport envía POST /api/v1/projects/{projectId}/reports/sprints/{sprintId} con el
// token y devuelve la respuesta grabada, sin exigir ningún código.
func (e *reportsTestEnv) generateSprintReport(t *testing.T, token string, projectID, sprintID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/reports/sprints/"+sprintID.String(), token, nil)
}

// generateProjectReport envía POST /api/v1/projects/{projectId}/reports/project con el token y
// devuelve la respuesta grabada, sin exigir ningún código.
func (e *reportsTestEnv) generateProjectReport(t *testing.T, token string, projectID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/reports/project", token, nil)
}

// decodeReport exige el 200 de la respuesta y decodifica el reporte como un mapa genérico, para
// comparar reportes y secciones sin atarse a los tipos del dominio.
func decodeReport(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var report map[string]any
	requireStatus(t, rec, http.StatusOK, &report)
	return report
}

// withoutGeneration borra del encabezado del reporte los datos de la generación (header.generatedAt
// y header.generatedBy), que cambian en cada entrega, y devuelve el mismo mapa; sirve para comparar
// dos reportes del mismo estado del proyecto.
func withoutGeneration(report map[string]any) map[string]any {
	if header, ok := report["header"].(map[string]any); ok {
		delete(header, "generatedAt")
		delete(header, "generatedBy")
	}
	return report
}

// generationRows lee de report_generations, en orden de generación, los registros del proyecto.
func (e *reportsTestEnv) generationRows(t *testing.T, projectID uuid.UUID) []reportsdomain.ReportGeneration {
	t.Helper()
	rows := []reportsdomain.ReportGeneration{}
	require.NoError(t, e.db.Where("project_id = ?", projectID).Order("generated_at ASC").Find(&rows).Error,
		"no se pudo leer report_generations")
	return rows
}

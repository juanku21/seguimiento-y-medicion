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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/service"
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

// Entorno de las pruebas de integración de los defectos (T011). Compone el router como cmd/app
// —sesión de la 001, proyectos de la 002, backlog de la 003, sprints de la 004 y defectos— sobre la
// base smye_test, con un único reloj falso que comparten el servicio de cambio de estado del
// proyecto, el de sprints y el de defectos. Los proyectos, las historias y los sprints se crean con
// los endpoints reales; solo forceDefectStatus escribe directamente en la base, como dato de
// preparación. Los ayudantes llevan el prefijo "defect" para no chocar con los de
// validation_test.go, que llevan el prefijo "validation".

// defectPassword es la contraseña de las cuentas de prueba; cumple la política de la 001.
const defectPassword = "Defects2026"

// defectTimezone es la zona horaria inyectada como zona del sistema.
const defectTimezone = "America/Argentina/Buenos_Aires"

// Fechas de calendario de los proyectos de prueba.
const (
	defectProjectStart = "2026-10-01"
	defectProjectEnd   = "2026-12-20"
)

// defectCriterion es el criterio de aceptación de las historias de prueba, para que queden listas
// para planificar una vez estimadas.
const defectCriterion = "El reporte suma los descuentos."

// defectClock es el reloj falso que comparten los servicios; es seguro para uso concurrente porque
// lo leen los manejadores.
type defectClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now devuelve el instante vigente del reloj falso.
func (c *defectClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set fija el reloj falso en el instante indicado.
func (c *defectClock) Set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = at
}

// defectTestEnv reúne el router de prueba, la base de test y el reloj falso.
type defectTestEnv struct {
	router   *gin.Engine
	db       *gorm.DB
	clock    *defectClock
	location *time.Location
}

// newDefectTestEnv abre la base de test vacía, carga la zona inyectada, fija el reloj en el miércoles
// 2026-10-21 a las 12:00 de esa zona y compone el router con newDefectRouter.
func newDefectTestEnv(t *testing.T) *defectTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	location, err := time.LoadLocation(defectTimezone)
	require.NoError(t, err, "no se pudo cargar la zona %s", defectTimezone)
	env := &defectTestEnv{
		db:       db,
		clock:    &defectClock{now: time.Date(2026, time.October, 21, 12, 0, 0, 0, location)},
		location: location,
	}
	env.router = newDefectRouter(db, cfg, env.clock, location)
	return env
}

// newDefectRouter compone, igual que cmd/app, el límite del cuerpo, el rechazo de queries mal
// formadas, la cadena de autenticación de la 001, el módulo de proyectos de la 002 con su subgrupo
// /:projectId, el backlog de la 003, los sprints de la 004 y los defectos, sin CORS ni Swagger. El
// cambio de estado del proyecto, los sprints y los defectos reciben el mismo reloj falso.
func newDefectRouter(db *gorm.DB, cfg config.Config, clock *defectClock, location *time.Location) *gin.Engine {
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

	// Módulo del backlog de la 003 con sus puertos, como en cmd/app.
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

	// Módulo de sprints de la 004 con el reloj falso.
	sprintRepository := sprintsrepository.NewSprintRepository(db)
	sprints := sprintsservice.NewSprintService(sprintRepository, sprintsrepository.NewSprintEffort(db), location, clock.Now)
	sprintsHandler := sprintsdelivery.NewHandler(sprints)
	sprintsdelivery.RegisterRoutes(projectGroup, sprintsHandler)
	sprintsdelivery.RegisterSprintRoutes(router, authHandler.RequireSession, sprintRepository, sprintsHandler)
	sprintsdelivery.RegisterStoryRoutes(router, authHandler.RequireSession, backlogdelivery.RequireStoryAccess(storyRepository), sprintsHandler)

	// Módulo de defectos con el mismo reloj falso, sobre el subgrupo /:projectId de la 002 y el grupo
	// plano /api/v1/defects/:defectId, con la misma instancia del repositorio que cmd/app.
	defectRepository := repository.NewDefectRepository(db)
	defects := service.NewDefectService(defectRepository, clock.Now)
	defectHandler := NewHandler(defects)
	RegisterProjectRoutes(projectGroup, defectHandler)
	RegisterDefectRoutes(router, authHandler.RequireSession, defectRepository, defectHandler)
	return router
}

// do envía una petición al router con el cuerpo JSON opcional y el token como Bearer (si no es
// vacío), y devuelve la respuesta grabada.
func (e *defectTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
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

// defectRequireStatus exige el código indicado y, si hay destino, decodifica el cuerpo JSON en él.
func defectRequireStatus(t *testing.T, rec *httptest.ResponseRecorder, code int, target any) {
	t.Helper()
	require.Equal(t, code, rec.Code, "cuerpo: %s", rec.Body.String())
	if target != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target), "el cuerpo no es JSON válido: %s", rec.Body.String())
	}
}

// defectParseID convierte el identificador de una respuesta en UUID o detiene la prueba.
func defectParseID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	require.NoError(t, err, "id = %q no es un UUID", raw)
	return id
}

// signUp registra una cuenta con la 001, inicia su sesión y devuelve el identificador y el token.
func (e *defectTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	var profile struct {
		ID string `json:"id"`
	}
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/register", "",
		map[string]any{"fullName": fullName, "email": email, "password": defectPassword}), http.StatusCreated, &profile)
	var login struct {
		Token string `json:"token"`
	}
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/auth/login", "",
		map[string]any{"email": email, "password": defectPassword}), http.StatusOK, &login)
	return defectParseID(t, profile.ID), login.Token
}

// createProjectInProgress crea con la 002 un proyecto del titular del token y lo pasa a En curso.
func (e *defectTestEnv) createProjectInProgress(t *testing.T, ownerToken, name string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects", ownerToken, map[string]any{
		"name":               name,
		"startDate":          defectProjectStart,
		"plannedEndDate":     defectProjectEnd,
		"hoursPerStoryPoint": 6.5,
	}), http.StatusCreated, &created)
	projectID := defectParseID(t, created.ID)
	e.setProjectStatus(t, ownerToken, projectID, "in_progress")
	return projectID
}

// setProjectStatus cambia el estado del proyecto con PATCH /api/v1/projects/{id}/status de la 002.
func (e *defectTestEnv) setProjectStatus(t *testing.T, ownerToken string, projectID uuid.UUID, status string) {
	t.Helper()
	defectRequireStatus(t, e.do(t, http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/status", ownerToken,
		map[string]any{"status": status}), http.StatusOK, nil)
}

// addMember suma al proyecto, con la 002, la cuenta del correo indicado.
func (e *defectTestEnv) addMember(t *testing.T, ownerToken string, projectID uuid.UUID, email string) {
	t.Helper()
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/members", ownerToken,
		map[string]any{"email": email}), http.StatusCreated, nil)
}

// createStory crea con la 003 una historia estimada en 5 Story Points, lista para planificar.
func (e *defectTestEnv) createStory(t *testing.T, token string, projectID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/stories", token, map[string]any{
		"title":              title,
		"priority":           "high",
		"storyPoints":        nil,
		"acceptanceCriteria": []string{defectCriterion},
	}), http.StatusCreated, &created)
	storyID := defectParseID(t, created.ID)
	defectRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", token,
		map[string]any{"storyPoints": 5}), http.StatusOK, nil)
	return storyID
}

// createSprint crea con la 004 un sprint Planificado con el período indicado.
func (e *defectTestEnv) createSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string) uuid.UUID {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/sprints", token, map[string]any{
		"name":      name,
		"goal":      "Reportes con los totales correctos.",
		"startDate": start,
		"endDate":   end,
	}), http.StatusCreated, &created)
	return defectParseID(t, created.ID)
}

// startedSprint crea un sprint con el período indicado, compromete en él una historia de apoyo y lo
// inicia con la 004 (un sprint solo se inicia con historias comprometidas).
func (e *defectTestEnv) startedSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string) uuid.UUID {
	t.Helper()
	sprintID := e.createSprint(t, token, projectID, name, start, end)
	supportID := e.createStory(t, token, projectID, "Historia de apoyo de "+name)
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", token,
		map[string]any{"storyIds": []string{supportID.String()}}), http.StatusOK, nil)
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/start", token, nil),
		http.StatusOK, nil)
	return sprintID
}

// activeSprint crea y deja Activo un sprint que abarca el "hoy" del reloj falso.
func (e *defectTestEnv) activeSprint(t *testing.T, token string, projectID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	return e.startedSprint(t, token, projectID, name, "2026-10-19", "2026-10-30")
}

// closedSprint crea, inicia y cierra un sprint con el período indicado.
func (e *defectTestEnv) closedSprint(t *testing.T, token string, projectID uuid.UUID, name, start, end string) uuid.UUID {
	t.Helper()
	sprintID := e.startedSprint(t, token, projectID, name, start, end)
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/close", token, nil),
		http.StatusOK, nil)
	return sprintID
}

// completeStory compromete la historia en el sprint Activo y la lleva de Pendiente a En progreso y a
// Completada con la 004.
func (e *defectTestEnv) completeStory(t *testing.T, token string, sprintID, storyID uuid.UUID) {
	t.Helper()
	defectRequireStatus(t, e.do(t, http.MethodPost, "/api/v1/sprints/"+sprintID.String()+"/stories", token,
		map[string]any{"storyIds": []string{storyID.String()}}), http.StatusOK, nil)
	for _, status := range []string{"in_progress", "completed"} {
		defectRequireStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", token,
			map[string]any{"status": status}), http.StatusOK, nil)
	}
}

// defectsPath es la ruta del alta de defectos según contracts/openapi.yaml.
func defectsPath(projectID uuid.UUID) string {
	return "/api/v1/projects/" + projectID.String() + "/defects"
}

// postDefect envía POST /api/v1/projects/{projectId}/defects con el cuerpo recibido.
func (e *defectTestEnv) postDefect(t *testing.T, token string, projectID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, defectsPath(projectID), token, body)
}

// forceDefectStatus escribe directamente en smye_test el estado y el sprint de resolución del defecto,
// como dato de preparación de los escenarios que dependen de estados que solo producen otras
// historias (Descartado, de US6; reabierto, de US5). Ninguna prueba lo usa para verificar una
// transición.
func (e *defectTestEnv) forceDefectStatus(t *testing.T, defectID uuid.UUID, status domain.DefectStatus, resolvedSprintID *uuid.UUID) {
	t.Helper()
	require.NoError(t, e.db.Model(&domain.Defect{}).Where("id = ?", defectID).
		Updates(map[string]any{"status": status, "resolved_sprint_id": resolvedSprintID}).Error,
		"no se pudo forzar el estado del defecto")
}

// defectCount devuelve la cantidad de filas de defects.
func (e *defectTestEnv) defectCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, e.db.Model(&domain.Defect{}).Count(&count).Error, "no se pudieron contar los defectos")
	return count
}

// storyDefectCount devuelve la cantidad de filas de defects de la historia.
func (e *defectTestEnv) storyDefectCount(t *testing.T, storyID uuid.UUID) int64 {
	t.Helper()
	var count int64
	require.NoError(t, e.db.Model(&domain.Defect{}).Where("story_id = ?", storyID).Count(&count).Error,
		"no se pudieron contar los defectos de la historia")
	return count
}

// everHadDefects lee la marca ever_had_defects de la historia en la base.
func (e *defectTestEnv) everHadDefects(t *testing.T, storyID uuid.UUID) bool {
	t.Helper()
	var story backlogdomain.UserStory
	require.NoError(t, e.db.Where("id = ?", storyID).Take(&story).Error, "no se encontró la historia %s", storyID)
	return story.EverHadDefects
}

// storedDefect lee de la base el defecto con el identificador indicado.
func (e *defectTestEnv) storedDefect(t *testing.T, id string) domain.Defect {
	t.Helper()
	var defect domain.Defect
	require.NoError(t, e.db.Where("id = ?", id).Take(&defect).Error, "no se encontró el defecto %s en la base", id)
	return defect
}

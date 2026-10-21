package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	authrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/repository"
	authservice "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
)

// Pruebas de integración del middleware de acceso a una historia de US3 (T039). Montan
// RequireStoryAccess con httptest sobre un manejador protegido de prueba, detrás del middleware de
// sesión real de la 001 y con el repositorio real del backlog contra smye_test, y verifican que un
// integrante que no es el propietario pasa con su StoryAccess en el contexto (RN-01, RN-02), que la
// historia inexistente, la de un proyecto ajeno, la de un integrante ya quitado y un identificador
// que no es UUID responden el mismo 404 byte a byte con a lo sumo una consulta a la base, y que sin
// titular en el contexto se responde el 401 de la 001 (research.md sección 3; FR-045 a FR-047).

// storyNotFoundBody es el cuerpo exacto, byte a byte, del 404 de historia inexistente del contrato.
const storyNotFoundBody = `{"error":"La historia no existe."}`

// storyAccessSessionBody es el cuerpo exacto del 401 de la 001 cuando no hay titular de sesión.
const storyAccessSessionBody = `{"error":"Se requiere iniciar sesión."}`

// storyAccessProbeRoute es la ruta del manejador protegido de prueba; no existe en producción.
const storyAccessProbeRoute = "/api/v1/solo-prueba-historia/:storyId"

// storyAccessProbe registra cuántas veces se ejecutó el manejador protegido y qué StoryAccess
// encontró en el contexto en su última ejecución.
type storyAccessProbe struct {
	executions int
	access     *domain.StoryAccess
	hasAccess  bool
}

// handle es el manejador protegido de prueba: cuenta su ejecución, guarda el StoryAccess que dejó
// el middleware en el contexto y responde 200.
func (p *storyAccessProbe) handle(c *gin.Context) {
	p.executions++
	p.access, p.hasAccess = storyAccessFromContext(c)
	c.Status(http.StatusOK)
}

// queryCounter es un logger de GORM que no escribe nada y cuenta cada sentencia SQL ejecutada:
// GORM llama a Trace una vez por sentencia, cualquiera sea el nivel de log, así que el contador
// mide cuántas consultas hizo el middleware durante una petición.
type queryCounter struct {
	logger.Interface
	count atomic.Int64
}

// newQueryCounter crea el contador sobre el logger mudo de GORM.
func newQueryCounter() *queryCounter {
	return &queryCounter{Interface: logger.Discard}
}

// LogMode devuelve el mismo contador para que GORM no lo reemplace al cambiar de nivel.
func (q *queryCounter) LogMode(logger.LogLevel) logger.Interface {
	return q
}

// Trace suma una sentencia ejecutada.
func (q *queryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	q.count.Add(1)
}

// storyAccessTestEnv reúne el router con el manejador de prueba, la base de test, el servicio de
// autenticación para crear cuentas y sesiones, la sonda del manejador protegido y el contador de
// consultas del repositorio que usa el middleware.
type storyAccessTestEnv struct {
	router  *gin.Engine
	db      *gorm.DB
	auth    *authservice.AuthService
	probe   *storyAccessProbe
	queries *queryCounter
}

// newStoryAccessTestEnv abre la base de test vacía y monta, en este orden, el middleware de sesión
// real de la 001, RequireStoryAccess con el repositorio real del backlog y el manejador protegido de
// prueba. El repositorio recibe una sesión de GORM con el contador de consultas como logger, de modo
// que solo se cuentan las consultas del middleware y no las de la preparación del escenario.
func newStoryAccessTestEnv(t *testing.T) storyAccessTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	queries := newQueryCounter()
	countedDB := db.Session(&gorm.Session{Logger: queries})
	probe := &storyAccessProbe{}
	router := gin.New()
	router.GET(storyAccessProbeRoute,
		authdelivery.NewHandler(auth).RequireSession,
		RequireStoryAccess(repository.NewStoryRepository(countedDB)),
		probe.handle,
	)
	return storyAccessTestEnv{router: router, db: db, auth: auth, probe: probe, queries: queries}
}

// signUp registra una cuenta e inicia su sesión, devolviendo su identificador y su token.
func (e storyAccessTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	return backlogTestEnv{auth: e.auth}.signUp(t, fullName, email)
}

// call pide el manejador protegido para el identificador de historia indicado con el token como
// Bearer, con el contador de consultas en cero al empezar, y devuelve la respuesta y la cantidad de
// consultas que se hicieron durante la petición.
func (e storyAccessTestEnv) call(t *testing.T, token, storyID string) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	e.queries.count.Store(0)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-historia/"+storyID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, e.queries.count.Load()
}

// storyAccessSeedStory inserta directamente en la base una historia pendiente del proyecto, creada
// por el autor indicado, y devuelve su identificador, para preparar el "Dado" sin pasar por la API.
func storyAccessSeedStory(t *testing.T, db *gorm.DB, projectID, authorID uuid.UUID) uuid.UUID {
	t.Helper()
	story := domain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       backlogStoryTitle,
		Priority:    domain.StoryPriorityMedium,
		Status:      domain.StoryStatusPending,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	return story.ID
}

// Prueba transversal de RequireStoryAccess (T039): un integrante accede a una historia.
// Bruno, integrante que no es el propietario ni el autor, pide una historia de su proyecto: el
// middleware hace una sola consulta, el manejador protegido se ejecuta una vez y encuentra en el
// contexto el StoryAccess con la historia, su proyecto y si el proyecto está Finalizado (RN-01,
// RN-02). El proyecto Finalizado también se puede consultar y llega marcado (FR-048).
func TestStoryAccess_IntegranteNoPropietarioPasa(t *testing.T) {
	cases := []struct {
		name     string
		status   projectsdomain.ProjectStatus
		finished bool
	}{
		{name: "proyecto en curso", status: projectsdomain.ProjectStatusInProgress, finished: false},
		{name: "proyecto finalizado", status: projectsdomain.ProjectStatusFinished, finished: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newStoryAccessTestEnv(t)
			ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			memberID, memberToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			projectID := backlogSeedProject(t, env.db, ownerID, tc.status, memberID)
			storyID := storyAccessSeedStory(t, env.db, projectID, ownerID)

			rec, queries := env.call(t, memberToken, storyID.String())

			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, 1, env.probe.executions, "el manejador protegido debería ejecutarse una vez")
			assert.Equal(t, int64(1), queries, "el acceso debería resolverse con una sola consulta")
			require.True(t, env.probe.hasAccess, "el middleware debería dejar el StoryAccess en el contexto")
			require.NotNil(t, env.probe.access)
			assert.Equal(t, storyID, env.probe.access.StoryID)
			assert.Equal(t, projectID, env.probe.access.ProjectID)
			assert.Equal(t, tc.finished, env.probe.access.ProjectFinished)
		})
	}
}

// Prueba transversal de RequireStoryAccess (T039): una historia ajena responde como una inexistente.
// La historia de un proyecto del que quien pide no es integrante, la de un proyecto del que fue
// quitado y un identificador que no es UUID responden el mismo 404 que un identificador UUID que no
// existe: idéntico byte a byte en estado, tipo de contenido y cuerpo ("La historia no existe."), sin
// ejecutar el manejador protegido y con a lo sumo una consulta a la base (FR-044, FR-047, SC-006 y
// caso límite "Integrante quitado del proyecto").
func TestStoryAccess_HistoriaAjenaIgualAInexistente(t *testing.T) {
	env := newStoryAccessTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	removedID, removedToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	_, outsiderToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := backlogSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress, removedID)
	storyID := storyAccessSeedStory(t, env.db, projectID, removedID)

	// Bruno es quitado del proyecto con el repositorio real de la 002, que borra su membresía.
	require.NoError(t, projectsrepository.NewProjectMemberRepository(env.db).Delete(context.Background(), projectID, removedID),
		"no se pudo quitar al integrante del proyecto")

	// La referencia es un identificador UUID que no existe, pedido por quien no es integrante.
	missing, missingQueries := env.call(t, outsiderToken, uuid.New().String())
	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body.String())
	assert.Equal(t, storyNotFoundBody, missing.Body.String(), "inexistente: el cuerpo debería ser el del contrato")
	assert.Equal(t, int64(1), missingQueries, "inexistente: debería resolverse con una sola consulta")

	cases := []struct {
		name          string
		token         string
		storyID       string
		exactlyOneSQL bool
	}{
		{name: "historia de un proyecto ajeno", token: outsiderToken, storyID: storyID.String(), exactlyOneSQL: true},
		{name: "historia de un proyecto del que fue quitado", token: removedToken, storyID: storyID.String(), exactlyOneSQL: true},
		{name: "identificador que no es UUID", token: outsiderToken, storyID: "no-es-un-uuid", exactlyOneSQL: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, queries := env.call(t, tc.token, tc.storyID)

			// La respuesta es el 404 del contrato, indistinguible de la del identificador inexistente.
			assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, storyNotFoundBody, rec.Body.String(), "el cuerpo debería ser el del contrato")
			assert.Equal(t, missing.Code, rec.Code, "los códigos de estado deberían ser idénticos")
			assert.Equal(t, missing.Header().Get("Content-Type"), rec.Header().Get("Content-Type"),
				"los tipos de contenido deberían ser idénticos")
			assert.Equal(t, missing.Body.Bytes(), rec.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")

			// Se resuelve con una sola consulta; el identificador mal formado puede no consultar.
			if tc.exactlyOneSQL {
				assert.Equal(t, int64(1), queries, "debería resolverse con una sola consulta")
			} else {
				assert.LessOrEqual(t, queries, int64(1), "no debería hacer más de una consulta")
			}
		})
	}
	assert.Zero(t, env.probe.executions, "el manejador protegido no debería ejecutarse")
}

// Prueba transversal de RequireStoryAccess (T039): sin sesión válida responde 401.
// Montado sin el middleware de sesión, de modo que no hay titular en el contexto, RequireStoryAccess
// corta con el 401 de la 001 sin consultar la base y sin ejecutar el manejador protegido (FR-045).
func TestStoryAccess_SinSesion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	queries := newQueryCounter()
	probe := &storyAccessProbe{}
	router := gin.New()
	router.GET(storyAccessProbeRoute,
		RequireStoryAccess(repository.NewStoryRepository(db.Session(&gorm.Session{Logger: queries}))),
		probe.handle,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-historia/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, storyAccessSessionBody, rec.Body.String())
	assert.Zero(t, queries.count.Load(), "sin sesión no debería consultar la base")
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

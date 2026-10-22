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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/repository"
)

// Pruebas de integración del middleware de acceso a un sprint de US2 (T026). Montan
// RequireSprintAccess con httptest sobre un manejador protegido de prueba, detrás del middleware de
// sesión real de la 001 y con el repositorio real de sprints contra smye_test, y verifican que un
// integrante que no es el propietario pasa con su SprintAccess en el contexto (RN-01), que el sprint
// inexistente, el de un proyecto ajeno, el de un integrante ya quitado y un identificador que no es
// UUID responden el mismo 404 byte a byte con a lo sumo una consulta a la base, y que sin titular en
// el contexto se responde el 401 de la 001 (research.md sección 3; FR-056 a FR-058, SC-014). Los
// ayudantes llevan el prefijo "sprintAccess" para no chocar con los de handler_test.go.

// sprintNotFoundBody es el cuerpo exacto, byte a byte, del 404 de sprint inexistente del contrato.
const sprintNotFoundBody = `{"error":"El sprint no existe."}`

// sprintAccessSessionBody es el cuerpo exacto del 401 de la 001 cuando no hay titular de sesión.
const sprintAccessSessionBody = `{"error":"Se requiere iniciar sesión."}`

// sprintAccessProbeRoute es la ruta del manejador protegido de prueba; no existe en producción.
const sprintAccessProbeRoute = "/api/v1/solo-prueba-sprint/:sprintId"

// sprintAccessProbe registra cuántas veces se ejecutó el manejador protegido y qué SprintAccess
// encontró en el contexto en su última ejecución.
type sprintAccessProbe struct {
	executions int
	access     *domain.SprintAccess
	hasAccess  bool
}

// handle es el manejador protegido de prueba: cuenta su ejecución, guarda el SprintAccess que dejó
// el middleware en el contexto y responde 200.
func (p *sprintAccessProbe) handle(c *gin.Context) {
	p.executions++
	p.access, p.hasAccess = sprintAccessFromContext(c)
	c.Status(http.StatusOK)
}

// sprintQueryCounter es un logger de GORM que no escribe nada y cuenta cada sentencia SQL
// ejecutada: GORM llama a Trace una vez por sentencia, cualquiera sea el nivel de log, así que el
// contador mide cuántas consultas hizo el middleware durante una petición.
type sprintQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

// newSprintQueryCounter crea el contador sobre el logger mudo de GORM.
func newSprintQueryCounter() *sprintQueryCounter {
	return &sprintQueryCounter{Interface: logger.Discard}
}

// LogMode devuelve el mismo contador para que GORM no lo reemplace al cambiar de nivel.
func (q *sprintQueryCounter) LogMode(logger.LogLevel) logger.Interface {
	return q
}

// Trace suma una sentencia ejecutada.
func (q *sprintQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	q.count.Add(1)
}

// sprintAccessTestEnv reúne el router con el manejador de prueba, la base de test, el servicio de
// autenticación para crear cuentas y sesiones, la sonda del manejador protegido y el contador de
// consultas del repositorio que usa el middleware.
type sprintAccessTestEnv struct {
	router  *gin.Engine
	db      *gorm.DB
	auth    *authservice.AuthService
	probe   *sprintAccessProbe
	queries *sprintQueryCounter
}

// newSprintAccessTestEnv abre la base de test vacía y monta, en este orden, el middleware de sesión
// real de la 001, RequireSprintAccess con el repositorio real de sprints y el manejador protegido de
// prueba. El repositorio recibe una sesión de GORM con el contador de consultas como logger, de modo
// que solo se cuentan las consultas del middleware y no las de la preparación del escenario.
func newSprintAccessTestEnv(t *testing.T) sprintAccessTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	queries := newSprintQueryCounter()
	countedDB := db.Session(&gorm.Session{Logger: queries})
	probe := &sprintAccessProbe{}
	router := gin.New()
	router.GET(sprintAccessProbeRoute,
		authdelivery.NewHandler(auth).RequireSession,
		RequireSprintAccess(repository.NewSprintRepository(countedDB)),
		probe.handle,
	)
	return sprintAccessTestEnv{router: router, db: db, auth: auth, probe: probe, queries: queries}
}

// signUp registra una cuenta e inicia su sesión, devolviendo su identificador y su token.
func (e sprintAccessTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	return sprintTestEnv{auth: e.auth}.signUp(t, fullName, email)
}

// call pide el manejador protegido para el identificador de sprint indicado con el token como
// Bearer, con el contador de consultas en cero al empezar, y devuelve la respuesta y la cantidad de
// consultas que se hicieron durante la petición.
func (e sprintAccessTestEnv) call(t *testing.T, token, sprintID string) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	e.queries.count.Store(0)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-sprint/"+sprintID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, e.queries.count.Load()
}

// Prueba transversal de RequireSprintAccess (T026): un integrante accede a un sprint.
// Bruno, integrante que no es el propietario ni el autor del sprint, pide un sprint de su proyecto:
// el middleware hace una sola consulta, el manejador protegido se ejecuta una vez y encuentra en el
// contexto el SprintAccess con el sprint, su proyecto y si el proyecto está Finalizado (RN-01). El
// sprint de un proyecto Finalizado también se puede consultar y llega marcado (FR-059).
func TestSprintAccess_IntegranteNoPropietarioPasa(t *testing.T) {
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
			env := newSprintAccessTestEnv(t)
			ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			memberID, memberToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			projectID := sprintSeedProject(t, env.db, ownerID, tc.status, memberID)
			sprintID := sprintSeed(t, env.db, projectID, ownerID, sprintName, sprintStartDate, sprintEndDate)

			rec, queries := env.call(t, memberToken, sprintID.String())

			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, 1, env.probe.executions, "el manejador protegido debería ejecutarse una vez")
			assert.Equal(t, int64(1), queries, "el acceso debería resolverse con una sola consulta")
			require.True(t, env.probe.hasAccess, "el middleware debería dejar el SprintAccess en el contexto")
			require.NotNil(t, env.probe.access)
			assert.Equal(t, sprintID, env.probe.access.SprintID)
			assert.Equal(t, projectID, env.probe.access.ProjectID)
			assert.Equal(t, tc.finished, env.probe.access.ProjectFinished)
		})
	}
}

// Prueba transversal de RequireSprintAccess (T026): un sprint ajeno responde como uno inexistente.
// El sprint de un proyecto del que quien pide no es integrante, el de un proyecto del que fue
// quitado y un identificador que no es UUID responden el mismo 404 que un identificador UUID que no
// existe: idéntico byte a byte en estado, tipo de contenido y cuerpo ("El sprint no existe."), sin
// ejecutar el manejador protegido y con a lo sumo una consulta a la base (FR-057, FR-058, SC-014).
func TestSprintAccess_SprintAjenoIgualAInexistente(t *testing.T) {
	env := newSprintAccessTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	removedID, removedToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	_, outsiderToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := sprintSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress, removedID)
	sprintID := sprintSeed(t, env.db, projectID, removedID, sprintName, sprintStartDate, sprintEndDate)

	// Bruno es quitado del proyecto con el repositorio real de la 002, que borra su membresía.
	require.NoError(t, projectsrepository.NewProjectMemberRepository(env.db).Delete(context.Background(), projectID, removedID),
		"no se pudo quitar al integrante del proyecto")

	// La referencia es un identificador UUID que no existe, pedido por quien no es integrante.
	missing, missingQueries := env.call(t, outsiderToken, uuid.New().String())
	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body.String())
	assert.Equal(t, sprintNotFoundBody, missing.Body.String(), "inexistente: el cuerpo debería ser el del contrato")
	assert.Equal(t, int64(1), missingQueries, "inexistente: debería resolverse con una sola consulta")

	cases := []struct {
		name          string
		token         string
		sprintID      string
		exactlyOneSQL bool
	}{
		{name: "sprint de un proyecto ajeno", token: outsiderToken, sprintID: sprintID.String(), exactlyOneSQL: true},
		{name: "sprint de un proyecto del que fue quitado", token: removedToken, sprintID: sprintID.String(), exactlyOneSQL: true},
		{name: "identificador que no es UUID", token: outsiderToken, sprintID: "no-es-un-uuid", exactlyOneSQL: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, queries := env.call(t, tc.token, tc.sprintID)

			// La respuesta es el 404 del contrato, indistinguible de la del identificador inexistente.
			assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, sprintNotFoundBody, rec.Body.String(), "el cuerpo debería ser el del contrato")
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

// Prueba transversal de RequireSprintAccess (T026): sin sesión válida responde 401.
// Montado sin el middleware de sesión, de modo que no hay titular en el contexto,
// RequireSprintAccess corta con el 401 de la 001 sin consultar la base y sin ejecutar el manejador
// protegido.
func TestSprintAccess_SinSesion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	queries := newSprintQueryCounter()
	probe := &sprintAccessProbe{}
	router := gin.New()
	router.GET(sprintAccessProbeRoute,
		RequireSprintAccess(repository.NewSprintRepository(db.Session(&gorm.Session{Logger: queries}))),
		probe.handle,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-sprint/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, sprintAccessSessionBody, rec.Body.String())
	assert.Zero(t, queries.count.Load(), "sin sesión no debería consultar la base")
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

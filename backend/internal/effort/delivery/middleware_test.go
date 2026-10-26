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
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
)

// Pruebas de integración del middleware de acceso a un registro de esfuerzo de US3 (T043). Montan
// RequireEffortEntryAccess con httptest sobre un manejador protegido de prueba, detrás del middleware
// de sesión real de la 001 y con el repositorio real de esfuerzo contra smye_test, y verifican que un
// integrante vigente pasa con su EntryAccess en el contexto sea o no el autor del registro, que el
// identificador mal formado, el registro inexistente, el de un proyecto ajeno y el propio de una
// persona quitada del proyecto responden el mismo 404 byte a byte con a lo sumo una consulta a la
// base, y que sin titular en el contexto se responde el 401 de la 001 (research.md sección 3;
// FR-057, FR-059, SC-018; 006/US3).

// entryNotFoundBody es el cuerpo exacto, byte a byte, del 404 de registro de esfuerzo inexistente del
// contrato.
const entryNotFoundBody = `{"error":"El registro de esfuerzo no existe."}`

// entryAccessSessionBody es el cuerpo exacto del 401 de la 001 cuando no hay titular de sesión.
const entryAccessSessionBody = `{"error":"Se requiere iniciar sesión."}`

// entryAccessProbeRoute es la ruta del manejador protegido de prueba; no existe en producción.
const entryAccessProbeRoute = "/api/v1/solo-prueba-registro/:entryId"

// entryAccessProbe registra cuántas veces se ejecutó el manejador protegido y qué EntryAccess
// encontró en el contexto en su última ejecución.
type entryAccessProbe struct {
	executions int
	access     *domain.EntryAccess
	hasAccess  bool
}

// handle es el manejador protegido de prueba: cuenta su ejecución, guarda el EntryAccess que dejó el
// middleware en el contexto y responde 200.
func (p *entryAccessProbe) handle(c *gin.Context) {
	p.executions++
	p.access, p.hasAccess = entryAccessFromContext(c)
	c.Status(http.StatusOK)
}

// entryAccessQueryCounter es un logger de GORM que no escribe nada y cuenta cada sentencia SQL
// ejecutada: GORM llama a Trace una vez por sentencia, cualquiera sea el nivel de log, así que el
// contador mide cuántas consultas hizo el middleware durante una petición.
type entryAccessQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

// LogMode devuelve el mismo contador para que GORM no lo reemplace al cambiar de nivel.
func (q *entryAccessQueryCounter) LogMode(logger.LogLevel) logger.Interface {
	return q
}

// Trace suma una sentencia ejecutada.
func (q *entryAccessQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	q.count.Add(1)
}

// entryAccessTestEnv reúne el router con el manejador de prueba, la base de test, el servicio de
// autenticación para crear cuentas y sesiones, la sonda del manejador protegido y el contador de
// consultas del repositorio que usa el middleware.
type entryAccessTestEnv struct {
	router  *gin.Engine
	db      *gorm.DB
	auth    *authservice.AuthService
	probe   *entryAccessProbe
	queries *entryAccessQueryCounter
}

// newEntryAccessTestEnv abre la base de test vacía y monta, en este orden, el middleware de sesión
// real de la 001, RequireEffortEntryAccess con el repositorio real de esfuerzo y el manejador
// protegido de prueba. El repositorio recibe una sesión de GORM con el contador de consultas como
// logger, de modo que solo se cuentan las consultas del middleware y no las de la preparación.
func newEntryAccessTestEnv(t *testing.T) entryAccessTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	queries := &entryAccessQueryCounter{Interface: logger.Discard}
	countedDB := db.Session(&gorm.Session{Logger: queries})
	probe := &entryAccessProbe{}
	router := gin.New()
	router.GET(entryAccessProbeRoute,
		authdelivery.NewHandler(auth).RequireSession,
		RequireEffortEntryAccess(repository.NewEffortRepository(countedDB)),
		probe.handle,
	)
	return entryAccessTestEnv{router: router, db: db, auth: auth, probe: probe, queries: queries}
}

// signUp registra una cuenta con el servicio de la 001, inicia su sesión y devuelve su identificador
// y su token.
func (e entryAccessTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	user, err := e.auth.Register(ctx, fullName, email, effortPassword)
	require.NoError(t, err, "no se pudo registrar la cuenta de prueba %s", email)
	login, err := e.auth.Login(ctx, email, effortPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de prueba %s", email)
	return user.ID, login.Token
}

// call pide el manejador protegido para el identificador de registro indicado con el token como
// Bearer, con el contador de consultas en cero al empezar, y devuelve la respuesta y la cantidad de
// consultas que se hicieron durante la petición.
func (e entryAccessTestEnv) call(t *testing.T, token, entryID string) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	e.queries.count.Store(0)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-registro/"+entryID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, e.queries.count.Load()
}

// entryAccessSeedProject inserta directamente en la base un proyecto del propietario en el estado
// indicado, con las membresías del propietario y de cada integrante adicional, y devuelve su
// identificador, para preparar el "Dado" sin pasar por la API de la 002.
func entryAccessSeedProject(t *testing.T, db *gorm.DB, ownerID uuid.UUID, status projectsdomain.ProjectStatus, extraMembers ...uuid.UUID) uuid.UUID {
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

// entryAccessSeedStory inserta directamente en la base una historia pendiente del proyecto, creada
// por el autor indicado, y devuelve su identificador.
func entryAccessSeedStory(t *testing.T, db *gorm.DB, projectID, authorID uuid.UUID) uuid.UUID {
	t.Helper()
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       "Alta de clientes desde el portal",
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	return story.ID
}

// entryAccessSeedEntry inserta directamente en la base un registro de esfuerzo sin sprint de la
// persona sobre la historia y devuelve su identificador, para preparar el "Dado" sin depender del
// alta.
func entryAccessSeedEntry(t *testing.T, db *gorm.DB, storyID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	entry := domain.EffortEntry{
		ID:              uuid.New(),
		StoryID:         storyID,
		UserID:          userID,
		EntryDate:       time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Activity:        "Maquetado del formulario de alta",
		HoursHundredths: 400,
	}
	require.NoError(t, db.Create(&entry).Error, "no se pudo sembrar el registro de esfuerzo")
	return entry.ID
}

// Prueba transversal de RequireEffortEntryAccess (T043): un integrante accede a un registro.
// Un integrante vigente del proyecto pide un registro de esfuerzo de una historia del proyecto, sea
// el autor o no lo sea (la autoría la decide el servicio, research.md sección 3): el middleware hace
// una sola consulta, el manejador protegido se ejecuta una vez y encuentra en el contexto el
// EntryAccess con el registro, su historia, su proyecto, su autor y si el proyecto está Finalizado
// (FR-057, FR-058, FR-060; 006/US3).
func TestEntryAccess_IntegrantePasa(t *testing.T) {
	cases := []struct {
		name     string
		asAuthor bool
		status   projectsdomain.ProjectStatus
		finished bool
	}{
		{name: "autor del registro", asAuthor: true, status: projectsdomain.ProjectStatusInProgress, finished: false},
		{name: "integrante que no es el autor", asAuthor: false, status: projectsdomain.ProjectStatusInProgress, finished: false},
		{name: "autor del registro en un proyecto finalizado", asAuthor: true, status: projectsdomain.ProjectStatusFinished, finished: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newEntryAccessTestEnv(t)
			ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			authorID, authorToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			colleagueID, colleagueToken := env.signUp(t, "Carla Díaz", "carla@example.com")
			projectID := entryAccessSeedProject(t, env.db, ownerID, tc.status, authorID, colleagueID)
			storyID := entryAccessSeedStory(t, env.db, projectID, ownerID)
			entryID := entryAccessSeedEntry(t, env.db, storyID, authorID)

			// Pide el autor o un integrante que no es el autor, según el caso.
			token := colleagueToken
			if tc.asAuthor {
				token = authorToken
			}
			rec, queries := env.call(t, token, entryID.String())

			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, 1, env.probe.executions, "el manejador protegido debería ejecutarse una vez")
			assert.Equal(t, int64(1), queries, "el acceso debería resolverse con una sola consulta")
			require.True(t, env.probe.hasAccess, "el middleware debería dejar el EntryAccess en el contexto")
			require.NotNil(t, env.probe.access)
			assert.Equal(t, entryID, env.probe.access.EntryID)
			assert.Equal(t, storyID, env.probe.access.StoryID)
			assert.Equal(t, projectID, env.probe.access.ProjectID)
			assert.Equal(t, authorID, env.probe.access.AuthorID, "el autor debería ser quien registró, no quien pide")
			assert.Equal(t, tc.finished, env.probe.access.ProjectFinished)
		})
	}
}

// Prueba transversal de RequireEffortEntryAccess (T043): un registro ajeno responde como uno
// inexistente.
// El registro de un proyecto del que quien pide no es integrante, el registro propio de una persona
// que fue quitada del proyecto y un identificador que no es UUID responden el mismo 404 que un
// identificador UUID que no existe: idéntico byte a byte en estado, tipo de contenido y cuerpo ("El
// registro de esfuerzo no existe."), sin ejecutar el manejador protegido y con a lo sumo una
// consulta a la base (FR-057, FR-059, SC-018 y escenario 18 de 006/US3).
func TestEntryAccess_RegistroAjenoIgualAInexistente(t *testing.T) {
	env := newEntryAccessTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	removedID, removedToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	_, outsiderToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID := entryAccessSeedProject(t, env.db, ownerID, projectsdomain.ProjectStatusInProgress, removedID)
	storyID := entryAccessSeedStory(t, env.db, projectID, ownerID)
	entryID := entryAccessSeedEntry(t, env.db, storyID, removedID)

	// Bruno, autor del registro, es quitado del proyecto con el repositorio real de la 002, que
	// borra su membresía; el registro se conserva.
	require.NoError(t, projectsrepository.NewProjectMemberRepository(env.db).Delete(context.Background(), projectID, removedID),
		"no se pudo quitar al integrante del proyecto")

	// La referencia es un identificador UUID que no existe, pedido por quien no es integrante.
	missing, missingQueries := env.call(t, outsiderToken, uuid.New().String())
	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body.String())
	assert.Equal(t, entryNotFoundBody, missing.Body.String(), "inexistente: el cuerpo debería ser el del contrato")
	assert.Equal(t, int64(1), missingQueries, "inexistente: debería resolverse con una sola consulta")

	cases := []struct {
		name          string
		token         string
		entryID       string
		exactlyOneSQL bool
	}{
		{name: "registro de un proyecto ajeno", token: outsiderToken, entryID: entryID.String(), exactlyOneSQL: true},
		{name: "registro propio de una persona quitada del proyecto", token: removedToken, entryID: entryID.String(), exactlyOneSQL: true},
		{name: "identificador que no es UUID", token: outsiderToken, entryID: "no-es-un-uuid", exactlyOneSQL: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, queries := env.call(t, tc.token, tc.entryID)

			// La respuesta es el 404 del contrato, indistinguible de la del identificador inexistente.
			assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, entryNotFoundBody, rec.Body.String(), "el cuerpo debería ser el del contrato")
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

	// El registro de la persona quitada se conserva (FR-028).
	var stored int64
	require.NoError(t, env.db.Model(&domain.EffortEntry{}).Where("id = ?", entryID).Count(&stored).Error)
	assert.Equal(t, int64(1), stored, "el registro de la persona quitada debería conservarse")
}

// Prueba transversal de RequireEffortEntryAccess (T043): sin sesión válida responde 401.
// Montado sin el middleware de sesión, de modo que no hay titular en el contexto,
// RequireEffortEntryAccess corta con el 401 de la 001 sin consultar la base y sin ejecutar el
// manejador protegido (FR-056).
func TestEntryAccess_SinSesion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	queries := &entryAccessQueryCounter{Interface: logger.Discard}
	probe := &entryAccessProbe{}
	router := gin.New()
	router.GET(entryAccessProbeRoute,
		RequireEffortEntryAccess(repository.NewEffortRepository(db.Session(&gorm.Session{Logger: queries}))),
		probe.handle,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-registro/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, entryAccessSessionBody, rec.Body.String())
	assert.Zero(t, queries.count.Load(), "sin sesión no debería consultar la base")
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

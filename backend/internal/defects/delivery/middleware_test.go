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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/repository"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	projectsrepository "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del middleware de acceso a un defecto de US2 (T027). Montan
// RequireDefectAccess con httptest sobre un manejador protegido de prueba, detrás del middleware de
// sesión real de la 001 y con el repositorio real de defectos contra smye_test, y verifican que un
// integrante vigente pasa con su DefectAccess en el contexto sea o no el autor del defecto, que el
// identificador mal formado, el defecto inexistente, el de un proyecto ajeno y el registrado por una
// persona ya quitada del proyecto, pedido por ella, responden el mismo 404 byte a byte con a lo sumo
// una consulta a la base, y que sin titular en el contexto se responde el 401 de la 001
// (research.md sección 3; FR-051, FR-053, FR-055, SC-017).

// defectNotFoundBody es el cuerpo exacto, byte a byte, del 404 de defecto inexistente del contrato.
const defectNotFoundBody = `{"error":"El defecto no existe."}`

// defectAccessSessionBody es el cuerpo exacto del 401 de la 001 cuando no hay titular de sesión.
const defectAccessSessionBody = `{"error":"Se requiere iniciar sesión."}`

// defectAccessProbeRoute es la ruta del manejador protegido de prueba; no existe en producción.
const defectAccessProbeRoute = "/api/v1/solo-prueba-defecto/:defectId"

// defectAccessProbe registra cuántas veces se ejecutó el manejador protegido y qué DefectAccess
// encontró en el contexto en su última ejecución.
type defectAccessProbe struct {
	executions int
	access     *domain.DefectAccess
	hasAccess  bool
}

// handle es el manejador protegido de prueba: cuenta su ejecución, guarda el DefectAccess que dejó el
// middleware en el contexto y responde 200.
func (p *defectAccessProbe) handle(c *gin.Context) {
	p.executions++
	p.access, p.hasAccess = defectAccessFromContext(c)
	c.Status(http.StatusOK)
}

// defectAccessQueryCounter es un logger de GORM que no escribe nada y cuenta cada sentencia SQL
// ejecutada: GORM llama a Trace una vez por sentencia, cualquiera sea el nivel de log, así que el
// contador mide cuántas consultas hizo el middleware durante una petición.
type defectAccessQueryCounter struct {
	logger.Interface
	count atomic.Int64
}

// LogMode devuelve el mismo contador para que GORM no lo reemplace al cambiar de nivel.
func (q *defectAccessQueryCounter) LogMode(logger.LogLevel) logger.Interface {
	return q
}

// Trace suma una sentencia ejecutada.
func (q *defectAccessQueryCounter) Trace(context.Context, time.Time, func() (string, int64), error) {
	q.count.Add(1)
}

// defectAccessTestEnv reúne el router con el manejador de prueba, la base de test, el servicio de
// autenticación para crear cuentas y sesiones, la sonda del manejador protegido y el contador de
// consultas del repositorio que usa el middleware.
type defectAccessTestEnv struct {
	router  *gin.Engine
	db      *gorm.DB
	auth    *authservice.AuthService
	probe   *defectAccessProbe
	queries *defectAccessQueryCounter
}

// newDefectAccessTestEnv abre la base de test vacía y monta, en este orden, el middleware de sesión
// real de la 001, RequireDefectAccess con el repositorio real de defectos y el manejador protegido de
// prueba. El repositorio recibe una sesión de GORM con el contador de consultas como logger, de modo
// que solo se cuentan las consultas del middleware y no las de la preparación.
func newDefectAccessTestEnv(t *testing.T) defectAccessTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	queries := &defectAccessQueryCounter{Interface: logger.Discard}
	countedDB := db.Session(&gorm.Session{Logger: queries})
	probe := &defectAccessProbe{}
	router := gin.New()
	router.GET(defectAccessProbeRoute,
		authdelivery.NewHandler(auth).RequireSession,
		RequireDefectAccess(repository.NewDefectRepository(countedDB)),
		probe.handle,
	)
	return defectAccessTestEnv{router: router, db: db, auth: auth, probe: probe, queries: queries}
}

// signUp registra una cuenta con el servicio de la 001, inicia su sesión y devuelve su identificador
// y su token.
func (e defectAccessTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	user, err := e.auth.Register(ctx, fullName, email, defectPassword)
	require.NoError(t, err, "no se pudo registrar la cuenta de prueba %s", email)
	login, err := e.auth.Login(ctx, email, defectPassword)
	require.NoError(t, err, "no se pudo iniciar la sesión de prueba %s", email)
	return user.ID, login.Token
}

// call pide el manejador protegido para el identificador de defecto indicado con el token como
// Bearer, con el contador de consultas en cero al empezar, y devuelve la respuesta y la cantidad de
// consultas que se hicieron durante la petición.
func (e defectAccessTestEnv) call(t *testing.T, token, defectID string) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	e.queries.count.Store(0)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-defecto/"+defectID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, e.queries.count.Load()
}

// defectAccessSeed inserta directamente en la base, para preparar el "Dado" sin pasar por las APIs de
// la 002, la 003, la 004 ni por el alta de defectos: un proyecto del propietario en el estado
// indicado con las membresías del propietario y de cada integrante adicional, una historia, un sprint
// Activo y un defecto Abierto de esa historia registrado por el autor indicado. Devuelve el
// identificador del proyecto y el del defecto.
func defectAccessSeed(t *testing.T, db *gorm.DB, ownerID, authorID uuid.UUID, status projectsdomain.ProjectStatus, extraMembers ...uuid.UUID) (uuid.UUID, uuid.UUID) {
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

	// La historia y el sprint de detección del defecto.
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Title:       "Reporte mensual de ventas",
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		CreatedByID: ownerID,
	}
	require.NoError(t, db.Create(&story).Error, "no se pudo sembrar la historia")
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Name:        "Sprint 1",
		Goal:        "Reportes con los totales correctos.",
		StartDate:   time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
		Status:      sprintsdomain.SprintStatusActive,
		CreatedByID: ownerID,
	}
	require.NoError(t, db.Create(&sprint).Error, "no se pudo sembrar el sprint")

	// El defecto Abierto, con el autor como titular de todas sus escrituras.
	defect := domain.Defect{
		ID:                uuid.New(),
		StoryID:           story.ID,
		Description:       "El total del reporte no suma los descuentos.",
		Severity:          domain.DefectSeverityHigh,
		Status:            domain.DefectStatusOpen,
		DetectedSprintID:  sprint.ID,
		CreatedByID:       authorID,
		UpdatedByID:       authorID,
		StatusChangedByID: authorID,
		StatusChangedAt:   time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC),
	}
	require.NoError(t, db.Create(&defect).Error, "no se pudo sembrar el defecto")
	return project.ID, defect.ID
}

// Prueba transversal de RequireDefectAccess (T027): un integrante accede a un defecto.
// Un integrante vigente del proyecto pide un defecto de una historia del proyecto, sea el autor o no
// lo sea (no hay propiedad individual sobre un defecto, FR-052): el middleware hace una sola
// consulta, el manejador protegido se ejecuta una vez y encuentra en el contexto el DefectAccess con
// el defecto, su proyecto y si el proyecto está Finalizado (research.md sección 3; FR-051).
func TestDefectAccess_IntegrantePasa(t *testing.T) {
	cases := []struct {
		name     string
		asAuthor bool
		status   projectsdomain.ProjectStatus
		finished bool
	}{
		{name: "autor del defecto", asAuthor: true, status: projectsdomain.ProjectStatusInProgress, finished: false},
		{name: "integrante que no es el autor", asAuthor: false, status: projectsdomain.ProjectStatusInProgress, finished: false},
		{name: "integrante en un proyecto finalizado", asAuthor: false, status: projectsdomain.ProjectStatusFinished, finished: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDefectAccessTestEnv(t)
			ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
			authorID, authorToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
			colleagueID, colleagueToken := env.signUp(t, "Carla Díaz", "carla@example.com")
			projectID, defectID := defectAccessSeed(t, env.db, ownerID, authorID, tc.status, authorID, colleagueID)

			// Pide el autor o un integrante que no es el autor, según el caso.
			token := colleagueToken
			if tc.asAuthor {
				token = authorToken
			}
			rec, queries := env.call(t, token, defectID.String())

			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, 1, env.probe.executions, "el manejador protegido debería ejecutarse una vez")
			assert.Equal(t, int64(1), queries, "el acceso debería resolverse con una sola consulta")
			require.True(t, env.probe.hasAccess, "el middleware debería dejar el DefectAccess en el contexto")
			require.NotNil(t, env.probe.access)
			assert.Equal(t, defectID, env.probe.access.DefectID)
			assert.Equal(t, projectID, env.probe.access.ProjectID)
			assert.Equal(t, tc.finished, env.probe.access.ProjectFinished)
		})
	}
}

// Prueba transversal de RequireDefectAccess (T027): un defecto ajeno responde como uno inexistente.
// El defecto de un proyecto del que quien pide no es integrante, el defecto registrado por una
// persona que después fue quitada del proyecto, pedido por ella, y un identificador que no es UUID
// responden el mismo 404 que un identificador UUID que no existe: idéntico byte a byte en estado,
// tipo de contenido y cuerpo ("El defecto no existe."), sin ejecutar el manejador protegido y con a
// lo sumo una consulta a la base. El defecto de la persona quitada se conserva (FR-053, FR-055,
// SC-017).
func TestDefectAccess_DefectoAjenoIgualAInexistente(t *testing.T) {
	env := newDefectAccessTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	removedID, removedToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	_, outsiderToken := env.signUp(t, "Carla Díaz", "carla@example.com")
	projectID, defectID := defectAccessSeed(t, env.db, ownerID, removedID, projectsdomain.ProjectStatusInProgress, removedID)

	// Bruno, autor del defecto, es quitado del proyecto con el repositorio real de la 002, que borra
	// su membresía; el defecto se conserva.
	require.NoError(t, projectsrepository.NewProjectMemberRepository(env.db).Delete(context.Background(), projectID, removedID),
		"no se pudo quitar al integrante del proyecto")

	// La referencia es un identificador UUID que no existe, pedido por quien no es integrante.
	missing, missingQueries := env.call(t, outsiderToken, uuid.New().String())
	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body.String())
	assert.Equal(t, defectNotFoundBody, missing.Body.String(), "inexistente: el cuerpo debería ser el del contrato")
	assert.Equal(t, int64(1), missingQueries, "inexistente: debería resolverse con una sola consulta")

	cases := []struct {
		name          string
		token         string
		defectID      string
		exactlyOneSQL bool
	}{
		{name: "defecto de un proyecto ajeno", token: outsiderToken, defectID: defectID.String(), exactlyOneSQL: true},
		{name: "defecto registrado por una persona quitada del proyecto, pedido por ella", token: removedToken, defectID: defectID.String(), exactlyOneSQL: true},
		{name: "identificador que no es UUID", token: outsiderToken, defectID: "no-es-un-uuid", exactlyOneSQL: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, queries := env.call(t, tc.token, tc.defectID)

			// La respuesta es el 404 del contrato, indistinguible de la del identificador inexistente.
			assert.Equal(t, http.StatusNotFound, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, defectNotFoundBody, rec.Body.String(), "el cuerpo debería ser el del contrato")
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

	// El defecto registrado por la persona quitada se conserva (FR-055).
	var stored int64
	require.NoError(t, env.db.Model(&domain.Defect{}).Where("id = ?", defectID).Count(&stored).Error)
	assert.Equal(t, int64(1), stored, "el defecto de la persona quitada debería conservarse")
}

// Prueba transversal de RequireDefectAccess (T027): sin sesión válida responde 401.
// Montado sin el middleware de sesión, de modo que no hay titular en el contexto, RequireDefectAccess
// corta con el 401 de la 001 sin consultar la base y sin ejecutar el manejador protegido.
func TestDefectAccess_SinSesion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	queries := &defectAccessQueryCounter{Interface: logger.Discard}
	probe := &defectAccessProbe{}
	router := gin.New()
	router.GET(defectAccessProbeRoute,
		RequireDefectAccess(repository.NewDefectRepository(db.Session(&gorm.Session{Logger: queries}))),
		probe.handle,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-defecto/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.Equal(t, defectAccessSessionBody, rec.Body.String())
	assert.Zero(t, queries.count.Load(), "sin sesión no debería consultar la base")
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

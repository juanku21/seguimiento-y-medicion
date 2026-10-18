package delivery

import (
	"net/http"
	"net/http/httptest"
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
)

// Pruebas unitarias obligatorias del middleware de membresía de US2 (T032). Montan RequireMembership
// con httptest sobre un manejador protegido de prueba, detrás del middleware de sesión real de la
// 001 y con el repositorio real contra smye_test, y verifican la regla RN-03: un integrante pasa con
// su membresía en el contexto, mientras que quien no es integrante y un identificador inexistente
// reciben respuestas 404 byte a byte idénticas sin que el manejador se ejecute (FR-049, SC-005).
// Sin identidad en el contexto, el middleware corta con el 401 de la 001 (FR-020 de la 001).

// projectNotFoundMessage es el texto exacto del 404 ProyectoInexistente de contracts/openapi.yaml.
const projectNotFoundMessage = "El proyecto no existe."

// membershipProbeRoute es la ruta del manejador protegido de prueba; no existe en producción.
const membershipProbeRoute = "/api/v1/solo-prueba-membresia/:projectId"

// membershipProbe registra cuántas veces se ejecutó el manejador protegido y qué membresía encontró
// en el contexto en su última ejecución.
type membershipProbe struct {
	executions    int
	membership    *domain.ProjectMember
	hasMembership bool
}

// membershipTestEnv reúne el router con el manejador de prueba, la base de test, el servicio de
// autenticación para crear cuentas y sesiones, y la sonda del manejador protegido.
type membershipTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	auth   *authservice.AuthService
	probe  *membershipProbe
}

// newMembershipTestEnv abre la base de test vacía y monta, en este orden, el middleware de sesión
// real de la 001, RequireMembership con el repositorio real y el manejador protegido de prueba, que
// anota su ejecución y la membresía del contexto y responde 200.
func newMembershipTestEnv(t *testing.T) membershipTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cfg := testsupport.OpenDatabase(t)
	auth := authservice.NewAuthService(
		authrepository.NewUserRepository(db),
		authrepository.NewSessionRepository(db),
		authrepository.NewAuthEventRepository(db),
		cfg.JWTSecret,
	)
	probe := &membershipProbe{}
	router := gin.New()
	router.GET(membershipProbeRoute,
		authdelivery.NewHandler(auth).RequireSession,
		RequireMembership(repository.NewProjectMemberRepository(db)),
		probe.handle,
	)
	return membershipTestEnv{router: router, db: db, auth: auth, probe: probe}
}

// handle es el manejador protegido de prueba: cuenta su ejecución y guarda la membresía que dejó el
// middleware en el contexto.
func (p *membershipProbe) handle(c *gin.Context) {
	p.executions++
	p.membership, p.hasMembership = membershipFromContext(c)
	c.Status(http.StatusOK)
}

// signUp registra una cuenta e inicia su sesión, devolviendo su identificador y su token.
func (e membershipTestEnv) signUp(t *testing.T, fullName, email string) (uuid.UUID, string) {
	t.Helper()
	return projectsTestEnv{auth: e.auth}.signUp(t, fullName, email)
}

// call pide el manejador protegido para el proyecto indicado con el token como Bearer.
func (e membershipTestEnv) call(t *testing.T, token, projectID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-membresia/"+projectID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// seedProjectWithMembers inserta en la base un proyecto del propietario con su membresía y la de
// cada integrante adicional, y devuelve su identificador, sin pasar por la API.
func seedProjectWithMembers(t *testing.T, db *gorm.DB, ownerID uuid.UUID, name string, extraMembers ...uuid.UUID) uuid.UUID {
	t.Helper()
	project := domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         name,
		NameNormalized:               name,
		StartDate:                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       domain.ProjectStatusPlanned,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto %q", name)
	for _, userID := range append([]uuid.UUID{ownerID}, extraMembers...) {
		member := domain.ProjectMember{ID: uuid.New(), ProjectID: project.ID, UserID: userID}
		require.NoError(t, db.Create(&member).Error, "no se pudo sembrar la membresía de %s", userID)
	}
	return project.ID
}

// Escenario: Un integrante consulta su proyecto (002/US2)
// El propietario y un integrante que no es propietario pasan el middleware: el manejador protegido
// se ejecuta una vez y encuentra en el contexto la membresía de quien pide en ese proyecto (RN-03).
func TestUS2_MembresiaIntegrantePasa(t *testing.T) {
	cases := []struct {
		name  string
		owner bool
	}{
		{name: "propietario", owner: true},
		{name: "integrante que no es propietario", owner: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newMembershipTestEnv(t)
			ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
			memberID, memberToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
			projectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes", memberID)

			userID, token := memberID, memberToken
			if tc.owner {
				userID, token = ownerID, ownerToken
			}
			rec := env.call(t, token, projectID.String())

			require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
			assert.Equal(t, 1, env.probe.executions, "el manejador protegido debería ejecutarse una vez")
			require.True(t, env.probe.hasMembership, "el middleware debería dejar la membresía en el contexto")
			require.NotNil(t, env.probe.membership)
			assert.Equal(t, projectID, env.probe.membership.ProjectID)
			assert.Equal(t, userID, env.probe.membership.UserID)
		})
	}
}

// Escenario: Proyecto ajeno idéntico a uno inexistente (002/US2)
// Quien no es integrante pide un proyecto existente de otra persona y luego un identificador que no
// existe: las dos respuestas son 404 con "El proyecto no existe.", byte a byte idénticas en estado,
// tipo de contenido y cuerpo, y el manejador protegido no se ejecuta en ninguno de los dos casos
// (FR-049, SC-005).
func TestUS2_MembresiaProyectoAjenoIgualAInexistente(t *testing.T) {
	env := newMembershipTestEnv(t)
	ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
	_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
	foreignProjectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes")

	foreign := env.call(t, outsiderToken, foreignProjectID.String())
	missing := env.call(t, outsiderToken, uuid.New().String())

	// Cada respuesta es el 404 del contrato y ninguna alcanza el manejador protegido.
	for name, rec := range map[string]*httptest.ResponseRecorder{"proyecto ajeno": foreign, "identificador inexistente": missing} {
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s: cuerpo: %s", name, rec.Body.String())
		assert.JSONEq(t, `{"error":"`+projectNotFoundMessage+`"}`, rec.Body.String(), "%s", name)
	}
	assert.Zero(t, env.probe.executions, "el manejador protegido no debería ejecutarse")

	// Las dos respuestas son indistinguibles byte a byte.
	assert.Equal(t, missing.Code, foreign.Code, "los códigos de estado deberían ser idénticos")
	assert.Equal(t, missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missing.Body.Bytes(), foreign.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
}

// Escenario: Sin sesión válida (002/US2)
// Montado sin el middleware de sesión, de modo que no hay titular en el contexto, RequireMembership
// corta con el 401 de la 001 y el manejador protegido no se ejecuta (FR-020 de la 001, FR-046).
func TestUS2_MembresiaSinSesion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := testsupport.OpenDatabase(t)
	probe := &membershipProbe{}
	router := gin.New()
	router.GET(membershipProbeRoute, RequireMembership(repository.NewProjectMemberRepository(db)), probe.handle)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-membresia/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "cuerpo: %s", rec.Body.String())
	assert.JSONEq(t, `{"error":"`+projectSessionMessage+`"}`, rec.Body.String())
	assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")
}

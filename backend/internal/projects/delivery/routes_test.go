package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Prueba de integración del cableado de rutas del módulo de proyectos (T089). A diferencia de
// middleware_test.go, que arma el grupo a mano, acá el manejador protegido de prueba cuelga del
// subgrupo /api/v1/projects/:projectId que devuelve RegisterRoutes, compuesto como en cmd/app con el
// middleware de sesión real de la 001 y el repositorio de membresías real contra smye_test. Así se
// verifica que toda ruta de un proyecto concreto queda detrás de RequireMembership (FR-049, SC-005).

// projectGroupPath es la ruta base que debe tener el subgrupo devuelto por RegisterRoutes.
const projectGroupPath = "/api/v1/projects/:projectId"

// routesProbeSuffix es el sufijo del manejador protegido de prueba dentro del subgrupo; no existe en
// producción y no choca con las rutas que agregan las historias siguientes.
const routesProbeSuffix = "/solo-prueba-rutas"

// newRoutesProbe monta el manejador protegido de prueba sobre el subgrupo que devolvió
// RegisterRoutes, después de exigir que ese subgrupo sea /api/v1/projects/:projectId.
func newRoutesProbe(t *testing.T, env projectsTestEnv) *membershipProbe {
	t.Helper()
	require.NotNil(t, env.projectGroup, "RegisterRoutes debería devolver el subgrupo del proyecto")
	require.Equal(t, projectGroupPath, env.projectGroup.BasePath(), "RegisterRoutes debería devolver el subgrupo /:projectId")
	probe := &membershipProbe{}
	env.projectGroup.GET(routesProbeSuffix, probe.handle)
	return probe
}

// probePath arma la ruta concreta del manejador de prueba para el identificador indicado.
func probePath(projectID string) string {
	return projectsPath + "/" + projectID + routesProbeSuffix
}

// Escenario: Proyecto ajeno es inexistente para las rutas del proyecto (002/US2)
// Sobre el subgrupo devuelto por RegisterRoutes, un integrante del proyecto alcanza el manejador con
// su membresía en el contexto, mientras que quien no es integrante de un proyecto existente y quien
// pide un identificador que no existe reciben el mismo 404 "El proyecto no existe.", byte a byte
// idéntico, sin que el manejador se ejecute (escenario 5 de US2, FR-049, SC-005).
func TestUS2_RutasProyectoAjenoIgualAInexistente(t *testing.T) {
	t.Run("un integrante alcanza el manejador del proyecto", func(t *testing.T) {
		env := newProjectsTestEnv(t)
		probe := newRoutesProbe(t, env)
		ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
		memberID, memberToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
		projectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes", memberID)

		rec := env.get(t, memberToken, probePath(projectID.String()))

		require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.Equal(t, 1, probe.executions, "el manejador protegido debería ejecutarse una vez")
		require.True(t, probe.hasMembership, "RequireMembership debería dejar la membresía en el contexto")
		require.NotNil(t, probe.membership)
		assert.Equal(t, projectID, probe.membership.ProjectID)
		assert.Equal(t, memberID, probe.membership.UserID)
	})

	t.Run("no integrante e identificador inexistente reciben el mismo 404", func(t *testing.T) {
		env := newProjectsTestEnv(t)
		probe := newRoutesProbe(t, env)
		ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
		_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
		foreignProjectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes")

		foreign := env.get(t, outsiderToken, probePath(foreignProjectID.String()))
		missing := env.get(t, outsiderToken, probePath(uuid.New().String()))

		// Cada respuesta es el 404 del contrato y ninguna alcanza el manejador protegido.
		for name, rec := range map[string]*httptest.ResponseRecorder{"proyecto ajeno": foreign, "identificador inexistente": missing} {
			assert.Equal(t, http.StatusNotFound, rec.Code, "%s: cuerpo: %s", name, rec.Body.String())
			assert.JSONEq(t, `{"error":"`+projectNotFoundMessage+`"}`, rec.Body.String(), "%s", name)
		}
		assert.Zero(t, probe.executions, "el manejador protegido no debería ejecutarse")

		// Las dos respuestas son indistinguibles byte a byte.
		assert.Equal(t, missing.Code, foreign.Code, "los códigos de estado deberían ser idénticos")
		assert.Equal(t, missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
		assert.Equal(t, missing.Body.Bytes(), foreign.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	})
}

// adminProbeSuffix es el sufijo del manejador de prueba dentro del subgrupo de administración; no
// existe en producción y no choca con las rutas de alta y baja de integrantes que agrega T052.
const adminProbeSuffix = "/solo-prueba-administracion"

// newAdminProbe monta el manejador de prueba sobre el subgrupo de administración que devolvió
// RegisterRoutes, después de exigir que ese subgrupo también cuelgue de /api/v1/projects/:projectId.
func newAdminProbe(t *testing.T, env projectsTestEnv) *membershipProbe {
	t.Helper()
	require.NotNil(t, env.adminGroup, "RegisterRoutes debería devolver el subgrupo de administración")
	require.Equal(t, projectGroupPath, env.adminGroup.BasePath(), "el subgrupo de administración debería colgar de /:projectId")
	probe := &membershipProbe{}
	env.adminGroup.GET(adminProbeSuffix, probe.handle)
	return probe
}

// adminProbePath arma la ruta concreta del manejador de administración para el identificador indicado.
func adminProbePath(projectID string) string {
	return projectsPath + "/" + projectID + adminProbeSuffix
}

// Escenario: No es propietario en las rutas de administración (002/US3)
// Sobre el subgrupo de administración devuelto por RegisterRoutes, el propietario alcanza el
// manejador; un integrante que no es propietario recibe 403 "Solo el propietario del proyecto puede
// realizar esta acción." sin que el manejador se ejecute; y quien no es integrante, igual que un
// identificador inexistente, recibe el mismo 404 de RequireMembership, porque el 403 nunca se evalúa
// antes que la membresía (escenario 9 de US3, FR-047, FR-050, FR-049, SC-006).
func TestUS3_RutasAdministracionSoloPropietario(t *testing.T) {
	t.Run("el propietario alcanza el manejador de administración", func(t *testing.T) {
		env := newProjectsTestEnv(t)
		probe := newAdminProbe(t, env)
		ownerID, ownerToken := env.signUp(t, "Ana Pérez", "ana@example.com")
		projectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes")

		rec := env.get(t, ownerToken, adminProbePath(projectID.String()))

		require.Equal(t, http.StatusOK, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.Equal(t, 1, probe.executions, "el manejador de administración debería ejecutarse una vez")
		require.True(t, probe.hasMembership, "RequireMembership debería dejar la membresía en el contexto")
		require.NotNil(t, probe.membership)
		assert.Equal(t, projectID, probe.membership.ProjectID)
		assert.Equal(t, ownerID, probe.membership.UserID)
	})

	t.Run("un integrante que no es propietario recibe 403", func(t *testing.T) {
		env := newProjectsTestEnv(t)
		probe := newAdminProbe(t, env)
		ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
		memberID, memberToken := env.signUp(t, "Bruno Díaz", "bruno@example.com")
		projectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes", memberID)

		rec := env.get(t, memberToken, adminProbePath(projectID.String()))

		assert.Equal(t, http.StatusForbidden, rec.Code, "cuerpo: %s", rec.Body.String())
		assert.JSONEq(t, `{"error":"`+memberNotOwnerMessage+`"}`, rec.Body.String())
		assert.Zero(t, probe.executions, "el manejador de administración no debería ejecutarse")
	})

	t.Run("no integrante e identificador inexistente reciben el mismo 404", func(t *testing.T) {
		env := newProjectsTestEnv(t)
		probe := newAdminProbe(t, env)
		ownerID, _ := env.signUp(t, "Ana Pérez", "ana@example.com")
		_, outsiderToken := env.signUp(t, "Carla Gómez", "carla@example.com")
		foreignProjectID := seedProjectWithMembers(t, env.db, ownerID, "portal de clientes")

		foreign := env.get(t, outsiderToken, adminProbePath(foreignProjectID.String()))
		missing := env.get(t, outsiderToken, adminProbePath(uuid.New().String()))

		// Ninguna respuesta es 403: la membresía corta antes con el 404 del contrato.
		for name, rec := range map[string]*httptest.ResponseRecorder{"proyecto ajeno": foreign, "identificador inexistente": missing} {
			assert.Equal(t, http.StatusNotFound, rec.Code, "%s: cuerpo: %s", name, rec.Body.String())
			assert.JSONEq(t, `{"error":"`+projectNotFoundMessage+`"}`, rec.Body.String(), "%s", name)
		}
		assert.Zero(t, probe.executions, "el manejador de administración no debería ejecutarse")

		// Las dos respuestas son indistinguibles byte a byte.
		assert.Equal(t, missing.Code, foreign.Code, "los códigos de estado deberían ser idénticos")
		assert.Equal(t, missing.Header().Get("Content-Type"), foreign.Header().Get("Content-Type"), "los tipos de contenido deberían ser idénticos")
		assert.Equal(t, missing.Body.Bytes(), foreign.Body.Bytes(), "los cuerpos deberían ser idénticos byte a byte")
	})
}

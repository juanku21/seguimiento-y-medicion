package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Pruebas unitarias obligatorias de las reglas de negocio RN-08 y RN-09 (spec.md, 001/US3): el
// registro y el inicio de sesión son las únicas rutas públicas y cualquier otra ruta que se cuelgue
// del grupo protegido que expone RegisterRoutes exige sesión (RN-08, FR-019); la acción protegida se
// atribuye al titular identificado por el claim sub que el middleware deja en el contexto (RN-09,
// FR-021). Son pruebas de la tabla de ruteo y del mecanismo de atribución, no de la validez real de
// la sesión (firma, vencimiento o fila de sessions): eso lo cubren las pruebas de integración de
// middleware_test.go (T055), así que estas no dependen de la base de datos. Los nombres llevan el
// prefijo "Rutas" para no chocar con TestUS3_AccesoConSesionVigente, TestUS3_RutasPublicas,
// TestUS3_SesionVencida ni TestUS3_SesionAdulterada, que T055 define en middleware_test.go.

// newRoutesTestRouter monta el router real de la feature con RegisterRoutes, sin base de datos (el
// manejador no llega a usar el servicio porque las rutas de sondeo fallan la validación de entrada
// antes de tocarlo), y devuelve también el grupo de rutas protegidas para colgar una ruta de prueba.
func newRoutesTestRouter(t *testing.T) (*gin.Engine, *gin.RouterGroup) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	protected := RegisterRoutes(router, NewHandler(nil))
	return router, protected
}

// Escenario: Rutas públicas (001/US3) — RN-08: registro e inicio de sesión son las únicas funciones
// públicas; toda otra ruta exige sesión.
func TestUS3_RutasSoloRegistroYLoginSonPublicas(t *testing.T) {
	router, protected := newRoutesTestRouter(t)

	// Ruta de sondeo colgada del grupo protegido: si se ejecuta, responde con un código distintivo
	// que ninguna otra ruta de la feature usa, para detectar si el middleware la dejó pasar.
	protected.GET("/solo-prueba-ruta-protegida", func(c *gin.Context) {
		c.Status(http.StatusTeapot)
	})

	t.Run("una ruta nueva del grupo protegido exige sesión", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/solo-prueba-ruta-protegida", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/v1/solo-prueba-ruta-protegida sin sesión devolvió %d; se esperaba %d (RN-08, FR-019)",
				rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("el registro sigue siendo público", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, registerPath, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("POST %s sin sesión respondió %d; no debería exigir sesión (RN-08)", registerPath, rec.Code)
		}
	})

	t.Run("el inicio de sesión sigue siendo público", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, loginPath, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("POST %s sin sesión respondió %d; no debería exigir sesión (RN-08)", loginPath, rec.Code)
		}
	})
}

// Escenario: Acceso con sesión vigente — atribución al titular (001/US3) — RN-09: toda acción
// protegida se atribuye a quien identifica el claim sub que el middleware deja en el contexto.
func TestUS3_RutasAtribucionPorClaimSub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	want := uuid.New()
	c.Set(contextKeySub, want)

	got, ok := userIDFromContext(c)
	if !ok || got != want {
		t.Errorf("userIDFromContext() = (%v, %v); se esperaba (%v, true) para atribuir la acción al titular identificado por sub (FR-021, RN-09)",
			got, ok, want)
	}
}

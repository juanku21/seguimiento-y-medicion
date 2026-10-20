package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
)

// Prueba de integración de la consulta previa CORS (T085). El router usa el middleware CORS que
// compone cmd/app con el origen permitido de backend/.env.test, y recibe la consulta previa que el
// navegador envía antes de cada operación de la API con cabeceras de sesión y JSON.

// preflightRequestHeaders son las cabeceras que el navegador anuncia en la consulta previa: el
// token de sesión y el cuerpo JSON.
const preflightRequestHeaders = "authorization,content-type"

// Escenario: Modificación exitosa desde el navegador del frontend (002/US5)
// La consulta previa del navegador desde el origen del frontend habilita cada método que usa la
// API, en particular PUT /api/v1/projects/{projectId} para modificar el proyecto (US5): la
// respuesta devuelve el origen permitido y lista el método pedido en Access-Control-Allow-Methods.
func TestUS5_ConsultaPreviaCORSPermiteMetodosDeLaAPI(t *testing.T) {
	_, cfg := testsupport.OpenDatabase(t)
	require.NotEmpty(t, cfg.CORSAllowedOrigin, "el .env.test debería definir CORS_ALLOWED_ORIGIN")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(cfg.CORSAllowedOrigin))

	// Cada subtest pide un método de las operaciones de la API sobre una ruta de proyecto.
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/api/v1/projects/00000000-0000-0000-0000-000000000000", nil)
			req.Header.Set("Origin", cfg.CORSAllowedOrigin)
			req.Header.Set("Access-Control-Request-Method", method)
			req.Header.Set("Access-Control-Request-Headers", preflightRequestHeaders)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Less(t, rec.Code, 300, "la consulta previa debería responder con éxito, se obtuvo %d", rec.Code)
			assert.Equal(t, cfg.CORSAllowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			allowed := strings.Split(rec.Header().Get("Access-Control-Allow-Methods"), ",")
			for i := range allowed {
				allowed[i] = strings.TrimSpace(allowed[i])
			}
			assert.Contains(t, allowed, method, "Access-Control-Allow-Methods debería incluir %s", method)
		})
	}
}

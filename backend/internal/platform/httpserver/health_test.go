package httpserver

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/database"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
)

// Pruebas de integración de GET /healthz, el endpoint de salud que consulta Render antes de
// enviarle tráfico al backend. El router registra la ruta sin el middleware de sesión, igual que
// cmd/app, porque el health check no envía token.

// healthRequest arma un router con GET /healthz sobre el pool indicado, le envía la solicitud sin
// cabecera Authorization y devuelve el código y el cuerpo decodificado.
func healthRequest(t *testing.T, db Pinger) (int, HealthResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/healthz", Health(db))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	var body HealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo debería ser JSON: %q", rec.Body.String())
	return rec.Code, body
}

// Con la base de test disponible, GET /healthz responde 200 con estado ok y sin exigir sesión.
func TestHealth_BaseDisponible(t *testing.T) {
	db, _ := testsupport.OpenDatabase(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	code, body := healthRequest(t, sqlDB)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body.Status)
}

// Con el pool de conexiones cerrado, la base no responde al ping y GET /healthz responde 503 con
// estado unavailable, para que Render no le envíe tráfico a esa instancia.
func TestHealth_BaseNoDisponible(t *testing.T) {
	_, cfg := testsupport.OpenDatabase(t)
	pool, err := sql.Open("pgx", database.DSN(cfg))
	require.NoError(t, err)
	require.NoError(t, pool.Close())

	code, body := healthRequest(t, pool)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "unavailable", body.Status)
}

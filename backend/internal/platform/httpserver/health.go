package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// healthPingTimeout acota la espera del ping a la base, para que el health check responda antes
// de que el proveedor lo dé por vencido.
const healthPingTimeout = 2 * time.Second

// Pinger es la parte del pool de conexiones que usa el endpoint de salud; la cumple *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthResponse es el cuerpo de GET /healthz.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Health devuelve el manejador de GET /healthz: hace ping a la base con un límite de
// healthPingTimeout y responde 200 si contesta o 503 si no, registrando el error sin exponerlo en
// la respuesta.
//
//	@Summary		Estado del servicio
//	@Description	Verifica que el backend esté en marcha y que la base de datos responda. Es público,
//	@Description	sin sesión, porque lo consulta el health check de la plataforma de despliegue.
//	@Tags			salud
//	@Produce		json
//	@Success		200	{object}	HealthResponse	"El backend y la base responden"
//	@Failure		503	{object}	HealthResponse	"La base de datos no responde"
//	@Router			/healthz [get]
func Health(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthPingTimeout)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			slog.Error("el health check no pudo contactar la base", "error", err)
			c.JSON(http.StatusServiceUnavailable, HealthResponse{Status: "unavailable"})
			return
		}
		c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
	}
}

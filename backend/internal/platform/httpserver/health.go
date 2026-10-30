package httpserver

import (
	"context"

	"github.com/gin-gonic/gin"
)

// Pinger es la parte del pool de conexiones que usa el endpoint de salud; la cumple *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// HealthResponse es el cuerpo de GET /healthz.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Health devuelve el manejador de GET /healthz.
func Health(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

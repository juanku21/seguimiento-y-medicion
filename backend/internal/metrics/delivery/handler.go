// Package delivery expone por HTTP las métricas de proyectos y sprints: DTOs, manejadores y rutas
// (Principio III).
package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/service"
)

// Handler agrupa los manejadores HTTP del módulo de métricas (firma mínima, pendiente de T015).
type Handler struct {
	metrics *service.MetricsService
}

// NewHandler crea el manejador sobre el servicio de métricas (firma mínima, pendiente de T015).
func NewHandler(metrics *service.MetricsService) *Handler {
	return &Handler{metrics: metrics}
}

// getSprintMetrics responde las métricas de un sprint (firma mínima, pendiente de T015).
func (h *Handler) getSprintMetrics(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

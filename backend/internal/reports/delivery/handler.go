// Package delivery expone por HTTP la generación de reportes: manejadores y rutas (Principio III).
package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/service"
)

// msgInternalError es el mensaje del 500, que no expone detalles internos.
const msgInternalError = "Ocurrió un error inesperado. Intentá de nuevo más tarde."

// Handler agrupa los manejadores HTTP del módulo de reportes sobre su servicio.
type Handler struct {
	reports *service.ReportService
}

// NewHandler crea el manejador sobre el servicio de reportes.
func NewHandler(reports *service.ReportService) *Handler {
	return &Handler{reports: reports}
}

// generateSprintReport responde por ahora el 500 genérico; el manejador definitivo llega después.
func (h *Handler) generateSprintReport(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

// generateProjectReport responde por ahora el 500 genérico; el manejador definitivo llega después.
func (h *Handler) generateProjectReport(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

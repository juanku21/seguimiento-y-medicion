// Package delivery expone por HTTP las métricas de proyectos y sprints: DTOs, manejadores y rutas
// (Principio III).
package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/service"
)

// Mensajes de las respuestas de error de las métricas. El 404 de sprint es el mismo cuerpo que el de
// la 004, para que un sprint inexistente, ajeno o mal formado sean indistinguibles (FR-042); el 500
// no expone detalles internos.
const (
	msgSprintNotFound = "El sprint no existe."
	msgInternalError  = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler agrupa los manejadores HTTP del módulo de métricas sobre el servicio de métricas.
type Handler struct {
	metrics *service.MetricsService
}

// NewHandler crea el manejador sobre el servicio de métricas.
func NewHandler(metrics *service.MetricsService) *Handler {
	return &Handler{metrics: metrics}
}

// getSprintMetrics godoc
//
// Los esquemas de error se anotan como delivery.ErrorResponse de auth/delivery, igual que en las
// demás features: swag los nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Métricas de un sprint
//	@Description	Devuelve las nueve métricas de un sprint del proyecto según su estado (US1, US4, US5).
//	@Description	Cerrado: Story Points, factor e historias salen de la instantánea de cierre (FR-013);
//	@Description	Story Points planificados y completados, horas estimadas y porcentaje de historias
//	@Description	completadas son definitive; horas reales y las dos desviaciones son provisional mientras
//	@Description	el plazo de gracia siga vigente y definitive después; los conteos de defectos son siempre
//	@Description	provisional (FR-019). Activo: datos y factor vigentes, las nueve partial (FR-014).
//	@Description	Planificado: solo Story Points planificados y horas estimadas, con finality null; las
//	@Description	otras siete son not_applicable (FR-015).
//	@Description	Cada métrica lleva un status explícito: available (incluido el 0), not_calculable si el
//	@Description	denominador es 0 y not_applicable si no aplica al estado del sprint; value es null salvo
//	@Description	con available. Horas y porcentajes se expresan con exactamente 2 decimales y los Story
//	@Description	Points como enteros. Un sprint inexistente, de otro proyecto o con identificador mal
//	@Description	formado responde el mismo 404, igual que un proyecto del que quien pide no es integrante
//	@Description	(FR-042).
//	@Tags			Métricas
//	@ID				getSprintMetrics
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Param			sprintId	path		string					true	"Identificador del sprint"		format(uuid)
//	@Success		200			{object}	SprintMetrics			"Métricas del sprint."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante (cuerpo de la 002), o el sprint no existe, es de otro proyecto o su identificador está mal formado (cuerpo de la 004)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/metrics/sprints/{sprintId} [get]
func (h *Handler) getSprintMetrics(c *gin.Context) {
	// RequireMembership ya validó el proyecto; si su identificador no es UUID es un error de
	// composición de la ruta y responde 500.
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "getSprintMetrics con un projectId inválido", "error", err)
		return
	}

	// Un sprintId que no es UUID responde como sprint inexistente, sin consultar.
	sprintID, err := uuid.Parse(c.Param("sprintId"))
	if err != nil {
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgSprintNotFound})
		return
	}

	// El servicio calcula las métricas; el sprint inexistente o ajeno se traduce a 404 y cualquier
	// otro error a un 500 genérico registrado en el log.
	metrics, warnings, err := h.metrics.SprintMetrics(c.Request.Context(), projectID, sprintID)
	if err != nil {
		if errors.Is(err, domain.ErrSprintNotFound) {
			c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgSprintNotFound})
			return
		}
		respondInternalError(c, "no se pudieron calcular las métricas del sprint",
			"projectId", projectID, "sprintId", sprintID, "error", err)
		return
	}

	c.JSON(http.StatusOK, newSprintMetrics(metrics, warnings))
}

// respondInternalError registra en el log el mensaje y los atributos recibidos y responde el 500
// genérico, sin exponer la causa a quien pide (Principio VII).
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

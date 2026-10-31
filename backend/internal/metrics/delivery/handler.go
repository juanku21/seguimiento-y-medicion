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
	projectID, ok := parseProjectID(c, "getSprintMetrics")
	if !ok {
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

	c.JSON(http.StatusOK, NewSprintMetrics(metrics, warnings))
}

// getActiveSprintMetrics godoc
//
//	@Summary		Métricas del sprint Activo
//	@Description	Devuelve las métricas del sprint Activo, calculadas con los datos y el factor vigentes
//	@Description	y todas marcadas partial (FR-014, US4). Si el proyecto no tiene sprint Activo responde
//	@Description	200 con activeSprint false y metrics null: no es un error ni un conjunto de ceros
//	@Description	(FR-016).
//	@Tags			Métricas
//	@ID				getActiveSprintMetrics
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string						true	"Identificador del proyecto"	format(uuid)
//	@Success		200			{object}	ActiveSprintMetricsResponse	"Métricas del sprint Activo o indicación de que no hay ninguno."
//	@Failure		401			{object}	delivery.ErrorResponse		"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse		"El proyecto no existe o quien pide no es integrante."
//	@Failure		500			{object}	delivery.ErrorResponse		"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/metrics/active-sprint [get]
func (h *Handler) getActiveSprintMetrics(c *gin.Context) {
	projectID, ok := parseProjectID(c, "getActiveSprintMetrics")
	if !ok {
		return
	}

	// Cualquier error del servicio se traduce a un 500 genérico registrado en el log.
	metrics, warnings, err := h.metrics.ActiveSprintMetrics(c.Request.Context(), projectID)
	if err != nil {
		respondInternalError(c, "no se pudieron calcular las métricas del sprint activo",
			"projectId", projectID, "error", err)
		return
	}

	// Sin sprint Activo se responde 200 con la indicación explícita, no un error.
	if metrics == nil {
		c.JSON(http.StatusOK, ActiveSprintMetricsResponse{ActiveSprint: false})
		return
	}
	response := NewSprintMetrics(*metrics, warnings)
	c.JSON(http.StatusOK, ActiveSprintMetricsResponse{ActiveSprint: true, Metrics: &response})
}

// parseProjectID obtiene el identificador del proyecto de la ruta. RequireMembership ya lo validó;
// si no es UUID es un error de composición de la ruta y responde 500.
func parseProjectID(c *gin.Context, handlerName string) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, handlerName+" con un projectId inválido", "error", err)
		return uuid.Nil, false
	}
	return projectID, true
}

// getProjectMetrics godoc
//
//	@Summary		Métricas del proyecto
//	@Description	Devuelve la velocidad del equipo con su serie (US2) y las métricas acumuladas (US3).
//	@Description	La velocidad promedia los Story
//	@Description	Points completados de los últimos 3 sprints Cerrados por momento real de cierre, o de
//	@Description	todos si hay menos; sin sprints Cerrados es not_calculable (FR-020, FR-021, FR-031).
//	@Description	Nunca lleva finality porque solo mira sprints Cerrados (RN-28). La serie trae un punto
//	@Description	por sprint Cerrado en orden cronológico, sin promediar ni recortar. Horas y porcentajes
//	@Description	se expresan con exactamente 2 decimales.
//	@Description	El bloque cumulative suma los sprints Cerrados y el Activo; los Planificados no entran.
//	@Description	Su finality califica al bloque entero: partial si incluye el Activo (includesActiveSprint
//	@Description	true) y definitive si no. Los porcentajes salen de los totales y no del promedio de los
//	@Description	de cada sprint (RN-11). effortOutsideSprintsHours es el esfuerzo sin sprint asociado
//	@Description	(FR-029) y effortInPlannedSprintsHours el anclado a sprints Planificados; ambos con
//	@Description	2 decimales y siempre presentes, 0 incluido. unestimatedBacklogStories cuenta las
//	@Description	historias sin estimar (FR-036) y, si es mayor que 0, se agrega la advertencia de
//	@Description	planificación unestimated_backlog_stories.
//	@Tags			Métricas
//	@ID				getProjectMetrics
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Success		200			{object}	ProjectMetricsResponse	"Métricas del proyecto."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/metrics [get]
func (h *Handler) getProjectMetrics(c *gin.Context) {
	projectID, ok := parseProjectID(c, "getProjectMetrics")
	if !ok {
		return
	}

	// Cualquier error del servicio se traduce a un 500 genérico registrado en el log.
	metrics, err := h.metrics.ProjectMetrics(c.Request.Context(), projectID)
	if err != nil {
		respondInternalError(c, "no se pudieron calcular las métricas del proyecto",
			"projectId", projectID, "error", err)
		return
	}

	c.JSON(http.StatusOK, NewProjectMetrics(projectID, metrics))
}

// getProjectSeries godoc
//
//	@Summary		Series del proyecto por sprint
//	@Description	Devuelve las seis series por sprint que grafica el dashboard, en una única respuesta
//	@Description	(US2, FR-022): Story Points planificados y completados, horas estimadas y reales, y
//	@Description	defectos detectados y resueltos. Las seis contienen los mismos sprints, en orden
//	@Description	cronológico ascendente por fecha de inicio: los Cerrados y el Activo si existe; los
//	@Description	Planificados no aparecen. Cada punto identifica su sprint y conserva el finality de la
//	@Description	métrica en ese sprint (partial en el Activo; definitive o provisional en un Cerrado).
//	@Description	Un 0 es un dato real (status available). Sin sprints Cerrados ni Activo, las seis
//	@Description	series vienen vacías.
//	@Tags			Métricas
//	@ID				getProjectSeries
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Success		200			{object}	ProjectSeriesResponse	"Series del proyecto."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/metrics/series [get]
func (h *Handler) getProjectSeries(c *gin.Context) {
	projectID, ok := parseProjectID(c, "getProjectSeries")
	if !ok {
		return
	}

	// Cualquier error del servicio se traduce a un 500 genérico registrado en el log.
	series, warnings, err := h.metrics.ProjectSeries(c.Request.Context(), projectID)
	if err != nil {
		respondInternalError(c, "no se pudieron calcular las series del proyecto",
			"projectId", projectID, "error", err)
		return
	}

	c.JSON(http.StatusOK, newProjectSeries(series, warnings))
}

// respondInternalError registra en el log el mensaje y los atributos recibidos y responde el 500
// genérico, sin exponer la causa a quien pide (Principio VII).
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

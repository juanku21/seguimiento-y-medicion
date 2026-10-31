// Package delivery expone por HTTP la generación de reportes: manejadores y rutas (Principio III).
package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/service"
)

// Mensajes de las respuestas de error. El 404 de sprint es el mismo cuerpo que el de la 004, para que
// un sprint inexistente, ajeno o mal formado sean indistinguibles; el 500 no expone detalles internos.
const (
	errMsgSprintNotFound   = "El sprint no existe."
	errMsgSprintNotStarted = "El sprint todavía no arrancó: no tiene nada que reportar."
	errMsgInternal         = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler agrupa los manejadores HTTP del módulo de reportes sobre su servicio.
type Handler struct {
	reports *service.ReportService
}

// NewHandler crea el manejador sobre el servicio de reportes.
func NewHandler(reports *service.ReportService) *Handler {
	return &Handler{reports: reports}
}

// generateSprintReport godoc
//
// Los esquemas de error se anotan como delivery.ErrorResponse de auth/delivery, igual que en las
// demás features: swag los nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Generar el reporte de un sprint
//	@Description	Arma el reporte de un sprint Cerrado o Activo y registra la generación (US1, FR-001).
//	@Description	El reporte del Activo lleva partial true (FR-005). Un sprint Planificado se rechaza con
//	@Description	409 (FR-004). Las seis secciones (encabezado, historias, estimaciones, esfuerzo,
//	@Description	métricas y defectos) están siempre presentes y cada una informa su estado. El documento
//	@Description	no se guarda: solo queda el registro de quién lo generó, cuándo y con qué alcance. No
//	@Description	lee cuerpo.
//	@Tags			Reportes
//	@ID				generateSprintReport
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Param			sprintId	path		string					true	"Identificador del sprint"
//	@Success		200			{object}	Report					"Reporte del sprint."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante (cuerpo de la 002), o el sprint no existe, es de otro proyecto o su identificador está mal formado (cuerpo de la 004). No deja registro."
//	@Failure		409			{object}	delivery.ErrorResponse	"El sprint está Planificado. No deja registro."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/reports/sprints/{sprintId} [post]
func (h *Handler) generateSprintReport(c *gin.Context) {
	// El proyecto lo valida el middleware de membresía; un identificador ilegible es un error interno.
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "generateSprintReport con un projectId inválido", "error", err)
		return
	}

	// Un sprintId que no es UUID responde como sprint inexistente, sin consultar.
	sprintID, err := uuid.Parse(c.Param("sprintId"))
	if err != nil {
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: errMsgSprintNotFound})
		return
	}

	// La sesión la exige el middleware; sin titular no se puede registrar la generación.
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "generateSprintReport sin titular de la sesión")
		return
	}

	// El sprint inexistente o planificado se traduce a su código; cualquier otro error, a un 500.
	report, err := h.reports.SprintReport(c.Request.Context(), projectID, sprintID, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSprintNotFound):
			c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: errMsgSprintNotFound})
		case errors.Is(err, domain.ErrSprintNotStarted):
			c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: errMsgSprintNotStarted})
		default:
			respondInternalError(c, "no se pudo generar el reporte del sprint",
				"projectId", projectID, "sprintId", sprintID, "error", err)
		}
		return
	}

	c.JSON(http.StatusOK, NewReport(report))
}

// respondInternalError registra el error y responde el 500 genérico sin detalles internos.
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: errMsgInternal})
}

// generateProjectReport godoc
//
//	@Summary		Generar el reporte del proyecto
//	@Description	Arma el reporte del proyecto completo y registra la generación (US3, FR-002). Abarca los
//	@Description	sprints Cerrados y el Activo si existe, en orden cronológico, y nunca los Planificados
//	@Description	(FR-006, FR-007, FR-011). Las historias se agrupan por sprint y terminan con el grupo de
//	@Description	historias no planificadas, que siempre está (FR-036). Lleva partial true si incluye al
//	@Description	Activo. Un proyecto sin sprints reportables responde 200 con las secciones en empty, no
//	@Description	con un error. No lee cuerpo.
//	@Tags			Reportes
//	@ID				generateProjectReport
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string					true	"Identificador del proyecto"	format(uuid)
//	@Success		200			{object}	Report					"Reporte del proyecto."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El proyecto no existe o quien pide no es integrante (cuerpo de la 002)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/reports/project [post]
func (h *Handler) generateProjectReport(c *gin.Context) {
	// El proyecto lo valida el middleware de membresía; un identificador ilegible es un error interno.
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "generateProjectReport con un projectId inválido", "error", err)
		return
	}

	// La sesión la exige el middleware; sin titular no se puede registrar la generación.
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "generateProjectReport sin titular de la sesión")
		return
	}

	// Cualquier error del servicio se traduce al 500 genérico: el proyecto sin sprints no es un error.
	report, err := h.reports.ProjectReport(c.Request.Context(), projectID, userID)
	if err != nil {
		respondInternalError(c, "no se pudo generar el reporte del proyecto", "projectId", projectID, "error", err)
		return
	}

	c.JSON(http.StatusOK, NewReport(report))
}

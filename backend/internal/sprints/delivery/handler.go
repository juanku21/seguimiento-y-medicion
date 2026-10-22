// Package delivery expone por HTTP los sprints de un proyecto.
package delivery

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Mensajes de las respuestas de error de los sprints, en español y dirigidos a la persona. Los
// textos del 400, del 401 y del 409 los fija contracts/openapi.yaml; el del cuerpo mal formado y el
// del 500 son los mismos que usan la 001, la 002 y la 003, y el 500 no expone detalles internos. El
// mensaje de superposición nombra el sprint en conflicto con el formato del contrato.
const (
	msgInvalidFields   = "Hay campos inválidos en la solicitud."
	msgInvalidBody     = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired = "Se requiere iniciar sesión."
	msgSprintsReadOnly = "El proyecto está finalizado y sus sprints son de solo lectura."
	msgSprintOverlap   = "El período se superpone con el del sprint «%s»."
	msgInternalError   = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler agrupa los manejadores HTTP del módulo de sprints sobre el servicio de sprints.
type Handler struct {
	sprints *service.SprintService
}

// NewHandler compone los manejadores de sprints sobre el servicio recibido.
func NewHandler(sprints *service.SprintService) *Handler {
	return &Handler{sprints: sprints}
}

// createSprint godoc
//
// Los esquemas de error se anotan como delivery.X de auth/delivery, igual que en la 002 y la 003:
// swag los nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Crea un sprint en el proyecto
//	@Description	Crea un sprint a partir de nombre, fecha de inicio, fecha de fin y Sprint Goal (FR-001).
//	@Description	El sprint nace planned, sin historias y sin instantánea (FR-006), atribuido al titular
//	@Description	de la sesión (FR-009). El nombre no es único. La fecha de fin debe ser estrictamente
//	@Description	posterior a la de inicio (FR-004). Se rechaza con 409 si el período se superpone con el
//	@Description	de cualquier otro sprint del proyecto, en cualquier estado; compartir solo el día de
//	@Description	borde no es superposición (FR-005). También se rechaza con 409 si el proyecto está
//	@Description	finished, porque sus sprints son de solo lectura. Crear un sprint no cambia el estado
//	@Description	del proyecto (FR-049). Cualquier integrante puede crear sprints (RN-01).
//	@Tags			Sprints
//	@ID				createSprint
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			request		body		SprintInput							true	"Datos del sprint nuevo"
//	@Success		201			{object}	SprintDetail						"Sprint creado en estado planned. El cuerpo es el detalle del sprint."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Entrada inválida. Indica todos los campos a corregir y por qué, y no crea nada (FR-011). Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	SprintConflictResponse				"El período se superpone con el de otro sprint del proyecto, que se nombra en el cuerpo (FR-005). Si el proyecto está finished, la respuesta es ErrorResponse con el mensaje \"El proyecto está finalizado y sus sprints son de solo lectura.\""
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/sprints [post]
func (h *Handler) createSprint(c *gin.Context) {
	// La identidad del autor sale solo de la sesión verificada por el middleware y el proyecto, del
	// parámetro que RequireMembership ya validó como UUID de un proyecto del que es integrante.
	actorID, projectID, ok := sessionAndProject(c, "createSprint")
	if !ok {
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio; ante cualquier error ya se
	// respondió el 400 y no se crea nada (FR-011).
	input, ok := bindSprintInput(c)
	if !ok {
		return
	}

	// Se crea el sprint y se traducen los errores de dominio a 409; cualquier otro error es 500
	// genérico, registrado en el log.
	view, err := h.sprints.Create(c.Request.Context(), projectID, actorID, input)
	if err != nil {
		respondServiceError(c, err, "no se pudo crear el sprint")
		return
	}

	c.JSON(http.StatusCreated, newSprintDetail(view))
}

// sessionAndProject devuelve el titular de la sesión y el proyecto de la ruta. Sin sesión responde el
// 401 del contrato. Un projectId que no sea UUID solo es posible si la ruta se registró fuera del
// subgrupo protegido por RequireMembership, así que es un error de composición: responde 500
// genérico y lo registra con el nombre del manejador.
func sessionAndProject(c *gin.Context, handlerName string) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return uuid.Nil, uuid.Nil, false
	}
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, handlerName+" sin un projectId válido en la ruta", "error", err)
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, projectID, true
}

// bindSprintInput decodifica el cuerpo JSON y lo valida con ValidateSprintInput. Un cuerpo que no es
// JSON válido o está vacío responde 400 con ErrorResponse, y los campos inválidos, 400 con
// ValidationErrorResponse listando todos a la vez (FR-011). Si ya respondió, devuelve false y el
// manejador solo tiene que cortar.
func bindSprintInput(c *gin.Context) (domain.SprintInput, bool) {
	var req SprintInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return domain.SprintInput{}, false
	}
	input, fields := ValidateSprintInput(req)
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return domain.SprintInput{}, false
	}
	return input, true
}

// respondServiceError traduce los errores de dominio del servicio de sprints a la respuesta HTTP del
// contrato: la superposición de períodos es 409 con el sprint en conflicto y el proyecto finalizado,
// 409 de solo lectura. Lo que no es un error de dominio conocido se registra en el log con el mensaje
// recibido y se responde 500 genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var overlap *domain.SprintOverlapError
	switch {
	case errors.As(err, &overlap):
		message := fmt.Sprintf(msgSprintOverlap, overlap.Sprint.Name)
		c.JSON(http.StatusConflict, newSprintConflictResponse(message, overlap.Sprint))
	case errors.Is(err, domain.ErrSprintsReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgSprintsReadOnly})
	default:
		respondInternalError(c, logMessage, "error", err)
	}
}

// respondInternalError registra en el log el mensaje y los atributos recibidos y responde el 500
// genérico, sin exponer la causa a quien pide (Principio VII).
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

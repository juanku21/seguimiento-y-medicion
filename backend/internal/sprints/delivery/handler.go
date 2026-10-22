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
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/service"
)

// Mensajes de las respuestas de error de los sprints, en español y dirigidos a la persona. Los
// textos del 400, del 401 y del 409 los fija contracts/openapi.yaml; el del cuerpo mal formado y el
// del 500 son los mismos que usan la 001, la 002 y la 003, y el 500 no expone detalles internos. El
// mensaje de superposición nombra el sprint en conflicto con el formato del contrato. El 404 de
// sprint inexistente lo comparten el middleware (sprint inexistente, ajeno o identificador mal
// formado) y los manejadores (FR-057, FR-058); los del 404 de historia, del sprint cerrado, del lote
// no admitido y de la historia Completada son los de la asignación y la baja de historias (US2).
const (
	msgInvalidFields         = "Hay campos inválidos en la solicitud."
	msgInvalidBody           = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired       = "Se requiere iniciar sesión."
	msgSprintNotFound        = "El sprint no existe."
	msgStoryNotFound         = "La historia no existe."
	msgSprintsReadOnly       = "El proyecto está finalizado y sus sprints son de solo lectura."
	msgSprintOverlap         = "El período se superpone con el del sprint «%s»."
	msgSprintClosed          = "El sprint está cerrado y no admite cambios."
	msgStoriesNotAdmitted    = "Algunas historias no se pueden comprometer en el sprint; no se asignó ninguna."
	msgCompletedStoryRemoval = "No se puede quitar una historia Completada. Primero devolvela a En progreso."
	msgInternalError         = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
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

// getSprint godoc
//
//	@Summary		Consulta el detalle de un sprint
//	@Description	Devuelve los datos del sprint, su estado y sus historias comprometidas con sus datos
//	@Description	vigentes (FR-053). Cualquier integrante del proyecto puede consultarlo, también en un
//	@Description	proyecto finished. Un sprint inexistente, eliminado o de un proyecto del que quien pide
//	@Description	no es integrante responde el mismo 404 (FR-058).
//	@Tags			Sprints
//	@ID				getSprint
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sprintId	path		string					true	"Identificador del sprint"	format(uuid)
//	@Success		200			{object}	SprintDetail			"Detalle del sprint."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El sprint no existe, fue eliminado o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/sprints/{sprintId} [get]
func (h *Handler) getSprint(c *gin.Context) {
	// RequireSprintAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := sprintAccessFromContext(c)
	if !ok {
		respondInternalError(c, "getSprint sin acceso al sprint en el contexto")
		return
	}

	// Se arma el detalle del sprint; cualquier error del servicio se traduce con respondServiceError.
	view, err := h.sprints.Detail(c.Request.Context(), access)
	if err != nil {
		respondServiceError(c, err, "no se pudo leer el detalle del sprint")
		return
	}

	c.JSON(http.StatusOK, newSprintDetail(view))
}

// assignSprintStories godoc
//
//	@Summary		Compromete historias del backlog en el sprint
//	@Description	Asigna una o más historias del backlog del mismo proyecto a un sprint planned o active
//	@Description	(FR-017). La operación es atómica: si alguna historia no es admisible, ninguna se
//	@Description	asigna (FR-023). Cada historia debe estar lista para planificar (Story Points y al menos
//	@Description	un criterio), no estar completed y no estar comprometida en otro sprint abierto (FR-018
//	@Description	a FR-020); si alguna falla, el 409 lista todas las historias con problemas y sus
//	@Description	motivos. Una historia inexistente o de otro proyecto responde 404 de historia
//	@Description	inexistente. Cualquier integrante puede asignar historias (RN-01).
//	@Tags			Sprints
//	@ID				assignSprintStories
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sprintId	path		string								true	"Identificador del sprint"	format(uuid)
//	@Param			request		body		StoryIDsInput						true	"Historias a comprometer, entre 1 y 100 y sin repetidos"
//	@Success		200			{object}	SprintDetail						"Historias comprometidas. El cuerpo es el detalle actualizado del sprint."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Lista de historias vacía, con más de 100, con identificadores inválidos o repetidos; señala storyIds o storyIds[i]. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El sprint no existe o es ajeno (\"El sprint no existe.\"), o alguna historia no existe o es de otro proyecto (\"La historia no existe.\")."
//	@Failure		409			{object}	StoriesNotAdmittedResponse			"Historias no admisibles, con un problema por historia (FR-018 a FR-020, FR-023). Si el proyecto está finished o el sprint closed, la respuesta es ErrorResponse con el mensaje \"El proyecto está finalizado y sus sprints son de solo lectura.\" o \"El sprint está cerrado y no admite cambios.\""
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/sprints/{sprintId}/stories [post]
func (h *Handler) assignSprintStories(c *gin.Context) {
	// RequireSprintAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := sprintAccessFromContext(c)
	if !ok {
		respondInternalError(c, "assignSprintStories sin acceso al sprint en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y una lista inválida, 400 con los campos señalados, sin asignar nada.
	var req StoryIDsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	storyIDs, fields := ValidateStoryIDsInput(req)
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se compromete el lote y se traducen los rechazos de dominio con respondServiceError.
	view, err := h.sprints.AssignStories(c.Request.Context(), access, storyIDs)
	if err != nil {
		respondServiceError(c, err, "no se pudieron asignar las historias al sprint")
		return
	}

	c.JSON(http.StatusOK, newSprintDetail(view))
}

// removeSprintStories godoc
//
//	@Summary		Quita historias del sprint
//	@Description	Quita una o más historias de un sprint planned o active y las devuelve al backlog como
//	@Description	pending y sin sprint (FR-021, FR-022). La operación es atómica (FR-023). Se rechaza con
//	@Description	409 si el sprint está closed (FR-024) o si alguna historia está completed: primero hay
//	@Description	que devolverla a En progreso (FR-025). Una historia que no está comprometida en este
//	@Description	sprint responde 404 de historia inexistente.
//	@Tags			Sprints
//	@ID				removeSprintStories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sprintId	path		string								true	"Identificador del sprint"	format(uuid)
//	@Param			storyId		query		[]string							true	"Identificador de una historia a quitar; se repite para quitar varias (entre 1 y 100, sin repetidos)"	collectionFormat(multi)
//	@Success		200			{object}	SprintDetail						"Historias quitadas. El cuerpo es el detalle actualizado del sprint."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Sin storyId, con más de 100, con identificadores inválidos o repetidos; señala storyId."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El sprint no existe o es ajeno (\"El sprint no existe.\"), o alguna historia no está en este sprint (\"La historia no existe.\")."
//	@Failure		409			{object}	delivery.ErrorResponse				"Proyecto finished, sprint closed o historia completed (\"No se puede quitar una historia Completada. Primero devolvela a En progreso.\")."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/sprints/{sprintId}/stories [delete]
func (h *Handler) removeSprintStories(c *gin.Context) {
	// RequireSprintAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := sprintAccessFromContext(c)
	if !ok {
		respondInternalError(c, "removeSprintStories sin acceso al sprint en el contexto")
		return
	}

	// Los identificadores llegan como parámetros storyId repetidos y se validan antes de llegar al
	// servicio; una lista inválida responde 400 con el campo storyId, sin quitar nada.
	storyIDs, fields := ValidateStoryIDParams(c.QueryArray(storyIDsQueryField))
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se quita el lote y se traducen los rechazos de dominio con respondServiceError.
	view, err := h.sprints.RemoveStories(c.Request.Context(), access, storyIDs)
	if err != nil {
		respondServiceError(c, err, "no se pudieron quitar las historias del sprint")
		return
	}

	c.JSON(http.StatusOK, newSprintDetail(view))
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
// contrato: la superposición de períodos es 409 con el sprint en conflicto, el lote no admitido es
// 409 con un problema por historia, y el proyecto finalizado, el sprint cerrado y la historia
// Completada que se quiere quitar son 409 con su mensaje. El sprint que desapareció entre el
// middleware y el servicio y la historia inexistente, ajena o fuera del sprint son 404. Lo que no es
// un error de dominio conocido se registra en el log con el mensaje recibido y se responde 500
// genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var overlap *domain.SprintOverlapError
	var notAdmitted *domain.StoriesNotAdmittedError
	switch {
	case errors.As(err, &overlap):
		message := fmt.Sprintf(msgSprintOverlap, overlap.Sprint.Name)
		c.JSON(http.StatusConflict, newSprintConflictResponse(message, overlap.Sprint))
	case errors.As(err, &notAdmitted):
		c.JSON(http.StatusConflict, newStoriesNotAdmittedResponse(msgStoriesNotAdmitted, notAdmitted.Problems))
	case errors.Is(err, domain.ErrSprintsReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgSprintsReadOnly})
	case errors.Is(err, domain.ErrSprintClosed):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgSprintClosed})
	case errors.Is(err, domain.ErrCompletedStoryRemoval):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgCompletedStoryRemoval})
	case errors.Is(err, domain.ErrSprintNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgSprintNotFound})
	case errors.Is(err, backlogdomain.ErrStoryNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgStoryNotFound})
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

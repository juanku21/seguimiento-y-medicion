// Package delivery expone por HTTP el backlog de un proyecto.
package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/service"
)

// Mensajes de las respuestas de error del backlog, en español y dirigidos a la persona. Los textos
// del 400, del 401 y del 409 los fija contracts/openapi.yaml; el del cuerpo mal formado y el del 500
// son los mismos que usan la 001 y la 002, y el 500 no expone detalles internos.
const (
	msgInvalidFields   = "Hay campos inválidos en la solicitud."
	msgInvalidBody     = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired = "Se requiere iniciar sesión."
	msgBacklogReadOnly = "El proyecto está finalizado y su backlog es de solo lectura."
	msgInternalError   = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler reúne los manejadores HTTP del backlog sobre el servicio de historias.
type Handler struct {
	stories *service.StoryService
}

// NewHandler crea los manejadores del backlog sobre el servicio de historias recibido.
func NewHandler(stories *service.StoryService) *Handler {
	return &Handler{stories: stories}
}

// createStory godoc
//
// Los esquemas de error se anotan como delivery.X de auth/delivery, igual que en la 002: swag los
// nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Crea una historia en el backlog del proyecto
//	@Description	Crea una historia a partir de título, descripción opcional, prioridad, Story Points
//	@Description	opcionales y criterios de aceptación opcionales (FR-001). La historia nace en estado
//	@Description	pending (FR-008), atribuida al titular de la sesión (FR-011, FR-049). Si no se indican
//	@Description	Story Points queda "sin estimar" (storyPoints: null); el 0 es una estimación válida y
//	@Description	distinta de "sin estimar" (FR-015). El título no es único dentro del backlog (FR-012).
//	@Description	La creación es atómica: si algún dato es inválido no se crea la historia ni ninguno de
//	@Description	sus criterios (FR-013). Se acepta en proyectos planned e in_progress y se rechaza en
//	@Description	proyectos finished (FR-048). Cualquier integrante puede crear historias (RN-01, RN-02).
//	@Tags			Backlog
//	@ID				createStory
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			request		body		StoryInput							true	"Datos de la historia nueva"
//	@Success		201			{object}	StoryDetail							"Historia creada en estado pending."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Entrada inválida. Indica qué campo corregir y por qué, y no crea nada (FR-013). Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-045)."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo (FR-047, SC-006)."
//	@Failure		409			{object}	delivery.ErrorResponse				"El proyecto está finished y su backlog es de solo lectura (FR-048, RN-15)."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/stories [post]
func (h *Handler) createStory(c *gin.Context) {
	// La identidad del autor sale solo de la sesión verificada por el middleware y el proyecto, del
	// parámetro que RequireMembership ya validó como UUID de un proyecto del que es integrante.
	actorID, projectID, ok := sessionAndProject(c, "createStory")
	if !ok {
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio; ante cualquier error ya se
	// respondió el 400 y no se crea nada (FR-013).
	input, ok := bindStoryInput(c)
	if !ok {
		return
	}

	// Se crea la historia y se traducen los errores del servicio: el proyecto finalizado es 409 y
	// cualquier otro error es 500 genérico, registrado en el log.
	view, err := h.stories.Create(c.Request.Context(), projectID, actorID, input)
	if err != nil {
		respondServiceError(c, err, "no se pudo crear la historia")
		return
	}

	c.JSON(http.StatusCreated, newStoryDetail(view))
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
		slog.ErrorContext(c.Request.Context(), handlerName+" sin un projectId válido en la ruta", "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, projectID, true
}

// bindStoryInput decodifica el cuerpo como StoryInput y lo valida con ValidateStoryInput. Un cuerpo
// que no es JSON válido responde 400 con ErrorResponse, y los campos inválidos, 400 con
// ValidationErrorResponse listando todos a la vez (FR-013). Si todo es válido, mapea los datos
// normalizados a la entrada del servicio campo a campo.
func bindStoryInput(c *gin.Context) (service.CreateStoryInput, bool) {
	var req StoryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return service.CreateStoryInput{}, false
	}
	story, fields := ValidateStoryInput(req)
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return service.CreateStoryInput{}, false
	}
	return service.CreateStoryInput{
		Title:              story.Title,
		Description:        story.Description,
		Priority:           story.Priority,
		StoryPoints:        story.StoryPoints,
		AcceptanceCriteria: story.AcceptanceCriteria,
	}, true
}

// respondServiceError traduce los errores de dominio del servicio de historias a la respuesta HTTP del
// contrato. Lo que no es un error de dominio conocido se registra en el log con el mensaje recibido y
// se responde 500 genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	switch {
	case errors.Is(err, domain.ErrBacklogReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgBacklogReadOnly})
	default:
		slog.ErrorContext(c.Request.Context(), logMessage, "error", err)
		c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
	}
}

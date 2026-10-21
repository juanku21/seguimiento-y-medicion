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
// del 400, del 401, del 404 y del 409 los fija contracts/openapi.yaml; el del cuerpo mal formado y
// el del 500 son los mismos que usan la 001 y la 002, y el 500 no expone detalles internos.
const (
	msgInvalidFields   = "Hay campos inválidos en la solicitud."
	msgInvalidBody     = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired = "Se requiere iniciar sesión."
	msgStoryNotFound   = "La historia no existe."
	msgBacklogReadOnly = "El proyecto está finalizado y su backlog es de solo lectura."
	msgStoryCompleted  = "La historia está completada y no se puede modificar ni reestimar."
	msgStoryCommitted  = "La historia está comprometida en un sprint abierto; primero hay que quitarla del sprint."
	msgDeletionBlocked = "La historia tiene historial asociado y no se puede eliminar."
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

// getStory godoc
//
//	@Summary		Consulta el detalle de una historia
//	@Description	Devuelve todos los datos de la historia: criterios de aceptación en el orden en que se
//	@Description	cargaron, si está lista para planificar y qué le falta, quién la creó, cuándo se creó y
//	@Description	se actualizó, y si está o estuvo asignada a un sprint (FR-039). Sin criterios, la lista
//	@Description	viene vacía y no es un error (FR-040). Se puede consultar también en proyectos finished
//	@Description	(FR-048).
//	@Tags			Backlog
//	@ID				getStory
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string					true	"Identificador propio, estable, inmutable y no secuencial de la historia (FR-009)"	format(uuid)
//	@Success		200		{object}	StoryDetail				"Detalle de la historia."
//	@Failure		401		{object}	delivery.ErrorResponse	"Se requiere iniciar sesión (FR-045)."
//	@Failure		404		{object}	delivery.ErrorResponse	"La historia no existe, fue eliminada o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo (FR-044, FR-047, SC-006)."
//	@Failure		500		{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId} [get]
func (h *Handler) getStory(c *gin.Context) {
	// RequireStoryAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "getStory sin acceso a la historia en el contexto")
		return
	}

	// Se arma el detalle de la historia; cualquier error del servicio se traduce con
	// respondServiceError.
	view, err := h.stories.Get(c.Request.Context(), access.StoryID)
	if err != nil {
		respondServiceError(c, err, "no se pudo leer el detalle de la historia")
		return
	}

	c.JSON(http.StatusOK, newStoryDetail(view))
}

// estimateStory godoc
//
//	@Summary		Estima una historia en Story Points
//	@Description	Asigna a la historia un valor de la escala (0, 1, 2, 3, 5, 8, 13, 20, 40, 100) o la
//	@Description	devuelve a "sin estimar" con null (FR-014, FR-016, FR-018). El campo es obligatorio. El
//	@Description	valor nuevo reemplaza al anterior y actualiza el momento de última actualización
//	@Description	(FR-017). Todo valor fuera de la escala (intermedio, negativo o decimal) se rechaza con
//	@Description	400 indicando la escala, sin alterar el valor vigente (FR-019). Se rechaza con 409 si la
//	@Description	historia está completed (FR-030), si el proyecto está finished (FR-048) o si se intenta
//	@Description	devolver a "sin estimar" una historia comprometida en un sprint abierto (FR-018, RN-18).
//	@Description	Cambiar un valor de la escala por otro sí se admite en una historia comprometida.
//	@Tags			Backlog
//	@ID				estimateStory
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string								true	"Identificador propio, estable, inmutable y no secuencial de la historia (FR-009)"	format(uuid)
//	@Param			request	body		StoryPointsInput					true	"Story Points de la escala, o null para dejar la historia sin estimar"
//	@Success		200		{object}	StoryDetail							"Historia estimada."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Campo ausente o valor fuera de la escala (FR-019); el valor vigente no cambia. El campo storyPoints lleva el mensaje \"Los valores admitidos son 0, 1, 2, 3, 5, 8, 13, 20, 40 y 100.\" si el valor está fuera de la escala, o \"Los Story Points son obligatorios; para dejar la historia sin estimar se envía null.\" si falta. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-045)."
//	@Failure		404		{object}	delivery.ErrorResponse				"La historia no existe, fue eliminada o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo (FR-044, FR-047, SC-006)."
//	@Failure		409		{object}	delivery.ErrorResponse				"La historia está completed, el proyecto está finished, o se intenta devolver a \"sin estimar\" una historia comprometida en un sprint abierto; la estimación no cambia (FR-018, FR-030, FR-048, RN-18)."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/story-points [put]
func (h *Handler) estimateStory(c *gin.Context) {
	// RequireStoryAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "estimateStory sin acceso a la historia en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y un campo ausente o fuera de la escala, 400 con el campo señalado (FR-019).
	storyPoints, ok := bindAndValidate(c, ValidateStoryPointsInput)
	if !ok {
		return
	}

	// Se registra la estimación y se traducen los errores de dominio a 404 o 409 con
	// respondServiceError; cualquier otro error es 500 genérico, registrado en el log.
	view, err := h.stories.Estimate(c.Request.Context(), access, storyPoints)
	if err != nil {
		respondServiceError(c, err, "no se pudo estimar la historia")
		return
	}

	c.JSON(http.StatusOK, newStoryDetail(view))
}

// updateStory godoc
//
//	@Summary		Refina una historia
//	@Description	Modifica el título, la descripción, la prioridad o los criterios de aceptación de la
//	@Description	historia. Solo se aplican los campos incluidos: un campo ausente deja su valor vigente y
//	@Description	un campo enviado vacío borra el valor cuando ese campo admite estar vacío (FR-020). La
//	@Description	descripción en null, "" o solo espacios se borra; acceptanceCriteria en [] deja la
//	@Description	historia sin criterios y la lista enviada reemplaza completa a la anterior, lo que
//	@Description	resuelve agregar, quitar y reordenar (FR-022); title y priority no admiten quedar vacíos.
//	@Description	Las validaciones son las mismas que en la creación (FR-021). La modificación es atómica:
//	@Description	si algún campo es inválido no se aplica ninguno (FR-023). No altera el identificador, el
//	@Description	estado, el proyecto, el autor ni el momento de creación (FR-024). Dos modificaciones
//	@Description	simultáneas de campos distintos conservan los dos cambios; sobre el mismo campo prevalece
//	@Description	la última, sin aviso, y la lista de criterios cuenta como un único campo (FR-025). Un
//	@Description	cuerpo con status se rechaza con 400 (FR-027) y uno con storyPoints también, porque la
//	@Description	estimación se cambia con PUT /api/v1/stories/{storyId}/story-points. Se rechaza con 409
//	@Description	si la historia está completed (FR-030), si el proyecto está finished (FR-048) o si se
//	@Description	intenta dejar sin criterios a una historia comprometida en un sprint abierto (FR-022,
//	@Description	RN-18).
//	@Tags			Backlog
//	@ID				updateStory
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string								true	"Identificador propio, estable, inmutable y no secuencial de la historia (FR-009)"	format(uuid)
//	@Param			request	body		StoryPatch							true	"Campos a modificar; debe incluir al menos uno"
//	@Success		200		{object}	StoryDetail							"Historia actualizada."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Entrada inválida, cuerpo sin campos a modificar (field body), o intento de cambiar status o storyPoints por esta vía. No se aplica ningún cambio (FR-023). Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-045)."
//	@Failure		404		{object}	delivery.ErrorResponse				"La historia no existe, fue eliminada o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo (FR-044, FR-047, SC-006)."
//	@Failure		409		{object}	delivery.ErrorResponse				"La historia está completed, el proyecto está finished, o se intenta dejar sin criterios a una historia comprometida en un sprint abierto; la historia no cambia (FR-022, FR-030, FR-048, RN-18)."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId} [patch]
func (h *Handler) updateStory(c *gin.Context) {
	// RequireStoryAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "updateStory sin acceso a la historia en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y cualquier campo inválido, 400 con todos los campos señalados a la vez, sin
	// aplicar ningún cambio (FR-023).
	changes, ok := bindAndValidate(c, ValidateStoryPatch)
	if !ok {
		return
	}

	// Se aplican los cambios y se traducen los errores de dominio a 404 o 409 con
	// respondServiceError; cualquier otro error es 500 genérico, registrado en el log.
	view, err := h.stories.Update(c.Request.Context(), access, changes)
	if err != nil {
		respondServiceError(c, err, "no se pudo modificar la historia")
		return
	}

	c.JSON(http.StatusOK, newStoryDetail(view))
}

// deleteStory godoc
//
//	@Summary		Elimina una historia que nunca se trabajó
//	@Description	Elimina la historia de forma definitiva, junto con sus criterios de aceptación,
//	@Description	únicamente si nunca fue asignada a un sprint, no tiene esfuerzo registrado, no tiene
//	@Description	defectos asociados y no tiene sesiones de estimación por Planning Poker, Finalizadas o
//	@Description	Canceladas (FR-041, RN-16). Cualquier integrante puede eliminarla (FR-042). Si alguna
//	@Description	condición se cumple, responde 409 con DeletionBlockedResponse indicando todas las que
//	@Description	aplican, en el orden sprint_history, effort, defects y estimation_sessions, y la
//	@Description	historia no se toca (FR-043). Una historia eliminada desaparece de toda consulta
//	@Description	posterior: operar sobre ella responde 404 como historia inexistente (FR-044). En un
//	@Description	proyecto finished se rechaza con 409 y ErrorResponse (FR-048, RN-15).
//	@Tags			Backlog
//	@ID				deleteStory
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string					true	"Identificador propio, estable, inmutable y no secuencial de la historia (FR-009)"	format(uuid)
//	@Success		204		"Historia eliminada."
//	@Failure		401		{object}	delivery.ErrorResponse	"Se requiere iniciar sesión (FR-045)."
//	@Failure		404		{object}	delivery.ErrorResponse	"La historia no existe, fue eliminada o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo (FR-044, FR-047, SC-006)."
//	@Failure		409		{object}	DeletionBlockedResponse	"La historia tiene historial asociado; el cuerpo lista todos los motivos (FR-043). Si el proyecto está finished, el cuerpo es ErrorResponse con el mensaje de solo lectura (FR-048, RN-15)."
//	@Failure		500		{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId} [delete]
func (h *Handler) deleteStory(c *gin.Context) {
	// RequireStoryAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "deleteStory sin acceso a la historia en el contexto")
		return
	}

	// Se elimina la historia. El rechazo por historial responde 409 con todos los motivos; el resto
	// de los errores de dominio se traduce con respondServiceError.
	err := h.stories.Delete(c.Request.Context(), access)
	var blocked *domain.DeletionBlockedError
	switch {
	case errors.As(err, &blocked):
		c.JSON(http.StatusConflict, newDeletionBlockedResponse(msgDeletionBlocked, blocked.Reasons))
	case err != nil:
		respondServiceError(c, err, "no se pudo eliminar la historia")
	default:
		c.Status(http.StatusNoContent)
	}
}

// listStories godoc
//
//	@Summary		Lista el backlog del proyecto, ordenado y opcionalmente filtrado
//	@Description	Devuelve las historias del proyecto ordenadas por prioridad (high, medium, low) y,
//	@Description	dentro de cada prioridad, por fecha de creación ascendente; ante igual prioridad y fecha
//	@Description	se desempata por el identificador, de modo que el orden es estable (FR-032, RN-17). Los
//	@Description	filtros status y priority se pueden repetir: dentro de un mismo filtro los valores se
//	@Description	combinan con O y entre los dos filtros con Y (FR-034). Un valor que no existe se rechaza
//	@Description	con 400 indicando los valores admitidos (FR-037). Un backlog vacío y unos filtros sin
//	@Description	coincidencias no son errores: responden 200 con la lista vacía y emptyReason en
//	@Description	backlog_empty o no_matches (FR-035, FR-036). Ninguna historia de otro proyecto aparece
//	@Description	(FR-038). No ofrece búsqueda por texto ni paginación.
//	@Tags			Backlog
//	@ID				listStories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			status		query		[]string							false	"Filtra por estado; se puede repetir."		collectionFormat(multi)	Enums(pending, in_progress, completed)
//	@Param			priority	query		[]string							false	"Filtra por prioridad; se puede repetir."	collectionFormat(multi)	Enums(high, medium, low)
//	@Success		200			{object}	StoryListResponse					"Listado del backlog."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Un filtro tiene un valor que no existe (FR-037)."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión (FR-045)."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo (FR-047, SC-006)."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/stories [get]
func (h *Handler) listStories(c *gin.Context) {
	// El proyecto sale del parámetro que RequireMembership ya validó; la sesión solo se exige.
	_, projectID, ok := sessionAndProject(c, "listStories")
	if !ok {
		return
	}

	// Los filtros repetibles se validan antes de llegar al servicio; un valor no admitido responde
	// 400 con el mismo formato que la creación (FR-037).
	filter, fields := ParseStoryFilter(c.QueryArray("status"), c.QueryArray("priority"))
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se lista el backlog; cualquier error del servicio es 500 genérico, registrado en el log.
	view, err := h.stories.List(c.Request.Context(), projectID, filter)
	if err != nil {
		respondServiceError(c, err, "no se pudo listar el backlog")
		return
	}

	c.JSON(http.StatusOK, newStoryListResponse(view, filter))
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

// bindAndValidate decodifica el cuerpo JSON como Req y lo valida con validate antes de llegar al
// servicio. Un cuerpo que no es JSON válido o está vacío responde 400 con ErrorResponse, y los campos
// inválidos, 400 con ValidationErrorResponse listando todos a la vez. Si ya respondió, devuelve false
// y el manejador solo tiene que cortar.
func bindAndValidate[Req, Out any](c *gin.Context, validate func(Req) (Out, []authdelivery.FieldError)) (Out, bool) {
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		var zero Out
		return zero, false
	}
	out, fields := validate(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return out, false
	}
	return out, true
}

// respondInvalidFields responde el 400 del contrato con el mensaje general y el detalle de cada campo
// inválido, común a los cuerpos de creación, estimación y refinamiento y a los filtros del listado.
func respondInvalidFields(c *gin.Context, fields []authdelivery.FieldError) {
	c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
}

// bindStoryInput decodifica y valida el cuerpo de la creación con ValidateStoryInput (FR-013) y, si
// todo es válido, mapea los datos normalizados a la entrada del servicio campo a campo.
func bindStoryInput(c *gin.Context) (service.CreateStoryInput, bool) {
	story, ok := bindAndValidate(c, ValidateStoryInput)
	if !ok {
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
// contrato: la historia inexistente es 404 y la solo lectura, la historia completada y la historia
// comprometida que perdería su condición de lista son 409. Lo que no es un error de dominio conocido
// se registra en el log con el mensaje recibido y se responde 500 genérico, sin exponer la causa
// (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	switch {
	case errors.Is(err, domain.ErrStoryNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgStoryNotFound})
	case errors.Is(err, domain.ErrBacklogReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgBacklogReadOnly})
	case errors.Is(err, domain.ErrStoryCompleted):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgStoryCompleted})
	case errors.Is(err, domain.ErrCommittedStoryMustStayReady):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgStoryCommitted})
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

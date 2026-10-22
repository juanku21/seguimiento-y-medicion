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
// no admitido y de la historia Completada son los de la asignación y la baja de historias (US2). Los
// del otro sprint activo, que lo nombra, y de la transición inválida son los del inicio (US3). Los de
// la historia fuera del sprint Activo y de la transición de avance no admitida, que enumera las tres
// permitidas, son los del avance de las historias (US4). Los del sprint no Activo y del sprint ya
// Cerrado son propios del cierre (US5): este último difiere a propósito del de las demás escrituras
// porque el contrato remarca que el estado Cerrado es definitivo.
const (
	msgInvalidFields          = "Hay campos inválidos en la solicitud."
	msgInvalidBody            = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired        = "Se requiere iniciar sesión."
	msgSprintNotFound         = "El sprint no existe."
	msgStoryNotFound          = "La historia no existe."
	msgSprintsReadOnly        = "El proyecto está finalizado y sus sprints son de solo lectura."
	msgSprintOverlap          = "El período se superpone con el del sprint «%s»."
	msgSprintClosed           = "El sprint está cerrado y no admite cambios."
	msgStoriesNotAdmitted     = "Algunas historias no se pueden comprometer en el sprint; no se asignó ninguna."
	msgCompletedStoryRemoval  = "No se puede quitar una historia Completada. Primero devolvela a En progreso."
	msgActiveSprintExists     = "El proyecto ya tiene el sprint activo «%s». Cerralo antes de iniciar otro."
	msgInvalidStartTransition = "Transición no permitida: solo se inicia un sprint Planificado."
	msgProgressOutsideActive  = "Solo se registra avance sobre historias del sprint activo."
	msgInvalidStoryTransition = "Transición no permitida. Las admitidas son: Pendiente → En progreso, En progreso → Completada y Completada → En progreso."
	msgCloseNotActive         = "Solo se puede cerrar un sprint Activo."
	msgCloseClosed            = "El sprint ya está cerrado; el estado Cerrado es definitivo."
	msgInternalError          = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
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

// listSprints godoc
//
//	@Summary		Lista los sprints del proyecto
//	@Description	Devuelve los sprints del proyecto, con su estado, en orden de fecha de inicio
//	@Description	descendente y luego por identificador (FR-050, FR-052). El filtro status es opcional y
//	@Description	repetible (?status=planned&status=active). Con ?status=closed devuelve el historial de
//	@Description	sprints cerrados: cada ítem trae en results los totales congelados en su instantánea;
//	@Description	en los sprints que no están cerrados, results es null (FR-051). Un proyecto sin sprints
//	@Description	devuelve una lista vacía con emptyReason no_sprints, no un error (FR-054); si hay
//	@Description	sprints pero ninguno cumple el filtro, emptyReason es no_matches. Cualquier integrante
//	@Description	puede consultarlo, también en un proyecto finished.
//	@Tags			Sprints
//	@ID				listSprints
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			status		query		[]string							false	"Filtro por estado; puede repetirse"	collectionFormat(multi)	Enums(planned, active, closed)
//	@Success		200			{object}	SprintListResponse					"Lista de sprints, posiblemente vacía."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Algún valor de status no es planned, active ni closed; señala status."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/sprints [get]
func (h *Handler) listSprints(c *gin.Context) {
	// El proyecto sale del parámetro que RequireMembership ya validó como UUID de un proyecto del que
	// quien pide es integrante.
	_, projectID, ok := sessionAndProject(c, "listSprints")
	if !ok {
		return
	}

	// El filtro de estado se valida antes de llegar al servicio; un valor no admitido responde 400
	// con el campo status.
	statuses, fields := ParseSprintStatusFilter(c.QueryArray(sprintStatusQueryField))
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se listan los sprints con el motivo del vacío; cualquier error es 500 genérico.
	view, err := h.sprints.List(c.Request.Context(), projectID, statuses)
	if err != nil {
		respondServiceError(c, err, "no se pudieron listar los sprints")
		return
	}

	c.JSON(http.StatusOK, newSprintListResponse(view, statuses))
}

// getSprint godoc
//
//	@Summary		Consulta el detalle de un sprint
//	@Description	Devuelve los datos del sprint, su estado y sus historias (FR-053). En un sprint planned
//	@Description	o active, las historias muestran sus datos vigentes; en uno closed, el título, la
//	@Description	prioridad y los Story Points que tenían al cerrarse, tomados de la instantánea, con los
//	@Description	totales en results y el factor congelado en hoursPerStoryPoint (FR-043, US6-8). Solo en
//	@Description	un sprint active, schedule informa los días restantes hasta la fecha de fin prevista,
//	@Description	con el día de fin incluido; si ya pasó, informa overdueDays en lugar de un número
//	@Description	negativo (RN-23). En los demás estados schedule es null. Cualquier integrante del
//	@Description	proyecto puede consultarlo, también en un proyecto finished. Un sprint inexistente,
//	@Description	eliminado o de un proyecto del que quien pide no es integrante responde el mismo 404
//	@Description	(FR-058).
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

// startSprint godoc
//
//	@Summary		Inicia un sprint Planificado
//	@Description	Pasa el sprint de planned a active, registra el momento real de inicio y congela los
//	@Description	Story Points comprometidos al inicio (FR-026, FR-028). Se admite sin historias (FR-030)
//	@Description	y antes de la fecha de inicio prevista (FR-031). No cambia el estado del proyecto
//	@Description	(FR-029). Se rechaza con 409 si ya hay otro sprint active en el proyecto, indicando
//	@Description	cuál (FR-027), si el sprint no está planned (FR-032) o si el proyecto está finished.
//	@Description	Cualquier integrante puede iniciar un sprint (RN-01).
//	@Tags			Sprints
//	@ID				startSprint
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sprintId	path		string					true	"Identificador del sprint"	format(uuid)
//	@Success		200			{object}	SprintDetail			"Sprint iniciado. El cuerpo es el detalle del sprint."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El sprint no existe, fue eliminado o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	SprintConflictResponse	"Ya hay otro sprint activo en el proyecto, que se nombra en el cuerpo (FR-027). Si el sprint no está planned, está closed o el proyecto está finished, la respuesta es ErrorResponse con el mensaje \"Transición no permitida: solo se inicia un sprint Planificado.\", \"El sprint está cerrado y no admite cambios.\" o \"El proyecto está finalizado y sus sprints son de solo lectura.\""
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/sprints/{sprintId}/start [post]
func (h *Handler) startSprint(c *gin.Context) {
	// RequireSprintAccess ya verificó la sesión y la membresía y dejó el acceso en el contexto. Si no
	// está, la ruta se registró fuera de ese middleware: es un error de composición y responde 500.
	access, ok := sprintAccessFromContext(c)
	if !ok {
		respondInternalError(c, "startSprint sin acceso al sprint en el contexto")
		return
	}

	// Se inicia el sprint. La transición inválida tiene un mensaje propio del inicio, así que se
	// traduce acá; los demás rechazos de dominio pasan por respondServiceError.
	view, err := h.sprints.Start(c.Request.Context(), access)
	if errors.Is(err, domain.ErrInvalidSprintTransition) {
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgInvalidStartTransition})
		return
	}
	if err != nil {
		respondServiceError(c, err, "no se pudo iniciar el sprint")
		return
	}

	c.JSON(http.StatusOK, newSprintDetail(view))
}

// closeSprint godoc
//
//	@Summary		Cierra el sprint Activo y congela su instantánea
//	@Description	Pasa el sprint de active a closed en cualquier momento, antes o después de la fecha de
//	@Description	fin prevista, registrando el momento real de cierre y quién cerró: el titular de la
//	@Description	sesión (FR-039, FR-040). En la misma operación congela de cada historia comprometida su
//	@Description	título, prioridad, Story Points y resultado; copia el factor de horas por Story Point
//	@Description	vigente del proyecto; y devuelve al backlog como pending y sin sprint las historias no
//	@Description	completadas (FR-041 a FR-043). Un sprint sin historias cierra con la instantánea en
//	@Description	cero (FR-045). Desde el cierre, nada de lo congelado cambia (FR-044, RN-16). No cambia
//	@Description	el estado del proyecto (FR-049). Cualquier integrante puede cerrar un sprint (RN-01).
//	@Tags			Sprints
//	@ID				closeSprint
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sprintId	path		string					true	"Identificador del sprint"	format(uuid)
//	@Success		200			{object}	SprintDetail			"Sprint cerrado. El cuerpo es el detalle con la instantánea."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El sprint no existe, fue eliminado o pertenece a un proyecto del que quien pide no es integrante; las tres situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	delivery.ErrorResponse	"Proyecto finished o sprint no active: \"El proyecto está finalizado y sus sprints son de solo lectura.\", \"Solo se puede cerrar un sprint Activo.\" (sprint planned) o \"El sprint ya está cerrado; el estado Cerrado es definitivo.\" (sprint closed)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/sprints/{sprintId}/close [post]
func (h *Handler) closeSprint(c *gin.Context) {
	// RequireSprintAccess ya verificó la sesión y dejó el acceso en el contexto, así que la falta del
	// titular o del acceso solo es posible si la ruta se registró fuera de ese middleware: es un error
	// de composición y responde 500.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "closeSprint sin titular de la sesión en el contexto")
		return
	}
	access, ok := sprintAccessFromContext(c)
	if !ok {
		respondInternalError(c, "closeSprint sin acceso al sprint en el contexto")
		return
	}

	// Se cierra el sprint atribuido al titular de la sesión. El sprint no Activo y el ya Cerrado
	// tienen mensajes propios del cierre, así que se traducen acá; los demás rechazos de dominio
	// pasan por respondServiceError.
	view, err := h.sprints.Close(c.Request.Context(), access, actorID)
	switch {
	case errors.Is(err, domain.ErrSprintNotActive):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgCloseNotActive})
		return
	case errors.Is(err, domain.ErrSprintClosed):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgCloseClosed})
		return
	case err != nil:
		respondServiceError(c, err, "no se pudo cerrar el sprint")
		return
	}

	c.JSON(http.StatusOK, newSprintDetail(view))
}

// changeStoryStatus godoc
//
//	@Summary		Registra el avance de una historia del sprint Activo
//	@Description	Cambia el estado de una historia comprometida en el sprint active de su proyecto
//	@Description	(FR-033). Transiciones admitidas: pending → in_progress, in_progress → completed y
//	@Description	completed → in_progress. Al completar se registra la fecha y hora; al devolverla a En
//	@Description	progreso se borra (FR-034, FR-035). Se rechaza con 409 si la historia no está en el
//	@Description	sprint Activo (FR-036, FR-037) o si la transición no está admitida, incluido el salto
//	@Description	de pending a completed (FR-038). Cualquier integrante puede registrar avance (RN-01).
//	@Tags			Sprints
//	@ID				changeStoryStatus
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string								true	"Identificador de la historia"	format(uuid)
//	@Param			request	body		StoryStatusInput					true	"Estado destino de la historia"
//	@Success		200		{object}	StoryProgressResponse				"Estado registrado."
//	@Failure		400		{object}	delivery.ValidationErrorResponse	"Estado ausente o desconocido; señala status. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401		{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse				"La historia no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409		{object}	delivery.ErrorResponse				"Proyecto finished (\"El proyecto está finalizado y sus sprints son de solo lectura.\"), historia de un sprint Planificado o sin sprint (\"Solo se registra avance sobre historias del sprint activo.\"), historia de un sprint Cerrado (\"El sprint está cerrado y no admite cambios.\") o transición no admitida (\"Transición no permitida. Las admitidas son: Pendiente → En progreso, En progreso → Completada y Completada → En progreso.\")."
//	@Failure		500		{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/status [put]
func (h *Handler) changeStoryStatus(c *gin.Context) {
	// RequireStoryAccess de la 003 ya respondió 404 ante un identificador mal formado, así que un
	// storyId que no sea UUID solo es posible si la ruta se registró fuera de ese middleware: es un
	// error de composición y responde 500.
	storyID, err := uuid.Parse(c.Param("storyId"))
	if err != nil {
		respondInternalError(c, "changeStoryStatus sin un storyId válido en la ruta", "error", err)
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio: un JSON mal formado responde 400
	// con ErrorResponse y un estado ausente o desconocido, 400 con el campo status.
	var req StoryStatusInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	status, fields := ValidateStoryStatusInput(req)
	if len(fields) > 0 {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se aplica la transición y se traducen los rechazos de dominio con respondServiceError.
	ctx := c.Request.Context()
	if err := h.sprints.ChangeStoryProgress(ctx, storyID, status); err != nil {
		respondServiceError(c, err, "no se pudo registrar el avance de la historia")
		return
	}

	// La respuesta toma el sprint y la fecha de completado del compromiso abierto releído. Si ya no
	// existe, un cierre se intercaló después del cambio: es inesperado y responde 500 sin detalles.
	commitment, err := h.sprints.FindOpenCommitment(ctx, storyID)
	if err != nil {
		respondInternalError(c, "no se pudo leer el compromiso abierto de la historia", "error", err)
		return
	}
	if commitment == nil {
		respondInternalError(c, "la historia no tiene compromiso abierto después de registrar su avance", "story_id", storyID)
		return
	}

	c.JSON(http.StatusOK, newStoryProgressResponse(status, commitment))
}

// listStorySprints godoc
//
//	@Summary		Lista los sprints en los que estuvo una historia
//	@Description	Devuelve todos los sprints en los que la historia estuvo planificada y el resultado que
//	@Description	tuvo en cada uno, en orden de fecha de inicio ascendente (FR-055, RN-17). En los
//	@Description	sprints cerrados, outcome es completed o not_completed; en el sprint abierto en el que
//	@Description	está, null. Una historia quitada de un sprint antes del cierre no figura en ese sprint.
//	@Tags			Sprints
//	@ID				listStorySprints
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string						true	"Identificador de la historia"	format(uuid)
//	@Success		200		{object}	StorySprintHistoryResponse	"Historial de sprints de la historia, posiblemente vacío."
//	@Failure		401		{object}	delivery.ErrorResponse		"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse		"La historia no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		500		{object}	delivery.ErrorResponse		"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/sprints [get]
func (h *Handler) listStorySprints(c *gin.Context) {
	// RequireStoryAccess de la 003 ya respondió 404 ante un identificador mal formado, así que un
	// storyId que no sea UUID solo es posible si la ruta se registró fuera de ese middleware: es un
	// error de composición y responde 500.
	storyID, err := uuid.Parse(c.Param("storyId"))
	if err != nil {
		respondInternalError(c, "listStorySprints sin un storyId válido en la ruta", "error", err)
		return
	}

	// Se lee el historial de la historia; cualquier error es 500 genérico.
	items, err := h.sprints.StoryHistory(c.Request.Context(), storyID)
	if err != nil {
		respondServiceError(c, err, "no se pudo leer el historial de sprints de la historia")
		return
	}

	c.JSON(http.StatusOK, newStorySprintHistoryResponse(items))
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
// contrato: la superposición de períodos y el otro sprint activo son 409 con el sprint en conflicto,
// el lote no admitido es 409 con un problema por historia, y el proyecto finalizado, el sprint
// cerrado, la historia Completada que se quiere quitar, el avance fuera del sprint Activo y la
// transición de avance no admitida son 409 con su mensaje. El sprint que
// desapareció entre el middleware y el servicio y la historia inexistente, ajena o fuera del sprint
// son 404. Lo que no es un error de dominio conocido se registra en el log con el mensaje recibido y
// se responde 500 genérico, sin exponer la causa (Principio VII).
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var overlap *domain.SprintOverlapError
	var activeExists *domain.ActiveSprintExistsError
	var notAdmitted *domain.StoriesNotAdmittedError
	switch {
	case errors.As(err, &overlap):
		message := fmt.Sprintf(msgSprintOverlap, overlap.Sprint.Name)
		c.JSON(http.StatusConflict, newSprintConflictResponse(message, overlap.Sprint))
	case errors.As(err, &activeExists):
		message := fmt.Sprintf(msgActiveSprintExists, activeExists.Sprint.Name)
		c.JSON(http.StatusConflict, newSprintConflictResponse(message, activeExists.Sprint))
	case errors.As(err, &notAdmitted):
		c.JSON(http.StatusConflict, newStoriesNotAdmittedResponse(msgStoriesNotAdmitted, notAdmitted.Problems))
	case errors.Is(err, domain.ErrSprintsReadOnly):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgSprintsReadOnly})
	case errors.Is(err, domain.ErrSprintClosed):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgSprintClosed})
	case errors.Is(err, domain.ErrCompletedStoryRemoval):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgCompletedStoryRemoval})
	case errors.Is(err, domain.ErrProgressOutsideActiveSprint):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgProgressOutsideActive})
	case errors.Is(err, domain.ErrInvalidStoryTransition):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgInvalidStoryTransition})
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

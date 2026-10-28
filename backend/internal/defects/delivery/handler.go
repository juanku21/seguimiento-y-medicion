package delivery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/service"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// storyAccessContextKey es la clave con la que RequireStoryAccess de la 003 deja el StoryAccess en el
// contexto de gin. Replica la de backlog/delivery porque este paquete no lo importa (Principio III).
const storyAccessContextKey = "backlog.storyAccess"

// msgFilterForeign es el rechazo de un filtro que no corresponde al proyecto.
const msgFilterForeign = "El filtro no corresponde al proyecto."

// Mensajes de las respuestas de error del alta de defectos, en español y dirigidos a la persona.
const (
	msgInvalidFields      = "Hay campos inválidos en la solicitud."
	msgInvalidBody        = "El cuerpo de la solicitud debe ser un JSON válido."
	msgSessionRequired    = "Se requiere iniciar sesión."
	msgInternalError      = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
	msgStoryNotFound      = "La historia no existe."
	msgSprintNotFound     = "El sprint no existe."
	msgSprintPlanned      = "Un defecto solo se detecta en un sprint Activo o Cerrado."
	msgDefectsReadOnly    = "El proyecto está finalizado y sus defectos son de solo lectura."
	msgNoDetectionSprint  = "Todavía no hay ningún sprint Activo ni Cerrado que pueda ser sprint de detección."
	codeReadOnly          = "read_only"
	codeNoDetectionSprint = "no_detection_sprint"

	msgResolutionSprintPlanned   = "Un defecto solo se resuelve en un sprint Activo o Cerrado."
	msgResolutionBeforeDetection = "El sprint de resolución no puede empezar antes que el de detección."
	msgSkipToResolved            = "Un defecto Abierto primero debe pasar a En progreso."
	msgOnlyResolvedReopens       = "Solo se reabre un defecto Resuelto."
	codeInvalidTransition        = "invalid_transition"

	msgOnlyOpenDiscards = "Solo se descarta un defecto Abierto o En progreso; para descartarlo, primero reabrilo."
	msgDiscardedFinal   = "El defecto está Descartado, que es un estado definitivo."
	msgDefectDiscarded  = "El defecto está Descartado y es de solo lectura."
	msgLinksFrozen      = "La historia relacionada y el sprint de detección solo se corrigen mientras el defecto está Abierto."
	codeDefectDiscarded = "defect_discarded"
	codeLinksFrozen     = "links_frozen"
)

// statusLabels son los nombres en español de los estados, para los mensajes dirigidos a la persona.
var statusLabels = map[domain.DefectStatus]string{
	domain.DefectStatusOpen:       "Abierto",
	domain.DefectStatusInProgress: "En progreso",
	domain.DefectStatusResolved:   "Resuelto",
	domain.DefectStatusDiscarded:  "Descartado",
}

// DefectService declara lo que el manejador necesita del servicio de defectos.
type DefectService interface {
	// Create registra un defecto en el proyecto atribuido al actor.
	Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.DefectInput) (*domain.DefectView, error)
	// Update aplica una modificación parcial de campos atribuyéndola al actor.
	Update(ctx context.Context, defectID, actorID uuid.UUID, patch domain.DefectPatch) (*domain.DefectView, error)
	// ChangeStatus cambia el estado de un defecto atribuyendo el cambio al actor.
	ChangeStatus(ctx context.Context, defectID, actorID uuid.UUID, change domain.StatusChange) (*domain.DefectView, error)
	// Get devuelve el detalle de un defecto con sus relaciones resueltas.
	Get(ctx context.Context, defectID uuid.UUID) (*domain.DefectView, error)
	// ListForProject lista los defectos del proyecto según el filtro.
	ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.DefectFilter) (*service.DefectListView, error)
	// ListForStory lista los defectos de la historia a la que el actor ya accedió.
	ListForStory(ctx context.Context, access *backlogdomain.StoryAccess) (*service.DefectListView, error)
}

// Handler expone los endpoints HTTP de los defectos.
type Handler struct {
	svc DefectService
}

// NewHandler crea el manejador sobre el servicio recibido.
func NewHandler(svc DefectService) *Handler {
	return &Handler{svc: svc}
}

// createDefect godoc
//
//	@Summary		Registra un defecto
//	@Description	Registra un defecto sobre una historia del proyecto, con su descripción (1 a 2000
//	@Description	caracteres), su severidad y el sprint de detección, que debe estar Activo o Cerrado
//	@Description	(FR-001 a FR-006). El defecto nace Abierto y su autor es siempre el titular de la
//	@Description	sesión: cualquier dato de autoría que llegue en el cuerpo se ignora (FR-056). Cualquier
//	@Description	integrante del proyecto puede registrarlo.
//	@Tags			Defectos
//	@ID				createDefect
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			request		body		DefectInput							true	"Datos del defecto a registrar"
//	@Success		201			{object}	Defect								"Defecto creado."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Entrada inválida o sprint de detección Planificado. Indica todos los campos a corregir y por qué, y no crea nada. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto, la historia o el sprint no existen o pertenecen a otro proyecto; las situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	DefectConflictResponse				"Proyecto finished (read_only) o sin ningún sprint Activo ni Cerrado (no_detection_sprint)."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/defects [post]
func (h *Handler) createDefect(c *gin.Context) {
	// Se toman el titular de la sesión y el proyecto de la ruta; el autor nunca sale del cuerpo.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "createDefect sin un projectId válido en la ruta", "error", err)
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio (Principio VII).
	var req DefectInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	input, fields := ValidateDefectInput(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se registra el defecto y se traducen los rechazos de dominio a la respuesta del contrato.
	view, err := h.svc.Create(c.Request.Context(), projectID, actorID, input)
	if err != nil {
		respondServiceError(c, err, "no se pudo registrar el defecto")
		return
	}

	c.JSON(http.StatusCreated, newDefect(view, projectID))
}

// getDefect godoc
//
//	@Summary		Consulta el detalle de un defecto
//	@Description	Devuelve todos los datos del defecto, su estado y sus dos sprints; resolvedSprint es
//	@Description	null si el defecto no está resolved (FR-049). Cualquier integrante del proyecto puede
//	@Description	consultarlo.
//	@Tags			Defectos
//	@ID				getDefect
//	@Produce		json
//	@Security		BearerAuth
//	@Param			defectId	path		string					true	"Identificador del defecto"	format(uuid)
//	@Success		200			{object}	Defect					"Detalle del defecto."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"El defecto no existe, el identificador está mal formado o pertenece a un proyecto del que quien pide no es integrante; las situaciones devuelven el mismo cuerpo."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/defects/{defectId} [get]
func (h *Handler) getDefect(c *gin.Context) {
	// RequireDefectAccess ya resolvió el defecto y su proyecto; sin ese dato la cadena está mal armada.
	access, ok := defectAccessFromContext(c)
	if !ok {
		respondInternalError(c, "getDefect sin DefectAccess en el contexto")
		return
	}

	view, err := h.svc.Get(c.Request.Context(), access.DefectID)
	if err != nil {
		respondServiceError(c, err, "no se pudo leer el defecto")
		return
	}

	c.JSON(http.StatusOK, newDefect(view, access.ProjectID))
}

// changeDefectStatus godoc
//
//	@Summary		Cambia el estado de un defecto
//	@Description	Aplica una transición de la máquina de estados (FR-018): desde open se pasa a
//	@Description	in_progress y desde in_progress a resolved. Resolver exige resolvedSprintId: un sprint
//	@Description	del proyecto, active o closed, cuya fecha de inicio no sea anterior a la del sprint de
//	@Description	detección; el mismo sprint es válido (FR-023 a FR-026). En cualquier otro destino,
//	@Description	resolvedSprintId se rechaza. Ninguna transición cambia la historia ni el sprint de
//	@Description	detección (FR-027). La transición se valida contra el estado vigente al aplicarla; si
//	@Description	otra acción lo cambió, se rechaza con 409 invalid_transition y el estado actual (FR-018,
//	@Description	RN-24). El cambio se atribuye siempre al titular de la sesión, que puede ser cualquier
//	@Description	integrante del proyecto.
//	@Tags			Defectos
//	@ID				changeDefectStatus
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			defectId	path		string								true	"Identificador del defecto"	format(uuid)
//	@Param			request		body		DefectStatusChange					true	"Estado destino y, al resolver, sprint de resolución"
//	@Success		200			{object}	Defect								"Defecto con su nuevo estado."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Estado inválido, sprint de resolución ausente al resolver o indicado en otro destino, sprint de resolución Planificado o anterior al de detección (field: resolvedSprintId). Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El defecto no existe o es ajeno, o el sprint de resolución no existe o pertenece a otro proyecto; cada situación devuelve el mismo cuerpo que su inexistente."
//	@Failure		409			{object}	DefectConflictResponse				"Proyecto finished (read_only) o transición no admitida desde el estado vigente (invalid_transition, con currentStatus y allowedStatuses)."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/defects/{defectId}/status [put]
func (h *Handler) changeDefectStatus(c *gin.Context) {
	// El actor es el titular de la sesión y el defecto es el que resolvió RequireDefectAccess.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	access, ok := defectAccessFromContext(c)
	if !ok {
		respondInternalError(c, "changeDefectStatus sin DefectAccess en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio (Principio VII).
	var req DefectStatusChange
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	change, fields := ValidateStatusChange(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se aplica la transición y se traducen los rechazos de dominio a la respuesta del contrato.
	view, err := h.svc.ChangeStatus(c.Request.Context(), access.DefectID, actorID, change)
	if err != nil {
		respondServiceError(c, err, "no se pudo cambiar el estado del defecto")
		return
	}

	c.JSON(http.StatusOK, newDefect(view, access.ProjectID))
}

// updateDefect godoc
//
//	@Summary		Modifica los datos de un defecto
//	@Description	Corrige parcialmente la descripción (1 a 2000 caracteres), la severidad, la historia
//	@Description	relacionada y el sprint de detección; los campos ausentes no cambian y al menos uno debe
//	@Description	venir (FR-033, FR-034). La historia y el sprint de detección solo se corrigen con el
//	@Description	defecto open y siguen el mismo criterio que el registro (FR-038). El estado, el sprint de
//	@Description	resolución, el proyecto y el autor no se modifican por esta vía (FR-035). Un defecto
//	@Description	discarded es de solo lectura (FR-036). El cambio se atribuye al titular de la sesión, que
//	@Description	puede ser cualquier integrante del proyecto. Los 409 defect_discarded y links_frozen
//	@Description	informan el estado vigente releído y los destinos admitidos desde él.
//	@Tags			Defectos
//	@ID				updateDefect
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			defectId	path		string								true	"Identificador del defecto"	format(uuid)
//	@Param			request		body		DefectPatch							true	"Campos a corregir"
//	@Success		200			{object}	Defect								"Defecto con los datos corregidos."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Campos inválidos, cuerpo sin cambios, o estado, sprint de resolución, proyecto o autor indicados; sprint de detección Planificado. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El defecto no existe o es ajeno, o la historia o el sprint de detección no existen o pertenecen a otro proyecto; cada situación devuelve el mismo cuerpo que su inexistente."
//	@Failure		409			{object}	DefectConflictResponse				"Proyecto finished (read_only), defecto discarded (defect_discarded) o vínculos congelados (links_frozen), con currentStatus y allowedStatuses."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/defects/{defectId} [patch]
func (h *Handler) updateDefect(c *gin.Context) {
	// El actor es el titular de la sesión y el defecto es el que resolvió RequireDefectAccess.
	actorID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
		return
	}
	access, ok := defectAccessFromContext(c)
	if !ok {
		respondInternalError(c, "updateDefect sin DefectAccess en el contexto")
		return
	}

	// Se decodifica y valida el cuerpo antes de llegar al servicio (Principio VII).
	var req DefectPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	patch, fields := ValidateDefectPatch(req)
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Se aplica la modificación; los rechazos por estado se informan con el estado vigente releído.
	view, err := h.svc.Update(c.Request.Context(), access.DefectID, actorID, patch)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDefectDiscarded):
			h.respondStateConflict(c, access.DefectID, codeDefectDiscarded, msgDefectDiscarded)
		case errors.Is(err, domain.ErrLinksFrozen):
			h.respondStateConflict(c, access.DefectID, codeLinksFrozen, msgLinksFrozen)
		default:
			respondServiceError(c, err, "no se pudo modificar el defecto")
		}
		return
	}

	c.JSON(http.StatusOK, newDefect(view, access.ProjectID))
}

// respondStateConflict responde el 409 con el estado vigente del defecto, releído, y los destinos
// admitidos desde él. Si el defecto ya no se puede leer, responde el error que corresponda.
func (h *Handler) respondStateConflict(c *gin.Context, defectID uuid.UUID, code, message string) {
	view, err := h.svc.Get(c.Request.Context(), defectID)
	if err != nil {
		respondServiceError(c, err, "no se pudo releer el defecto tras el rechazo")
		return
	}
	current := string(view.Defect.Status)
	allowed := make([]string, 0)
	for _, status := range domain.AllowedTransitions(view.Defect.Status) {
		allowed = append(allowed, string(status))
	}
	c.JSON(http.StatusConflict, DefectConflictResponse{
		Code:            code,
		Error:           message,
		CurrentStatus:   &current,
		AllowedStatuses: allowed,
	})
}

// listProjectDefects godoc
//
//	@Summary		Lista los defectos del proyecto con filtros
//	@Description	Devuelve los defectos del proyecto que cumplen todos los filtros indicados (FR-040,
//	@Description	FR-041), incluidos los discarded cuando no se filtra por estado. Los valores repetidos
//	@Description	de un mismo filtro se combinan con "o". El filtro sprintId devuelve los defectos
//	@Description	detectados o resueltos en ese sprint (FR-042). Una lista vacía no es un error:
//	@Description	emptyReason es no_defects si el proyecto no tiene defectos o no_matches si los filtros
//	@Description	no encuentran ninguno (FR-045). Se ordena por severidad descendente y, a igual
//	@Description	severidad, por creación descendente (FR-048).
//	@Tags			Defectos
//	@ID				listProjectDefects
//	@Produce		json
//	@Security		BearerAuth
//	@Param			projectId	path		string								true	"Identificador del proyecto"	format(uuid)
//	@Param			status		query		[]string							false	"Estado del defecto; repetible"		Enums(open, in_progress, resolved, discarded)	collectionFormat(multi)
//	@Param			severity	query		[]string							false	"Severidad del defecto; repetible"	Enums(critical, high, medium, low)				collectionFormat(multi)
//	@Param			storyId		query		string								false	"Historia del proyecto"				format(uuid)
//	@Param			sprintId	query		string								false	"Sprint del proyecto; coincide por detección o por resolución"	format(uuid)
//	@Success		200			{object}	ProjectDefects						"Defectos del proyecto, posiblemente vacío."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Filtro inválido o que no corresponde al proyecto. Indica todos los filtros a corregir y por qué."
//	@Failure		401			{object}	delivery.ErrorResponse				"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse				"El proyecto no existe o quien pide no es integrante; las situaciones devuelven el mismo cuerpo."
//	@Failure		500			{object}	delivery.ErrorResponse				"Error interno sin detalles expuestos."
//	@Router			/api/v1/projects/{projectId}/defects [get]
func (h *Handler) listProjectDefects(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respondInternalError(c, "listProjectDefects sin un projectId válido en la ruta", "error", err)
		return
	}

	// Los filtros de la consulta se validan todos a la vez antes de llegar al servicio (Principio VII).
	filter, fields := ParseDefectFilter(c.QueryArray("status"), c.QueryArray("severity"), c.Query("storyId"), c.Query("sprintId"))
	if len(fields) > 0 {
		respondInvalidFields(c, fields)
		return
	}

	// Un filtro de otro proyecto se informa como campo del 400; el resto sigue la traducción común.
	list, err := h.svc.ListForProject(c.Request.Context(), projectID, filter)
	if err != nil {
		var invalid *domain.InvalidFilterError
		if errors.As(err, &invalid) {
			respondInvalidFields(c, []authdelivery.FieldError{{Field: invalid.Field, Message: msgFilterForeign}})
			return
		}
		respondServiceError(c, err, "no se pudo listar los defectos del proyecto")
		return
	}

	defects, reason := newDefectList(list, projectID)
	c.JSON(http.StatusOK, ProjectDefects{Defects: defects, EmptyReason: reason})
}

// listStoryDefects godoc
//
//	@Summary		Lista los defectos de una historia
//	@Description	Devuelve todos los defectos de la historia, de cualquier estado, con el mismo orden y
//	@Description	los mismos datos que el listado del proyecto (FR-043, FR-044, FR-048). Sin defectos,
//	@Description	emptyReason es no_defects y no es un error (FR-045).
//	@Tags			Defectos
//	@ID				listStoryDefects
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string					true	"Identificador de la historia"	format(uuid)
//	@Success		200		{object}	StoryDefects			"Defectos de la historia, posiblemente vacío."
//	@Failure		401		{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse	"La historia no existe o quien pide no es integrante de su proyecto; las situaciones devuelven el mismo cuerpo."
//	@Failure		500		{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/defects [get]
func (h *Handler) listStoryDefects(c *gin.Context) {
	// RequireStoryAccess de la 003 ya respondió 401 y 404; sin su dato la cadena está mal armada.
	access, ok := storyAccessFromContext(c)
	if !ok {
		respondInternalError(c, "listStoryDefects sin StoryAccess en el contexto")
		return
	}

	list, err := h.svc.ListForStory(c.Request.Context(), access)
	if err != nil {
		respondServiceError(c, err, "no se pudo listar los defectos de la historia")
		return
	}

	defects, reason := newDefectList(list, access.ProjectID)
	c.JSON(http.StatusOK, StoryDefects{Defects: defects, EmptyReason: reason})
}

// storyAccessFromContext devuelve el StoryAccess que RequireStoryAccess dejó en el contexto de gin, y
// si estaba presente.
func storyAccessFromContext(c *gin.Context) (*backlogdomain.StoryAccess, bool) {
	value, exists := c.Get(storyAccessContextKey)
	if !exists {
		return nil, false
	}
	access, ok := value.(*backlogdomain.StoryAccess)
	return access, ok && access != nil
}

// respondServiceError traduce un error del servicio a la respuesta del contrato; lo desconocido
// responde 500 sin exponer la causa y registra logMessage con el detalle.
func respondServiceError(c *gin.Context, err error, logMessage string) {
	var transition *domain.InvalidTransitionError
	switch {
	case errors.As(err, &transition):
		respondInvalidTransition(c, transition)
	case errors.Is(err, domain.ErrResolutionSprintRequired):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "resolvedSprintId", Message: msgResolutionSprintRequired}})
	case errors.Is(err, domain.ErrResolutionSprintPlanned):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "resolvedSprintId", Message: msgResolutionSprintPlanned}})
	case errors.Is(err, domain.ErrResolutionBeforeDetection):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "resolvedSprintId", Message: msgResolutionBeforeDetection}})
	case errors.Is(err, domain.ErrDefectNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgDefectNotFound})
	case errors.Is(err, domain.ErrDetectionSprintPlanned):
		respondInvalidFields(c, []authdelivery.FieldError{{Field: "detectedSprintId", Message: msgSprintPlanned}})
	case errors.Is(err, backlogdomain.ErrStoryNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgStoryNotFound})
	case errors.Is(err, sprintsdomain.ErrSprintNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgSprintNotFound})
	case errors.Is(err, domain.ErrDefectsReadOnly):
		respondConflict(c, codeReadOnly, msgDefectsReadOnly)
	case errors.Is(err, domain.ErrNoDetectionSprint):
		respondConflict(c, codeNoDetectionSprint, msgNoDetectionSprint)
	default:
		respondInternalError(c, logMessage, "error", err)
	}
}

// respondInvalidTransition responde el 409 invalid_transition con el estado vigente, los destinos
// admitidos desde él (siempre como arreglo) y el mensaje del caso.
func respondInvalidTransition(c *gin.Context, e *domain.InvalidTransitionError) {
	current := string(e.Current)
	allowed := make([]string, 0, len(e.Allowed))
	for _, status := range e.Allowed {
		allowed = append(allowed, string(status))
	}
	c.JSON(http.StatusConflict, DefectConflictResponse{
		Code:            codeInvalidTransition,
		Error:           invalidTransitionMessage(e),
		CurrentStatus:   &current,
		AllowedStatuses: allowed,
	})
}

// invalidTransitionMessage elige el mensaje del rechazo según el par (vigente, pedido): el salto de
// Abierto a Resuelto y el pedido de reabrir lo que no está Resuelto tienen texto propio; el resto
// recibe el genérico con las transiciones permitidas desde el estado vigente.
func invalidTransitionMessage(e *domain.InvalidTransitionError) string {
	switch {
	case e.Current == domain.DefectStatusDiscarded:
		return msgDiscardedFinal
	case e.Current == domain.DefectStatusResolved && e.Requested == domain.DefectStatusDiscarded:
		return msgOnlyOpenDiscards
	case e.Current == domain.DefectStatusOpen && e.Requested == domain.DefectStatusResolved:
		return msgSkipToResolved
	case e.Requested == domain.DefectStatusOpen:
		return msgOnlyResolvedReopens
	}

	// El genérico nombra los estados en español; sin destinos admitidos lo dice explícitamente.
	current := statusLabels[e.Current]
	if len(e.Allowed) == 0 {
		return fmt.Sprintf("No se puede pasar de %s a %s. Desde %s no se permite ninguna transición.",
			current, statusLabels[e.Requested], current)
	}
	labels := make([]string, 0, len(e.Allowed))
	for _, status := range e.Allowed {
		labels = append(labels, statusLabels[status])
	}
	return fmt.Sprintf("No se puede pasar de %s a %s. Desde %s se permite: %s.",
		current, statusLabels[e.Requested], current, strings.Join(labels, ", "))
}

// respondConflict responde el 409 sin estado vigente y con allowedStatuses como arreglo vacío.
func respondConflict(c *gin.Context, code, message string) {
	c.JSON(http.StatusConflict, DefectConflictResponse{Code: code, Error: message, AllowedStatuses: []string{}})
}

// respondInvalidFields responde el 400 con todos los campos inválidos a la vez.
func respondInvalidFields(c *gin.Context, fields []authdelivery.FieldError) {
	c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
}

// respondInternalError registra el detalle con slog y responde el 500 genérico (Principio VII).
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

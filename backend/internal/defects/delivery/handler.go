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
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

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
	msgBackToOpen                = "Para volver a Abierto, resolvé el defecto y después reabrilo."
	codeInvalidTransition        = "invalid_transition"
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
	// ChangeStatus cambia el estado de un defecto atribuyendo el cambio al actor.
	ChangeStatus(ctx context.Context, defectID, actorID uuid.UUID, change domain.StatusChange) (*domain.DefectView, error)
	// Get devuelve el detalle de un defecto con sus relaciones resueltas.
	Get(ctx context.Context, defectID uuid.UUID) (*domain.DefectView, error)
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
// Abierto a Resuelto y la vuelta de En progreso a Abierto tienen texto propio; el resto recibe el
// genérico con las transiciones permitidas desde el estado vigente.
func invalidTransitionMessage(e *domain.InvalidTransitionError) string {
	switch {
	case e.Current == domain.DefectStatusOpen && e.Requested == domain.DefectStatusResolved:
		return msgSkipToResolved
	case e.Current == domain.DefectStatusInProgress && e.Requested == domain.DefectStatusOpen:
		return msgBackToOpen
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

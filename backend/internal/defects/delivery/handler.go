package delivery

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

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
)

// DefectService declara lo que el manejador necesita del servicio de defectos.
type DefectService interface {
	// Create registra un defecto en el proyecto atribuido al actor.
	Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.DefectInput) (*domain.DefectView, error)
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
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newDefect(view, projectID))
}

// respondServiceError traduce un error del servicio a la respuesta del contrato; lo desconocido
// responde 500 sin exponer la causa.
func respondServiceError(c *gin.Context, err error) {
	switch {
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
		respondInternalError(c, "no se pudo registrar el defecto", "error", err)
	}
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

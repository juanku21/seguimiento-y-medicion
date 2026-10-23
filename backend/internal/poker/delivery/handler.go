package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/service"
)

// Mensajes de las respuestas de error del Planning Poker, en español y dirigidos a la persona. El
// 401 es el de la 001 y el 500 genérico el de los demás módulos, sin detalles internos. El 404 de
// sesión inexistente lo comparten el middleware de acceso (sesión inexistente, ajena o
// identificador mal formado) y el canal; el 404 de historia es el de la 003. Los tres 409 de
// apertura y el 409 de sesión cerrada los fija contracts/openapi.yaml.
const (
	msgSessionRequired          = "Se requiere iniciar sesión."
	msgEstimationSessionMissing = "La sesión de estimación no existe."
	msgStoryNotFound            = "La historia no existe."
	msgOpenSessionExists        = "La historia ya tiene una sesión de estimación en curso."
	msgStoryCompleted           = "Una historia completada no se estima."
	msgProjectFinished          = "El proyecto está finalizado y es de solo lectura."
	msgSessionClosed            = "La sesión de estimación ya está cerrada."
	msgInternalError            = "Ocurrió un error inesperado. Intentá de nuevo más tarde."
)

// Handler agrupa los manejadores HTTP del módulo de Planning Poker sobre el servicio de sesiones.
type Handler struct {
	sessions *service.SessionService
}

// NewHandler crea los manejadores sobre el servicio de sesiones recibido.
func NewHandler(sessions *service.SessionService) *Handler {
	return &Handler{sessions: sessions}
}

// openEstimationSession godoc
//
// Los esquemas de error se anotan como delivery.X de auth/delivery, igual que en la 002, la 003 y la
// 004: swag los nombra por el nombre del paquete y no por el alias de la importación.
//
//	@Summary		Abre una sesión de Planning Poker sobre la historia
//	@Description	Abre una sesión open con una primera ronda voting sin votos (FR-003). Quien la abre
//	@Description	queda como facilitador y como primer participante conectado (FR-002); su pantalla debe
//	@Description	abrir el canal de eventos para seguir contando como conectado después de 30 segundos.
//	@Description	Se admite sobre historias sin criterios de aceptación y sobre historias ya estimadas
//	@Description	(FR-009). Se rechaza con 409 si la historia ya tiene una sesión open, informando su
//	@Description	identificador para unirse (FR-006); si la historia está completed (FR-007), o si el
//	@Description	proyecto está finished (FR-008). De diez aperturas simultáneas sobre la misma historia
//	@Description	prospera exactamente una (SC-012).
//	@Tags			Planning Poker
//	@ID				openEstimationSession
//	@Produce		json
//	@Security		BearerAuth
//	@Param			storyId	path		string						true	"Identificador de la historia"	format(uuid)
//	@Success		201		{object}	EstimationSession			"Sesión abierta."
//	@Failure		401		{object}	delivery.ErrorResponse		"Se requiere iniciar sesión."
//	@Failure		404		{object}	delivery.ErrorResponse		"La historia no existe, fue eliminada o pertenece a un proyecto del que quien pide no es integrante; las situaciones devuelven el mismo cuerpo."
//	@Failure		409		{object}	OpenSessionExistsResponse	"La historia ya tiene una sesión abierta («La historia ya tiene una sesión de estimación en curso.», con sessionId), está completada («Una historia completada no se estima.», como ErrorResponse) o el proyecto está finalizado («El proyecto está finalizado y es de solo lectura.», como ErrorResponse)."
//	@Failure		500		{object}	delivery.ErrorResponse		"Error interno sin detalles expuestos."
//	@Router			/api/v1/stories/{storyId}/estimation-sessions [post]
func (h *Handler) openEstimationSession(c *gin.Context) {
	// RequireStoryAccess de la 003 ya respondió 404 ante un identificador mal formado y la sesión de
	// la 001 ya exigió un titular, así que su ausencia aquí es un error de composición y responde 500.
	storyID, err := uuid.Parse(c.Param("storyId"))
	if err != nil {
		respondInternalError(c, "openEstimationSession sin un storyId válido en la ruta", "error", err)
		return
	}
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "openEstimationSession sin titular de sesión en el contexto")
		return
	}

	// Se abre la sesión y se traducen los rechazos de dominio a los 404 y 409 del contrato.
	view, err := h.sessions.Open(c.Request.Context(), storyID, userID)
	var exists *domain.OpenSessionExistsError
	switch {
	case errors.As(err, &exists):
		c.JSON(http.StatusConflict, newOpenSessionExistsResponse(msgOpenSessionExists, exists.SessionID))
		return
	case errors.Is(err, domain.ErrStoryCompleted):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgStoryCompleted})
		return
	case errors.Is(err, domain.ErrProjectFinished):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgProjectFinished})
		return
	case errors.Is(err, backlogdomain.ErrStoryNotFound):
		c.JSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgStoryNotFound})
		return
	case err != nil:
		respondInternalError(c, "no se pudo abrir la sesión de estimación", "error", err)
		return
	}

	c.JSON(http.StatusCreated, newEstimationSession(view))
}

// respondInternalError registra el error en el log con sus atributos y responde 500 con el mensaje
// genérico, sin exponer detalles internos.
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

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
// apertura y el 409 de sesión cerrada los fija contracts/openapi.yaml, igual que el 403 de quien no
// participa, el 403 de acción reservada al facilitador, el 409 de ronda revelada del voto y los dos
// 409 de la revelación (ronda ya revelada y ronda sin votos). Los dos 400 de cuerpo son los de los
// demás módulos.
const (
	msgInvalidFields            = "Hay campos inválidos en la solicitud."
	msgInvalidBody              = "El cuerpo de la solicitud debe ser un JSON válido."
	msgNotParticipant           = "No participás de esta sesión de estimación."
	msgFacilitatorOnly          = "Esta acción está reservada al facilitador de la sesión."
	msgRoundRevealed            = "La ronda ya está revelada. Esperá una ronda nueva para votar."
	msgRoundAlreadyRevealed     = "La ronda ya fue revelada."
	msgNoVotesToReveal          = "Hace falta al menos un voto para revelar la ronda."
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

// castEstimationVote godoc
//
//	@Summary		Emite o cambia el voto en la ronda vigente
//	@Description	Registra la carta del titular en la ronda vigente, oculta para el resto (FR-021, FR-023).
//	@Description	Un voto nuevo reemplaza al anterior y el anterior no queda registrado (FR-022). Si con este
//	@Description	voto todos los participantes conectados votaron, la ronda se revela automáticamente en la
//	@Description	misma operación (FR-030, FR-031). Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?,
//	@Description	siempre como texto (FR-020). Cualquier otro valor se rechaza con 400 sin alterar el voto
//	@Description	vigente (FR-025). Un voto no se retira: una carta ausente, nula o vacía se rechaza con 400
//	@Description	indicando que solo se cambia por otra carta, incluida ? (FR-022, RN-36).
//	@Tags			Planning Poker
//	@ID				castEstimationVote
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sessionId	path		string							true	"Identificador de la sesión de estimación"	format(uuid)
//	@Param			body		body		VoteRequest						true	"Carta jugada"
//	@Success		200			{object}	EstimationSession				"Voto registrado; sesión resultante vista por quien votó."
//	@Failure		400			{object}	delivery.ValidationErrorResponse	"Carta no admitida («La carta no es válida. Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?.») o intento de retirar el voto («Un voto emitido no se retira: solo se cambia por otra carta, incluida «?».»), con el detalle en card. Si el cuerpo no es un JSON válido o está vacío, la respuesta es ErrorResponse."
//	@Failure		401			{object}	delivery.ErrorResponse			"Se requiere iniciar sesión."
//	@Failure		403			{object}	delivery.ErrorResponse			"Quien pide no es participante conectado de la sesión («No participás de esta sesión de estimación.», FR-028)."
//	@Failure		404			{object}	delivery.ErrorResponse			"La sesión de estimación no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	SessionClosedResponse			"La sesión está cerrada, con el resultado registrado («La sesión de estimación ya está cerrada.», FR-027), o la ronda vigente ya fue revelada («La ronda ya está revelada. Esperá una ronda nueva para votar.», como ErrorResponse, FR-026)."
//	@Failure		500			{object}	delivery.ErrorResponse			"Error interno sin detalles expuestos."
//	@Router			/api/v1/estimation-sessions/{sessionId}/vote [put]
func (h *Handler) castEstimationVote(c *gin.Context) {
	// RequireEstimationAccess y la sesión de la 001 ya dejaron el acceso y el titular en el contexto;
	// su ausencia aquí es un error de composición y responde 500.
	access, ok := sessionAccessFromContext(c)
	if !ok {
		respondInternalError(c, "castEstimationVote sin acceso a la sesión en el contexto")
		return
	}
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "castEstimationVote sin titular de sesión en el contexto")
		return
	}

	// El cuerpo se decodifica y se valida antes de llegar al servicio, así un cuerpo mal formado, una
	// carta no admitida o un intento de retirar el voto no alteran el voto vigente (Principio VII).
	var req VoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ErrorResponse{Error: msgInvalidBody})
		return
	}
	card, fields := ValidateVoteRequest(req)
	if fields != nil {
		c.JSON(http.StatusBadRequest, authdelivery.ValidationErrorResponse{Error: msgInvalidFields, Fields: fields})
		return
	}

	// Se registra el voto y se traducen los rechazos de dominio a los 403, 404 y 409 del contrato.
	view, err := h.sessions.Vote(c.Request.Context(), access.SessionID, userID, card)
	var closed *domain.SessionClosedError
	switch {
	case errors.As(err, &closed):
		c.JSON(http.StatusConflict, newSessionClosedResponse(msgSessionClosed, closed))
		return
	case errors.Is(err, domain.ErrRoundRevealed):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgRoundRevealed})
		return
	case errors.Is(err, domain.ErrNotParticipant):
		c.JSON(http.StatusForbidden, authdelivery.ErrorResponse{Error: msgNotParticipant})
		return
	case errors.Is(err, domain.ErrSessionNotFound):
		abortEstimationSessionNotFound(c)
		return
	case err != nil:
		respondInternalError(c, "no se pudo registrar el voto", "error", err)
		return
	}

	c.JSON(http.StatusOK, newEstimationSession(view))
}

// revealEstimationRound godoc
//
//	@Summary		Revela la ronda vigente (solo el facilitador)
//	@Description	Revela a mano la ronda vigente aunque haya participantes conectados que no votaron, que
//	@Description	figuran en notVoted (FR-032, FR-035). Exige al menos un voto emitido en la ronda, que
//	@Description	cuenta aunque su autor se haya desconectado (FR-033). Reservada al facilitador (FR-034,
//	@Description	FR-064). La ronda revelada muestra cada voto con su autor y su resultado: consensus con
//	@Description	su valor, divergence con el mínimo y el máximo y todos sus autores, o no_numeric_votes
//	@Description	si solo hubo «?» (FR-036 a FR-042). Un rechazo no produce ningún efecto (SC-011).
//	@Tags			Planning Poker
//	@ID				revealEstimationRound
//	@Produce		json
//	@Security		BearerAuth
//	@Param			sessionId	path		string					true	"Identificador de la sesión de estimación"	format(uuid)
//	@Success		200			{object}	EstimationSession		"Ronda revelada; sesión resultante vista por quien la reveló."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		403			{object}	delivery.ErrorResponse	"Quien pide no es participante conectado de la sesión («No participás de esta sesión de estimación.», FR-028) o no es el facilitador («Esta acción está reservada al facilitador de la sesión.», FR-064)."
//	@Failure		404			{object}	delivery.ErrorResponse	"La sesión de estimación no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	SessionClosedResponse	"La sesión está cerrada, con el resultado registrado («La sesión de estimación ya está cerrada.»), la ronda vigente ya fue revelada («La ronda ya fue revelada.», como ErrorResponse) o no tiene votos («Hace falta al menos un voto para revelar la ronda.», como ErrorResponse, FR-033)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/estimation-sessions/{sessionId}/reveal [post]
func (h *Handler) revealEstimationRound(c *gin.Context) {
	// RequireEstimationAccess y la sesión de la 001 ya dejaron el acceso y el titular en el contexto;
	// su ausencia aquí es un error de composición y responde 500.
	access, ok := sessionAccessFromContext(c)
	if !ok {
		respondInternalError(c, "revealEstimationRound sin acceso a la sesión en el contexto")
		return
	}
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "revealEstimationRound sin titular de sesión en el contexto")
		return
	}

	// Se revela la ronda y se traducen los rechazos de dominio a los 403, 404 y 409 del contrato.
	view, err := h.sessions.Reveal(c.Request.Context(), access.SessionID, userID)
	var closed *domain.SessionClosedError
	switch {
	case errors.As(err, &closed):
		c.JSON(http.StatusConflict, newSessionClosedResponse(msgSessionClosed, closed))
		return
	case errors.Is(err, domain.ErrRoundRevealed):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgRoundAlreadyRevealed})
		return
	case errors.Is(err, domain.ErrNoVotesToReveal):
		c.JSON(http.StatusConflict, authdelivery.ErrorResponse{Error: msgNoVotesToReveal})
		return
	case errors.Is(err, domain.ErrNotParticipant):
		c.JSON(http.StatusForbidden, authdelivery.ErrorResponse{Error: msgNotParticipant})
		return
	case errors.Is(err, domain.ErrFacilitatorOnly):
		c.JSON(http.StatusForbidden, authdelivery.ErrorResponse{Error: msgFacilitatorOnly})
		return
	case errors.Is(err, domain.ErrSessionNotFound):
		abortEstimationSessionNotFound(c)
		return
	case err != nil:
		respondInternalError(c, "no se pudo revelar la ronda", "error", err)
		return
	}

	c.JSON(http.StatusOK, newEstimationSession(view))
}

// respondInternalError registra el error en el log con sus atributos y responde 500 con el mensaje
// genérico, sin exponer detalles internos.
func respondInternalError(c *gin.Context, logMessage string, attrs ...any) {
	slog.ErrorContext(c.Request.Context(), logMessage, attrs...)
	c.JSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
}

package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// streamPingInterval es la frecuencia del comentario ": ping" que mantiene viva la conexión y
// detecta las caídas por error de escritura (research.md sección 2).
const streamPingInterval = 15 * time.Second

// streamEstimationSession godoc
//
//	@Summary		Se une a la sesión y recibe sus cambios en tiempo real
//	@Description	Canal de Server-Sent Events. Abrirlo une a quien pide como participante conectado de la
//	@Description	ronda vigente (FR-011, FR-017); si ya había participado, recupera su voto de la ronda
//	@Description	vigente (FR-016). Una misma persona puede abrir varios canales: cuenta como un único
//	@Description	participante y sigue conectada mientras le quede al menos uno (FR-013, RN-34). Cuando se
//	@Description	cierra el último, deja de contar como conectada a los 30 segundos si no vuelve (FR-014).
//	@Description	Eventos del canal: state, con un EstimationSession en JSON en data, que es el primero que
//	@Description	se recibe al abrir el canal (FR-077) y se repite ante cada hecho de la sesión y ante cada
//	@Description	cambio del título, la descripción o los criterios de la historia (FR-073, FR-074); el
//	@Description	cliente descarta los de version menor a la que ya tiene. removed, con {} en data, cuando
//	@Description	quien pide dejó de ser integrante del proyecto; después el servidor cierra el canal
//	@Description	(FR-019). Un comentario ": ping" cada 15 segundos, sin datos. Cuando la sesión se cierra, el servidor envía el último state y
//	@Description	cierra el canal. Si la sesión ya estaba cerrada al pedir el canal, responde 409 con el
//	@Description	resultado registrado y no se une (FR-018).
//	@Tags			Planning Poker
//	@ID				streamEstimationSession
//	@Produce		text/event-stream
//	@Security		BearerAuth
//	@Param			sessionId	path		string					true	"Identificador de la sesión de estimación"	format(uuid)
//	@Success		200			{string}	string					"Canal abierto; queda emitiendo eventos."
//	@Failure		401			{object}	delivery.ErrorResponse	"Se requiere iniciar sesión."
//	@Failure		404			{object}	delivery.ErrorResponse	"La sesión de estimación no existe o pertenece a un proyecto del que quien pide no es integrante; las dos situaciones devuelven el mismo cuerpo."
//	@Failure		409			{object}	SessionClosedResponse	"La sesión ya está finished o canceled, con el resultado registrado («La sesión de estimación ya está cerrada.»)."
//	@Failure		500			{object}	delivery.ErrorResponse	"Error interno sin detalles expuestos."
//	@Router			/api/v1/estimation-sessions/{sessionId}/events [get]
func (h *Handler) streamEstimationSession(c *gin.Context) {
	// RequireEstimationAccess y la sesión de la 001 ya dejaron el acceso y el titular en el contexto;
	// su ausencia aquí es un error de composición y responde 500.
	access, ok := sessionAccessFromContext(c)
	if !ok {
		respondInternalError(c, "streamEstimationSession sin acceso a la sesión en el contexto")
		return
	}
	userID, ok := authdelivery.CurrentUserID(c)
	if !ok {
		respondInternalError(c, "streamEstimationSession sin titular de sesión en el contexto")
		return
	}

	// Se une a la persona antes de escribir nada, así una sesión cerrada todavía puede responder 409
	// con el resultado registrado y una sesión que desapareció, el mismo 404 del middleware.
	ctx := c.Request.Context()
	conn, err := h.sessions.Join(ctx, access, userID)
	var closed *domain.SessionClosedError
	switch {
	case errors.As(err, &closed):
		c.JSON(http.StatusConflict, newSessionClosedResponse(closed))
		return
	case errors.Is(err, domain.ErrSessionNotFound):
		abortEstimationSessionNotFound(c)
		return
	case err != nil:
		respondInternalError(c, "no se pudo unir a la sesión de estimación", "error", err)
		return
	}
	defer h.sessions.Leave(conn)

	// Con la conexión unida se fijan las cabeceras del canal y se envían de inmediato, para que el
	// cliente sepa que el canal quedó abierto antes del primer evento.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	c.Writer.Flush()

	// Se reenvía cada vista como evento state y un ping cada 15 segundos, hasta que termina la
	// petición, el hub cierra el canal o falla una escritura; en todos los casos Leave libera la
	// conexión una sola vez. Si el hub cerró el canal porque la persona dejó de ser integrante, se
	// envía removed antes de terminar (FR-019).
	ticker := time.NewTicker(streamPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case view, open := <-conn.Views:
			if !open {
				if conn.Removed() {
					if err := writeStreamChunk(c.Writer, "event: removed\ndata: {}\n\n"); err != nil {
						slog.DebugContext(ctx, "se cortó el canal de la sesión de estimación", "error", err)
					}
				}
				return
			}
			if err := writeStateEvent(c.Writer, &view); err != nil {
				slog.DebugContext(ctx, "se cortó el canal de la sesión de estimación", "error", err)
				return
			}
		case <-ticker.C:
			if err := writeStreamChunk(c.Writer, ": ping\n\n"); err != nil {
				slog.DebugContext(ctx, "se cortó el canal de la sesión de estimación", "error", err)
				return
			}
		}
	}
}

// writeStateEvent serializa la vista con el esquema EstimationSession del contrato y la escribe
// como un evento state de una sola línea de datos.
func writeStateEvent(w gin.ResponseWriter, view *domain.SessionView) error {
	payload, err := json.Marshal(newEstimationSession(view))
	if err != nil {
		return fmt.Errorf("serializing session view: %w", err)
	}
	return writeStreamChunk(w, "event: state\ndata: "+string(payload)+"\n\n")
}

// writeStreamChunk escribe un fragmento del canal y lo envía al cliente sin esperar a que se llene
// el búfer.
func writeStreamChunk(w gin.ResponseWriter, chunk string) error {
	if _, err := w.WriteString(chunk); err != nil {
		return fmt.Errorf("writing stream chunk: %w", err)
	}
	w.Flush()
	return nil
}

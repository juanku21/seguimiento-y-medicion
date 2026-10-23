// Package delivery expone por HTTP el módulo de Planning Poker: middleware de acceso, manejadores
// REST y canal de eventos.
package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// contextKeySessionAccess es la clave con la que RequireEstimationAccess deja en el contexto de gin
// el SessionAccess de quien pide sobre la sesión de estimación de la ruta.
const contextKeySessionAccess = "poker.sessionAccess"

// RequireEstimationAccess exige que el titular de la sesión sea integrante vigente del proyecto de
// la sesión de estimación de la ruta, con una única salida 404 (RN-01; research.md sección 4). Sin
// titular en el contexto corta con el 401 de la 001. La sesión inexistente, la de un proyecto ajeno,
// la de un proyecto del que se fue quitado y el identificador mal formado comparten el mismo 404
// (FR-086 a FR-088, SC-018). Si hay acceso, deja el SessionAccess en el contexto y sigue la cadena.
func RequireEstimationAccess(sessions domain.SessionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser una sesión, así que se responde como
		// inexistente sin consultar la base.
		sessionID, err := uuid.Parse(c.Param("sessionId"))
		if err != nil {
			abortEstimationSessionNotFound(c)
			return
		}

		// Una sola consulta resuelve la existencia de la sesión y la membresía vigente del titular;
		// cualquier otro error es 500 genérico, registrado en el log.
		access, err := sessions.FindAccessible(c.Request.Context(), sessionID, userID)
		if errors.Is(err, domain.ErrSessionNotFound) {
			abortEstimationSessionNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar el acceso a la sesión de estimación", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(contextKeySessionAccess, access)
		c.Next()
	}
}

// abortEstimationSessionNotFound es la única salida 404 del middleware, para que la sesión
// inexistente, la ajena y el identificador mal formado produzcan respuestas byte a byte idénticas.
func abortEstimationSessionNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgEstimationSessionMissing})
}

// sessionAccessFromContext devuelve el SessionAccess que RequireEstimationAccess dejó en el
// contexto, y si estaba presente.
func sessionAccessFromContext(c *gin.Context) (*domain.SessionAccess, bool) {
	value, exists := c.Get(contextKeySessionAccess)
	if !exists {
		return nil, false
	}
	access, ok := value.(*domain.SessionAccess)
	return access, ok
}

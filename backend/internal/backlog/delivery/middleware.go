package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// contextKeyStoryAccess es la clave con la que RequireStoryAccess deja en el contexto de gin el
// StoryAccess de quien pide sobre la historia de la ruta.
const contextKeyStoryAccess = "backlog.storyAccess"

// RequireStoryAccess exige que el titular de la sesión sea integrante del proyecto de la historia de
// la ruta (FR-045 a FR-047, RN-01, RN-02). Sin titular en el contexto corta con el 401 de la 001. La
// historia inexistente, la ajena y el identificador mal formado comparten una única salida 404 con
// el mismo cuerpo, para que sean indistinguibles (SC-006). Si hay acceso, deja el StoryAccess en el
// contexto y sigue la cadena (research.md sección 3).
func RequireStoryAccess(stories domain.StoryRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser una historia, así que se responde como
		// inexistente sin consultar la base.
		storyID, err := uuid.Parse(c.Param("storyId"))
		if err != nil {
			abortStoryNotFound(c)
			return
		}

		// Una sola consulta resuelve la existencia de la historia, la membresía vigente y el estado
		// del proyecto; cualquier otro error es 500 genérico, registrado en el log.
		access, err := stories.FindAccessible(c.Request.Context(), storyID, userID)
		if errors.Is(err, domain.ErrStoryNotFound) {
			abortStoryNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar el acceso a la historia", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(contextKeyStoryAccess, access)
		c.Next()
	}
}

// abortStoryNotFound es la única salida 404 del middleware, para que la historia inexistente, la
// ajena y el identificador mal formado produzcan respuestas byte a byte idénticas (FR-047, SC-006).
func abortStoryNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgStoryNotFound})
}

// storyAccessFromContext devuelve el StoryAccess que RequireStoryAccess dejó en el contexto, y si
// estaba presente.
func storyAccessFromContext(c *gin.Context) (*domain.StoryAccess, bool) {
	value, exists := c.Get(contextKeyStoryAccess)
	if !exists {
		return nil, false
	}
	access, ok := value.(*domain.StoryAccess)
	return access, ok
}

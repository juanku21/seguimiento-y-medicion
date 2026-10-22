package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// contextKeySprintAccess es la clave con la que RequireSprintAccess deja en el contexto de gin el
// SprintAccess de quien pide sobre el sprint de la ruta.
const contextKeySprintAccess = "sprints.sprintAccess"

// RequireSprintAccess exige que el titular de la sesión sea integrante del proyecto del sprint de la
// ruta (FR-056 a FR-058, RN-01). Sin titular en el contexto corta con el 401 de la 001. El sprint
// inexistente, el ajeno y el identificador mal formado comparten una única salida 404 con el mismo
// cuerpo, para que sean indistinguibles (SC-014). Si hay acceso, deja el SprintAccess en el contexto
// y sigue la cadena (research.md sección 3).
func RequireSprintAccess(sprints domain.SprintRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser un sprint, así que se responde como
		// inexistente sin consultar la base.
		sprintID, err := uuid.Parse(c.Param("sprintId"))
		if err != nil {
			abortSprintNotFound(c)
			return
		}

		// Una sola consulta resuelve la existencia del sprint, la membresía vigente y el estado del
		// proyecto; cualquier otro error es 500 genérico, registrado en el log.
		access, err := sprints.FindAccessible(c.Request.Context(), sprintID, userID)
		if errors.Is(err, domain.ErrSprintNotFound) {
			abortSprintNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar el acceso al sprint", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(contextKeySprintAccess, access)
		c.Next()
	}
}

// abortSprintNotFound es la única salida 404 del middleware, para que el sprint inexistente, el
// ajeno y el identificador mal formado produzcan respuestas byte a byte idénticas (FR-058, SC-014).
func abortSprintNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgSprintNotFound})
}

// sprintAccessFromContext devuelve el SprintAccess que RequireSprintAccess dejó en el contexto, y si
// estaba presente.
func sprintAccessFromContext(c *gin.Context) (*domain.SprintAccess, bool) {
	value, exists := c.Get(contextKeySprintAccess)
	if !exists {
		return nil, false
	}
	access, ok := value.(*domain.SprintAccess)
	return access, ok
}

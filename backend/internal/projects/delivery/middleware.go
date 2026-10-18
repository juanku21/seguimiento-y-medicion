package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// contextKeyMembership es la clave con la que RequireMembership deja en el contexto de gin la
// membresía de quien pide en el proyecto de la ruta.
const contextKeyMembership = "projects.membership"

// RequireMembership exige que el titular de la sesión sea integrante del proyecto de la ruta
// (FR-049, RN-03). Sin titular en el contexto corta con el 401 de la 001. El proyecto inexistente,
// el ajeno y el identificador mal formado comparten una única salida 404 con el mismo cuerpo, para
// que sean indistinguibles (SC-005). Si la membresía existe, la deja en el contexto y sigue la
// cadena. El estado Finalizado no se verifica acá: es regla de dominio del servicio (FR-018).
func RequireMembership(members domain.ProjectMemberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser un proyecto, así que se responde como
		// inexistente sin consultar la base.
		projectID, err := uuid.Parse(c.Param("projectId"))
		if err != nil {
			abortProjectNotFound(c)
			return
		}

		// Una sola consulta resuelve a la vez la existencia del proyecto y la membresía; cualquier
		// otro error es 500 genérico, registrado en el log.
		member, err := members.FindByProjectAndUser(c.Request.Context(), projectID, userID)
		if errors.Is(err, domain.ErrProjectNotFound) {
			abortProjectNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar la membresía", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(contextKeyMembership, member)
		c.Next()
	}
}

// abortProjectNotFound es la única salida 404 del middleware, para que el proyecto inexistente y el
// ajeno produzcan respuestas byte a byte idénticas (FR-049, SC-005).
func abortProjectNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgProjectNotFound})
}

// membershipFromContext devuelve la membresía que RequireMembership dejó en el contexto, y si
// estaba presente.
func membershipFromContext(c *gin.Context) (*domain.ProjectMember, bool) {
	value, exists := c.Get(contextKeyMembership)
	if !exists {
		return nil, false
	}
	member, ok := value.(*domain.ProjectMember)
	return member, ok
}

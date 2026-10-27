package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// msgDefectNotFound es el mensaje de la única salida 404 de RequireDefectAccess.
const msgDefectNotFound = "El defecto no existe."

// contextKeyDefectAccess es la clave con la que RequireDefectAccess deja en el contexto de gin el
// DefectAccess de quien pide sobre el defecto de la ruta.
const contextKeyDefectAccess = "defects.defectAccess"

// RequireDefectAccess exige que el titular de la sesión sea integrante vigente del proyecto del
// defecto de la ruta (FR-051, FR-053, FR-055). Sin titular en el contexto corta con el 401 de la 001.
// El defecto inexistente, el de un proyecto ajeno y el identificador mal formado comparten una única
// salida 404 con el mismo cuerpo, para que sean indistinguibles (SC-017). Si hay acceso, deja el
// DefectAccess en el contexto y sigue la cadena (research.md sección 3).
func RequireDefectAccess(defects domain.DefectRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser un defecto, así que se responde como
		// inexistente sin consultar la base.
		defectID, err := uuid.Parse(c.Param("defectId"))
		if err != nil {
			abortDefectNotFound(c)
			return
		}

		// Una sola consulta resuelve la existencia del defecto, la membresía vigente y el estado del
		// proyecto; cualquier otro error es 500 genérico, registrado en el log.
		access, err := defects.FindAccessible(c.Request.Context(), defectID, userID)
		if errors.Is(err, domain.ErrDefectNotFound) {
			abortDefectNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar el acceso al defecto", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(contextKeyDefectAccess, access)
		c.Next()
	}
}

// abortDefectNotFound es la única salida 404 del middleware, para que el defecto inexistente, el
// ajeno y el identificador mal formado produzcan respuestas byte a byte idénticas (FR-053, SC-017).
func abortDefectNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgDefectNotFound})
}

// defectAccessFromContext devuelve el DefectAccess que RequireDefectAccess dejó en el contexto de gin,
// y si estaba presente.
func defectAccessFromContext(c *gin.Context) (*domain.DefectAccess, bool) {
	value, exists := c.Get(contextKeyDefectAccess)
	if !exists {
		return nil, false
	}
	access, ok := value.(*domain.DefectAccess)
	return access, ok
}

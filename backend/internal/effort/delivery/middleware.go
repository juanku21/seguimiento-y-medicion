package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// Mensajes del middleware de acceso a un registro de esfuerzo: el 401 es el mismo de la 001 y el 404
// es el único del contrato para el registro inexistente, ajeno o con identificador mal formado.
const (
	msgSessionRequired = "Se requiere iniciar sesión."
	msgEntryNotFound   = "El registro de esfuerzo no existe."
)

// entryAccessContextKey es la clave con la que RequireEffortEntryAccess deja en el contexto de gin el
// EntryAccess de quien pide sobre el registro de la ruta.
const entryAccessContextKey = "effort.entryAccess"

// RequireEffortEntryAccess exige que el titular de la sesión sea integrante vigente del proyecto del
// registro de esfuerzo de la ruta (FR-057, FR-059). Sin titular en el contexto corta con el 401 de la
// 001. El registro inexistente, el de un proyecto ajeno, el propio de una persona quitada y el
// identificador mal formado comparten una única salida 404 con el mismo cuerpo, para que sean
// indistinguibles (SC-018). Si hay acceso, deja el EntryAccess en el contexto y sigue la cadena; la
// autoría la decide el servicio (research.md sección 3).
func RequireEffortEntryAccess(entries domain.EffortRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := authdelivery.CurrentUserID(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, authdelivery.ErrorResponse{Error: msgSessionRequired})
			return
		}

		// Un identificador que no es UUID no puede ser un registro, así que se responde como
		// inexistente sin consultar la base.
		entryID, err := uuid.Parse(c.Param("entryId"))
		if err != nil {
			abortEntryNotFound(c)
			return
		}

		// Una sola consulta resuelve la existencia del registro, la membresía vigente y el estado del
		// proyecto; cualquier otro error es 500 genérico, registrado en el log.
		access, err := entries.FindAccessible(c.Request.Context(), entryID, userID)
		if errors.Is(err, domain.ErrEntryNotFound) {
			abortEntryNotFound(c)
			return
		}
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "no se pudo verificar el acceso al registro de esfuerzo", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, authdelivery.ErrorResponse{Error: msgInternalError})
			return
		}

		c.Set(entryAccessContextKey, access)
		c.Next()
	}
}

// abortEntryNotFound es la única salida 404 del middleware, para que el registro inexistente, el
// ajeno y el identificador mal formado produzcan respuestas byte a byte idénticas (FR-059, SC-018).
func abortEntryNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, authdelivery.ErrorResponse{Error: msgEntryNotFound})
}

// entryAccessFromContext devuelve el EntryAccess que RequireEffortEntryAccess dejó en el contexto de
// gin, y si estaba presente.
func entryAccessFromContext(c *gin.Context) (*domain.EntryAccess, bool) {
	value, exists := c.Get(entryAccessContextKey)
	if !exists {
		return nil, false
	}
	access, ok := value.(*domain.EntryAccess)
	return access, ok
}

// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"
)

// contextKeySub es la clave con la que el middleware de sesión deja en el contexto de gin el
// identificador del titular de la sesión, tomado del claim sub del JWT, para que toda acción
// protegida se le atribuya (FR-021, RN-09).
const contextKeySub = "auth.sub"

// contextKeyClaims es la clave con la que el middleware de sesión deja en el contexto de gin los
// claims completos de la sesión verificada, para que el cierre de sesión revoque justo la fila de
// su jti (FR-024, FR-026).
const contextKeyClaims = "auth.claims"

// msgSessionRequired es el texto del 401 de sesión inválida que fija la respuesta SesionInvalida
// de contracts/openapi.yaml. Es el mismo para sesión ausente, vencida y adulterada (FR-015, FR-016).
const msgSessionRequired = "Se requiere iniciar sesión."

// bearerPrefix es el esquema que debe tener la cabecera Authorization de una petición protegida.
const bearerPrefix = "Bearer "

// requireSession exige una sesión vigente antes de ejecutar la acción protegida (FR-015, FR-016,
// FR-020). Toma el token de la cabecera Authorization con esquema Bearer y lo valida con el
// servicio; si falta o es inválido, corta la cadena con el mismo 401 en todos los casos, sin
// ejecutar la acción. Un fallo real del repositorio corta con 500 para no confundirlo con la falta
// de sesión (Principio VII). Con la sesión vigente, deja el sub en el contexto para la atribución
// (FR-021, RN-09).
func (h *Handler) requireSession(c *gin.Context) {
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), bearerPrefix)
	if !ok || strings.TrimSpace(token) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: msgSessionRequired})
		return
	}

	claims, err := h.auth.ValidateSession(c.Request.Context(), token)
	if errors.Is(err, domain.ErrInvalidSession) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: msgSessionRequired})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "no se pudo validar la sesión", "error", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Error: msgInternalError})
		return
	}

	c.Set(contextKeySub, claims.UserID)
	c.Set(contextKeyClaims, claims)
	c.Next()
}

// userIDFromContext devuelve el identificador que el middleware de sesión dejó en el contexto bajo
// contextKeySub, y si estaba presente, para que el manejador protegido le atribuya la acción a su
// titular (FR-021, RN-09).
func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(contextKeySub)
	if !exists {
		return uuid.UUID{}, false
	}
	id, ok := value.(uuid.UUID)
	return id, ok
}

// claimsFromContext devuelve los claims de la sesión que el middleware dejó en el contexto bajo
// contextKeyClaims, y si estaban presentes, para que el cierre actúe sobre la sesión de la petición.
func claimsFromContext(c *gin.Context) (service.TokenClaims, bool) {
	value, exists := c.Get(contextKeyClaims)
	if !exists {
		return service.TokenClaims{}, false
	}
	claims, ok := value.(service.TokenClaims)
	return claims, ok
}

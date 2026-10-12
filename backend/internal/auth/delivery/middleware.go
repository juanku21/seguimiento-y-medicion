// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// contextKeySub es la clave con la que el middleware de sesión deja en el contexto de gin el
// identificador del titular de la sesión, tomado del claim sub del JWT, para que toda acción
// protegida se le atribuya (FR-021, RN-09).
const contextKeySub = "auth.sub"

// userIDFromContext devuelve el identificador que el middleware de sesión dejó en el contexto bajo
// contextKeySub, y si estaba presente, para que el manejador protegido le atribuya la acción a su
// titular (FR-021, RN-09). Firma mínima de T054; T060 la completa.
func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	return uuid.UUID{}, false
}

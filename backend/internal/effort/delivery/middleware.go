package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// RequireEffortEntryAccess exige que el titular de la sesión sea integrante vigente del proyecto del
// registro de esfuerzo de la ruta (FR-057, FR-059). Firma mínima de T043: la implementación, con la
// única salida 404 y el EntryAccess en el contexto, corresponde a T051.
func RequireEffortEntryAccess(entries domain.EffortRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// entryAccessFromContext devuelve el EntryAccess que RequireEffortEntryAccess dejó en el contexto de
// gin, y si estaba presente. Firma mínima de T043: la implementación corresponde a T051.
func entryAccessFromContext(c *gin.Context) (*domain.EntryAccess, bool) {
	return nil, false
}

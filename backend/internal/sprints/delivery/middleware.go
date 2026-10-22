package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// RequireSprintAccess exige que el titular de la sesión sea integrante del proyecto del sprint de la
// ruta y deja el SprintAccess en el contexto (research.md sección 3). Firma mínima creada por las
// pruebas de T026; se implementa en T032.
func RequireSprintAccess(sprints domain.SprintRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// sprintAccessFromContext devuelve el SprintAccess que RequireSprintAccess dejó en el contexto, y si
// estaba presente. Firma mínima creada por las pruebas de T026; se implementa en T032.
func sprintAccessFromContext(c *gin.Context) (*domain.SprintAccess, bool) {
	return nil, false
}

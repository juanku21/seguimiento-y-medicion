package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// RequireMembership exige que el titular de la sesión sea integrante del proyecto de la ruta
// (FR-049, RN-03). Firma mínima de T032: la implementa T036.
func RequireMembership(members domain.ProjectMemberRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// membershipFromContext devuelve la membresía que RequireMembership dejó en el contexto, y si
// estaba presente. Firma mínima de T032: la implementa T036.
func membershipFromContext(c *gin.Context) (*domain.ProjectMember, bool) {
	return nil, false
}

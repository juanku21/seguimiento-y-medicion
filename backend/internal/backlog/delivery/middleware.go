package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// RequireStoryAccess exige que el titular de la sesión sea integrante del proyecto de la historia de
// la ruta y deja en el contexto su StoryAccess (research.md sección 3). Firma mínima.
func RequireStoryAccess(stories domain.StoryRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// storyAccessFromContext devuelve el StoryAccess que RequireStoryAccess dejó en el contexto, y si
// estaba presente. Firma mínima.
func storyAccessFromContext(c *gin.Context) (*domain.StoryAccess, bool) {
	return nil, false
}

package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// RegisterRoutes registra las rutas de colección del backlog en el subgrupo
// /api/v1/projects/:projectId que devuelve projectsdelivery.RegisterRoutes, recibido desde cmd/app
// para no importar ese paquete. El subgrupo ya exige sesión y RequireMembership, así que quien no es
// integrante recibe el 404 indistinguible antes de llegar al manejador (FR-045, FR-047).
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/stories", handler.createStory)
	projectGroup.GET("/stories", handler.listStories)
}

// RegisterStoryRoutes crea el grupo plano /api/v1/stories/:storyId con el middleware de sesión
// inyectado desde cmd/app y RequireStoryAccess sobre el repositorio de historias, y registra en él
// las rutas de ítem de la historia (research.md sección 2).
func RegisterStoryRoutes(router *gin.Engine, requireSession gin.HandlerFunc, stories domain.StoryRepository, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, RequireStoryAccess(stories))
	storyGroup.GET("", handler.getStory)
	storyGroup.PUT("/story-points", handler.estimateStory)
}

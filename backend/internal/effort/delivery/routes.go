package delivery

import (
	"github.com/gin-gonic/gin"
)

// RegisterStoryRoutes registra las rutas de esfuerzo sobre una historia en el grupo plano
// /api/v1/stories/:storyId, con el middleware de sesión y el RequireStoryAccess de la 003 recibidos
// desde cmd/app para no importar backlog/delivery. La historia inexistente, ajena o con
// identificador mal formado recibe el mismo 404 que en la 003, antes de llegar al manejador.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, requireStoryAccess)
	storyGroup.POST("/effort-entries", handler.createEffortEntry)
}

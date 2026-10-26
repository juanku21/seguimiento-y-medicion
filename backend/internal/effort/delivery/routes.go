package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// RegisterStoryRoutes registra las rutas de esfuerzo sobre una historia en el grupo plano
// /api/v1/stories/:storyId, con el middleware de sesión y el RequireStoryAccess de la 003 recibidos
// desde cmd/app para no importar backlog/delivery. La historia inexistente, ajena o con
// identificador mal formado recibe el mismo 404 que en la 003, antes de llegar al manejador.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, requireStoryAccess)
	storyGroup.POST("/effort-entries", handler.createEffortEntry)
	storyGroup.GET("/effort", handler.getStoryEffort)
}

// RegisterEntryRoutes registra la modificación y la eliminación de un registro de esfuerzo en el
// grupo plano /api/v1/effort-entries/:entryId, con el middleware de sesión y el
// RequireEffortEntryAccess sobre el repositorio recibido. El registro inexistente, ajeno o con
// identificador mal formado recibe el 404 indistinguible del middleware antes de llegar al manejador.
func RegisterEntryRoutes(router *gin.Engine, requireSession gin.HandlerFunc, entries domain.EffortRepository, handler *Handler) {
	entryGroup := router.Group("/api/v1/effort-entries/:entryId", requireSession, RequireEffortEntryAccess(entries))
	entryGroup.PATCH("", handler.updateEffortEntry)
	entryGroup.DELETE("", handler.deleteEffortEntry)
}

package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// RegisterStoryRoutes registra la apertura de la sesión de estimación en el grupo plano
// /api/v1/stories/:storyId, con el middleware de sesión de la 001 y el RequireStoryAccess de la 003
// recibidos desde cmd/app para no importar backlog/delivery (research.md sección 3). La historia
// inexistente, ajena o con identificador mal formado recibe el mismo 404 que en la 003, antes de
// llegar al manejador.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, requireStoryAccess)
	storyGroup.POST("/estimation-sessions", handler.openEstimationSession)
}

// RegisterSessionRoutes crea el grupo plano /api/v1/estimation-sessions/:sessionId con el middleware
// de sesión inyectado desde cmd/app y RequireEstimationAccess sobre el repositorio recibido, y
// registra en él el canal de eventos y el voto. Quien no es integrante del proyecto de la sesión recibe el 404
// indistinguible del middleware antes de llegar al manejador.
func RegisterSessionRoutes(router *gin.Engine, requireSession gin.HandlerFunc, repo domain.SessionRepository, handler *Handler) {
	sessionGroup := router.Group("/api/v1/estimation-sessions/:sessionId", requireSession, RequireEstimationAccess(repo))
	sessionGroup.GET("/events", handler.streamEstimationSession)
	sessionGroup.PUT("/vote", handler.castEstimationVote)
}

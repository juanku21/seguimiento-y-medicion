package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// RegisterRoutes registra las rutas de colección de sprints en el subgrupo
// /api/v1/projects/:projectId que devuelve projectsdelivery.RegisterRoutes, recibido desde cmd/app
// para no importar ese paquete. El subgrupo ya exige sesión y RequireMembership, así que quien no es
// integrante recibe el 404 indistinguible antes de llegar al manejador.
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/sprints", handler.createSprint)
}

// RegisterSprintRoutes crea el grupo plano /api/v1/sprints/:sprintId con el middleware de sesión
// inyectado desde cmd/app y RequireSprintAccess sobre el repositorio de sprints, y registra en él las
// rutas de ítem del sprint (research.md sección 2). Quien no es integrante del proyecto del sprint
// recibe el 404 indistinguible del middleware antes de llegar al manejador (FR-058).
func RegisterSprintRoutes(router *gin.Engine, requireSession gin.HandlerFunc, sprints domain.SprintRepository, handler *Handler) {
	sprintGroup := router.Group("/api/v1/sprints/:sprintId", requireSession, RequireSprintAccess(sprints))
	sprintGroup.GET("", handler.getSprint)
	sprintGroup.POST("/stories", handler.assignSprintStories)
	sprintGroup.DELETE("/stories", handler.removeSprintStories)
	sprintGroup.POST("/start", handler.startSprint)
	sprintGroup.POST("/close", handler.closeSprint)
}

// RegisterStoryRoutes registra las rutas de sprints sobre una historia en el grupo plano
// /api/v1/stories/:storyId, con el middleware de sesión y el RequireStoryAccess de la 003 recibidos
// desde cmd/app para no importar backlog/delivery (research.md sección 2). La historia inexistente,
// ajena o con identificador mal formado recibe el mismo 404 que en la 003, antes de llegar al
// manejador (FR-058).
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, requireStoryAccess)
	storyGroup.PUT("/status", handler.changeStoryStatus)
}

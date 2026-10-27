package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// RegisterProjectRoutes registra POST y GET /defects sobre el subgrupo /api/v1/projects/:projectId de la
// 002, recibido desde cmd/app para no importar ese paquete. El subgrupo ya exige sesión y membresía.
func RegisterProjectRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/defects", handler.createDefect)
	projectGroup.GET("/defects", handler.listProjectDefects)
}

// RegisterStoryRoutes registra GET /api/v1/stories/:storyId/defects con el middleware de sesión y el
// RequireStoryAccess de la 003, recibidos desde cmd/app para no importar backlog/delivery.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
	storyGroup := router.Group("/api/v1/stories/:storyId", requireSession, requireStoryAccess)
	storyGroup.GET("/defects", handler.listStoryDefects)
}

// RegisterDefectRoutes registra el detalle, GET "", la modificación, PATCH "", y el cambio de estado, PUT /status, en el grupo
// plano /api/v1/defects/:defectId, con el middleware de sesión y RequireDefectAccess sobre el
// repositorio recibido. El defecto inexistente, ajeno o con identificador mal formado recibe el 404
// indistinguible del middleware antes de llegar al manejador.
func RegisterDefectRoutes(router *gin.Engine, requireSession gin.HandlerFunc, defects domain.DefectRepository, handler *Handler) {
	defectGroup := router.Group("/api/v1/defects/:defectId", requireSession, RequireDefectAccess(defects))
	defectGroup.GET("", handler.getDefect)
	defectGroup.PATCH("", handler.updateDefect)
	defectGroup.PUT("/status", handler.changeDefectStatus)
}

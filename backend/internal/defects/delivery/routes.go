package delivery

import "github.com/gin-gonic/gin"

// RegisterProjectRoutes registra POST /defects sobre el subgrupo /api/v1/projects/:projectId de la
// 002, recibido desde cmd/app para no importar ese paquete. El subgrupo ya exige sesión y membresía.
func RegisterProjectRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/defects", handler.createDefect)
}

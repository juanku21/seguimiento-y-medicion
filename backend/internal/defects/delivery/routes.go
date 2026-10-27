package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// RegisterProjectRoutes registra POST /defects sobre el subgrupo /api/v1/projects/:projectId de la
// 002, recibido desde cmd/app para no importar ese paquete. El subgrupo ya exige sesión y membresía.
func RegisterProjectRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/defects", handler.createDefect)
}

// RegisterDefectRoutes registra las rutas de un defecto en el grupo plano /api/v1/defects/:defectId,
// con el middleware de sesión y RequireDefectAccess. Firma mínima de T028: el registro de GET "" y
// PUT /status corresponde a T036.
func RegisterDefectRoutes(router *gin.Engine, requireSession gin.HandlerFunc, defects domain.DefectRepository, handler *Handler) {
}

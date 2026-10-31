package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes registra las rutas de reportes en el subgrupo /api/v1/projects/:projectId, que ya
// exige sesión y membresía vigente; lo recibe como parámetro para no importar projects/delivery.
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/reports/sprints/:sprintId", handler.generateSprintReport)
	projectGroup.POST("/reports/project", handler.generateProjectReport)
}

package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes registra las rutas de métricas en el subgrupo /api/v1/projects/:projectId, que ya
// exige sesión y membresía vigente (firma mínima, pendiente de T015).
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.GET("/metrics/sprints/:sprintId", handler.getSprintMetrics)
}

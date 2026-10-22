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
// rutas de ítem del sprint (research.md sección 2). Firma mínima creada por las pruebas de T028; se
// implementa en T037.
func RegisterSprintRoutes(router *gin.Engine, requireSession gin.HandlerFunc, sprints domain.SprintRepository, handler *Handler) {
}

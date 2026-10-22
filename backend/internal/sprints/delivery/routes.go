package delivery

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registra las rutas de colección de sprints en el subgrupo
// /api/v1/projects/:projectId que devuelve projectsdelivery.RegisterRoutes, recibido desde cmd/app
// para no importar ese paquete. El subgrupo ya exige sesión y RequireMembership, así que quien no es
// integrante recibe el 404 indistinguible antes de llegar al manejador.
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
	projectGroup.POST("/sprints", handler.createSprint)
}

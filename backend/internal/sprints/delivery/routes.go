package delivery

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registra las rutas de colección de sprints en el subgrupo
// /api/v1/projects/:projectId que devuelve projectsdelivery.RegisterRoutes, recibido desde cmd/app
// para no importar ese paquete. Firma mínima de T013: el registro de POST /sprints corresponde a
// T020.
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
}

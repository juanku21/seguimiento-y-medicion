package delivery

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registra las rutas de colección del backlog en el subgrupo
// /api/v1/projects/:projectId que devuelve projectsdelivery.RegisterRoutes, recibido desde cmd/app
// para no importar ese paquete. Firma mínima para las pruebas de integración de T015; el registro de
// POST /stories corresponde a T024.
func RegisterRoutes(projectGroup *gin.RouterGroup, handler *Handler) {
}

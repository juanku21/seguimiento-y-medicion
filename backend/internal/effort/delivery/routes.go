package delivery

import (
	"github.com/gin-gonic/gin"
)

// RegisterStoryRoutes registra las rutas de esfuerzo sobre una historia en el grupo plano
// /api/v1/stories/:storyId, con el middleware de sesión y el RequireStoryAccess de la 003 recibidos
// desde cmd/app. Firma mínima de T015: la implementación corresponde a T030.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
}

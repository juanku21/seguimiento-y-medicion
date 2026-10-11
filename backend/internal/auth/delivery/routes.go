// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 y registra en él las rutas de la feature con los
// manejadores recibidos. El registro es público: no pasa por el middleware de sesión.
func RegisterRoutes(router *gin.Engine, handler *Handler) {
	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.registerUser)
}

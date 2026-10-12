// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 y registra en él las rutas de la feature con los
// manejadores recibidos. El registro y el inicio de sesión son públicos: no pasan por el middleware
// de sesión.
func RegisterRoutes(router *gin.Engine, handler *Handler) {
	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.registerUser)
	api.POST("/auth/login", handler.loginUser)
}

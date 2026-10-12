// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 y registra en él las rutas de la feature con los
// manejadores recibidos. El registro y el inicio de sesión son públicos: no pasan por el middleware
// de sesión. La consulta del propio perfil se cuelga del grupo protegido, que aplica el middleware
// de sesión, así que exige sesión vigente (RN-08, FR-019). Devuelve ese grupo para que las
// historias siguientes registren en él sus propios endpoints.
func RegisterRoutes(router *gin.Engine, handler *Handler) *gin.RouterGroup {
	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.registerUser)
	api.POST("/auth/login", handler.loginUser)
	protected := api.Group("", handler.requireSession)
	protected.GET("/users/me", handler.getOwnProfile)
	return protected
}

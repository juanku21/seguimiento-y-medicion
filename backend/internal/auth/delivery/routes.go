// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 y registra en él las rutas de la feature con los
// manejadores recibidos. El registro y el inicio de sesión son públicos: no pasan por el middleware
// de sesión. Devuelve el grupo de rutas protegidas, sobre el que las historias siguientes registran
// sus propios endpoints: el grupo aplica el middleware de sesión, así que toda ruta que se cuelgue
// de él exige sesión vigente (RN-08, FR-019).
func RegisterRoutes(router *gin.Engine, handler *Handler) *gin.RouterGroup {
	api := router.Group("/api/v1")
	api.POST("/auth/register", handler.registerUser)
	api.POST("/auth/login", handler.loginUser)
	return api.Group("", handler.requireSession)
}

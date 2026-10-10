// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 sobre el que se registran las rutas públicas y
// protegidas de la feature. Todavía está vacío: cada historia de usuario agrega aquí sus rutas.
func RegisterRoutes(router *gin.Engine) {
	router.Group("/api/v1")
}

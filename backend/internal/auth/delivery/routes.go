// Package delivery expone por HTTP la feature de autenticación y cuentas de usuario.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo versionado /api/v1 sobre el que se registran las rutas públicas y
// protegidas de la feature con los manejadores recibidos. Todavía está vacío: cada historia de
// usuario agrega aquí sus rutas (POST /api/v1/auth/register lo registra T034).
func RegisterRoutes(router *gin.Engine, handler *Handler) {
	router.Group("/api/v1")
}

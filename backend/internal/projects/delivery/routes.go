// Package delivery expone por HTTP el módulo de proyectos y sus integrantes.
package delivery

import "github.com/gin-gonic/gin"

// RegisterRoutes crea el grupo /api/v1/projects con el middleware de sesión recibido, de modo que
// ninguna ruta del módulo sea pública (FR-046). El middleware llega inyectado como gin.HandlerFunc
// desde cmd/app para que este paquete no lo importe (research.md sección 10). Recibe los manejadores
// del módulo, registra la creación y el listado de proyectos en la raíz del grupo y devuelve el grupo para que
// las historias siguientes cuelguen de él sus endpoints.
func RegisterRoutes(router *gin.Engine, requireSession gin.HandlerFunc, handler *Handler) *gin.RouterGroup {
	projects := router.Group("/api/v1/projects", requireSession)
	projects.POST("", handler.createProject)
	projects.GET("", handler.listProjects)
	return projects
}

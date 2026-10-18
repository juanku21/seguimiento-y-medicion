// Package delivery expone por HTTP el módulo de proyectos y sus integrantes.
package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// RegisterRoutes crea el grupo /api/v1/projects con el middleware de sesión recibido, de modo que
// ninguna ruta del módulo sea pública (FR-046). El middleware llega inyectado como gin.HandlerFunc
// desde cmd/app para que este paquete no lo importe (research.md sección 10). Registra la creación y
// el listado de proyectos en la raíz del grupo y devuelve el subgrupo /api/v1/projects/:projectId,
// protegido además por RequireMembership sobre el repositorio de integrantes recibido, para que las
// historias siguientes cuelguen de él las rutas de un proyecto concreto (FR-049, research.md
// sección 5).
func RegisterRoutes(router *gin.Engine, requireSession gin.HandlerFunc, handler *Handler, members domain.ProjectMemberRepository) *gin.RouterGroup {
	projects := router.Group("/api/v1/projects", requireSession)
	projects.POST("", handler.createProject)
	projects.GET("", handler.listProjects)
	return projects.Group("/:projectId", RequireMembership(members))
}

// Package delivery expone por HTTP el módulo de proyectos y sus integrantes.
package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// RegisterRoutes crea el grupo /api/v1/projects con el middleware de sesión recibido, de modo que
// ninguna ruta del módulo sea pública (FR-046). El middleware llega inyectado como gin.HandlerFunc
// desde cmd/app para que este paquete no lo importe (research.md sección 10). Registra la creación y
// el listado de proyectos en la raíz del grupo y devuelve dos subgrupos de
// /api/v1/projects/:projectId: el del proyecto, protegido por RequireMembership sobre el repositorio
// de integrantes recibido (FR-049, research.md sección 5), y el de administración, que cuelga de él
// y agrega RequireOwnership sobre el repositorio de proyectos para las acciones reservadas al
// propietario (FR-047, FR-050). Al encadenarse después de RequireMembership, quien no es integrante
// recibe el 404 antes de que se evalúe el 403 (SC-006).
func RegisterRoutes(router *gin.Engine, requireSession gin.HandlerFunc, handler *Handler, members domain.ProjectMemberRepository, projects domain.ProjectRepository) (projectGroup, adminGroup *gin.RouterGroup) {
	group := router.Group("/api/v1/projects", requireSession)
	group.POST("", handler.createProject)
	group.GET("", handler.listProjects)
	projectGroup = group.Group("/:projectId", RequireMembership(members))
	return projectGroup, projectGroup.Group("", RequireOwnership(projects))
}

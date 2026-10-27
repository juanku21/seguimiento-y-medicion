package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// RequireDefectAccess exige que el titular de la sesión sea integrante vigente del proyecto del
// defecto de la ruta (FR-051, FR-053, FR-055). Firma mínima de T027: la implementación, con la única
// salida 404 y el DefectAccess en el contexto, corresponde a T034.
func RequireDefectAccess(defects domain.DefectRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// defectAccessFromContext devuelve el DefectAccess que RequireDefectAccess dejó en el contexto de gin,
// y si estaba presente. Firma mínima de T027: la implementación corresponde a T034.
func defectAccessFromContext(c *gin.Context) (*domain.DefectAccess, bool) {
	return nil, false
}

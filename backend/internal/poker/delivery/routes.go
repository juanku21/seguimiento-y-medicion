package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// RegisterStoryRoutes registra las rutas de Planning Poker que cuelgan de una historia, detrás de la
// sesión de la 001 y del RequireStoryAccess de la 003 recibido como gin.HandlerFunc (research.md
// sección 3). Firma mínima de T015: la implementa T027.
func RegisterStoryRoutes(router *gin.Engine, requireSession, requireStoryAccess gin.HandlerFunc, handler *Handler) {
}

// RegisterSessionRoutes registra el grupo plano /api/v1/estimation-sessions/:sessionId detrás de la
// sesión de la 001 y de RequireEstimationAccess sobre el repositorio recibido. Firma mínima de T015:
// la implementa T027.
func RegisterSessionRoutes(router *gin.Engine, requireSession gin.HandlerFunc, repo domain.SessionRepository, handler *Handler) {
}

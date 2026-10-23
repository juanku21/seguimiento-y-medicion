// Package delivery expone por HTTP el módulo de Planning Poker: middleware de acceso, manejadores
// REST y canal de eventos.
package delivery

import (
	"github.com/gin-gonic/gin"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// RequireEstimationAccess exige que el titular de la sesión sea integrante vigente del proyecto de
// la sesión de estimación de la ruta, con una única salida 404 (RN-01; research.md sección 4).
// Firma mínima de T014: la implementa T024.
func RequireEstimationAccess(sessions domain.SessionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// sessionAccessFromContext devuelve el SessionAccess que RequireEstimationAccess dejó en el
// contexto, y si estaba presente. Firma mínima de T014: la implementa T024.
func sessionAccessFromContext(c *gin.Context) (*domain.SessionAccess, bool) {
	return nil, false
}

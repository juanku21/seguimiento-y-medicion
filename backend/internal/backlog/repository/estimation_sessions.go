package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EstimationSessions implementa domain.EstimationSessions mientras la feature
// specs/005-planning-poker no exista (research.md sección 11). Cuando esa feature llegue, reemplaza
// el cuerpo de este archivo con la lectura real de las sesiones de Planning Poker, Finalizadas y
// Canceladas, sin tocar backlog/service.
type EstimationSessions struct{}

// NewEstimationSessions crea el puerto de sesiones de estimación sobre la conexión recibida. Firma
// mínima de la T086: todavía no guarda la conexión ni consulta ninguna tabla (lo implementa la T088).
func NewEstimationSessions(_ *gorm.DB) *EstimationSessions {
	return &EstimationSessions{}
}

// HasSessions informa que la historia no tiene sesiones de estimación. No es un dato inventado: en
// el estado actual del producto no existe Planning Poker.
func (s *EstimationSessions) HasSessions(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

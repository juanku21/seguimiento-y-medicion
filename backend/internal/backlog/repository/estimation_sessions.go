package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// EstimationSessions implementa domain.EstimationSessions leyendo las sesiones de Planning Poker de
// specs/005-planning-poker (research.md sección 15.1).
type EstimationSessions struct {
	db *gorm.DB
}

// NewEstimationSessions crea el puerto de sesiones de estimación sobre la conexión recibida.
func NewEstimationSessions(db *gorm.DB) *EstimationSessions {
	return &EstimationSessions{db: db}
}

// HasSessions informa si la historia tiene al menos una sesión de estimación, en cualquier estado
// (open, finished o canceled), con un EXISTS sobre estimation_sessions por story_id (FR-089, RN-33).
func (s *EstimationSessions) HasSessions(ctx context.Context, storyID uuid.UUID) (bool, error) {
	db := s.db.WithContext(ctx)
	sessions := db.
		Model(&pokerdomain.EstimationSession{}).
		Select("1").
		Where("story_id = ?", storyID)
	var has bool
	if err := db.Raw("SELECT EXISTS (?)", sessions).Scan(&has).Error; err != nil {
		return false, fmt.Errorf("consultar las sesiones de estimación de la historia: %w", err)
	}
	return has, nil
}

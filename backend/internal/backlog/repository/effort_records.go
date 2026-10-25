package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// EffortRecords implementa domain.EffortRecords con la marca ever_had_effort de user_stories, que el
// alta de esfuerzo de la 006 deja en true y nunca vuelve a false (RN-22). Por eso una historia cuyos
// registros se borraron todos sigue sin poder eliminarse, sin tocar backlog/service.
type EffortRecords struct {
	db *gorm.DB
}

// NewEffortRecords crea el puerto de registros de esfuerzo sobre la conexión recibida.
func NewEffortRecords(db *gorm.DB) *EffortRecords {
	return &EffortRecords{db: db}
}

// HasEffort lee ever_had_effort de la historia. Una historia inexistente no tiene esfuerzo: la
// existencia la verifica el servicio antes de consultar este puerto.
func (r *EffortRecords) HasEffort(ctx context.Context, storyID uuid.UUID) (bool, error) {
	var flags []bool
	err := r.db.WithContext(ctx).
		Model(&domain.UserStory{}).
		Where("id = ?", storyID).
		Limit(1).
		Pluck("ever_had_effort", &flags).Error
	if err != nil {
		return false, fmt.Errorf("leer si la historia tuvo esfuerzo: %w", err)
	}
	return len(flags) > 0 && flags[0], nil
}

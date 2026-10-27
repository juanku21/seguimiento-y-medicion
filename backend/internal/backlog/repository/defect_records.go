package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// DefectRecords implementa domain.DefectRecords con la marca ever_had_defects de user_stories, que
// el alta de defectos de la 007 deja en true y que nunca vuelve a false, ni siquiera al mover el
// único defecto a otra historia. Por eso una historia que tuvo defectos no puede eliminarse, sin
// tocar backlog/service.
type DefectRecords struct {
	db *gorm.DB
}

// NewDefectRecords crea el puerto de registros de defectos sobre la conexión recibida.
func NewDefectRecords(db *gorm.DB) *DefectRecords {
	return &DefectRecords{db: db}
}

// HasDefects lee ever_had_defects de la historia. Una historia inexistente no tiene defectos: la
// existencia la verifica el servicio antes de consultar este puerto.
func (r *DefectRecords) HasDefects(ctx context.Context, storyID uuid.UUID) (bool, error) {
	var flags []bool
	err := r.db.WithContext(ctx).
		Model(&domain.UserStory{}).
		Where("id = ?", storyID).
		Limit(1).
		Pluck("ever_had_defects", &flags).Error
	if err != nil {
		return false, fmt.Errorf("leer si la historia tuvo defectos: %w", err)
	}
	return len(flags) > 0 && flags[0], nil
}

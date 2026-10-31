package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// GenerationRepository persiste el registro de auditoría de cada reporte entregado.
type GenerationRepository struct {
	db *gorm.DB
}

// NewGenerationRepository crea el repositorio del registro de generaciones sobre la conexión dada.
func NewGenerationRepository(db *gorm.DB) *GenerationRepository {
	return &GenerationRepository{db: db}
}

// Save inserta el registro de una generación.
func (r *GenerationRepository) Save(ctx context.Context, generation domain.ReportGeneration) error {
	if err := r.db.WithContext(ctx).Create(&generation).Error; err != nil {
		return fmt.Errorf("guardar el registro de generación: %w", err)
	}
	return nil
}

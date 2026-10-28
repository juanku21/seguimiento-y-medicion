// Package repository implementa la persistencia del módulo de métricas con GORM.
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// MetricsRepository lee los datos crudos de métricas (firma mínima, pendiente de T013).
type MetricsRepository struct {
	db *gorm.DB
}

// NewMetricsRepository crea el repositorio sobre la conexión indicada (firma mínima, pendiente de T013).
func NewMetricsRepository(db *gorm.DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

// readOnly ejecuta fn en una transacción REPEATABLE READ de solo lectura (firma mínima, pendiente de T013).
func (r *MetricsRepository) readOnly(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return errors.New("not implemented")
}

// LoadProject carga los datos del proyecto (firma mínima, pendiente de la historia que la usa).
func (r *MetricsRepository) LoadProject(ctx context.Context, projectID uuid.UUID) (*domain.ProjectData, error) {
	return nil, errors.New("not implemented")
}

// LoadSprint carga un sprint y el factor vigente del proyecto (firma mínima, pendiente de T013).
func (r *MetricsRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (*domain.SprintData, int, error) {
	return nil, 0, errors.New("not implemented")
}

// LoadActiveSprint carga el sprint Activo (firma mínima, pendiente de la historia que la usa).
func (r *MetricsRepository) LoadActiveSprint(ctx context.Context, projectID uuid.UUID) (*domain.SprintData, int, error) {
	return nil, 0, errors.New("not implemented")
}

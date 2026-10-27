// Package repository implementa la persistencia del módulo de defectos con GORM (Principio III).
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectRepository implementa domain.DefectRepository sobre la tabla defects. Firma mínima: las
// operaciones las implementan las tareas de cada historia.
type DefectRepository struct {
	db *gorm.DB
}

// NewDefectRepository crea el repositorio de defectos sobre la conexión recibida.
func NewDefectRepository(db *gorm.DB) *DefectRepository {
	return &DefectRepository{db: db}
}

// FindAccessible resuelve el acceso del usuario a un defecto.
func (r *DefectRepository) FindAccessible(_ context.Context, _, _ uuid.UUID) (*domain.DefectAccess, error) {
	return nil, errors.New("not implemented")
}

// Create da de alta un defecto en el proyecto.
func (r *DefectRepository) Create(_ context.Context, _, _ uuid.UUID, _ domain.DefectInput, _ time.Time) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// ChangeStatus aplica un cambio de estado.
func (r *DefectRepository) ChangeStatus(_ context.Context, _, _ uuid.UUID, _ domain.StatusChange, _ time.Time) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// Update aplica una modificación parcial de campos.
func (r *DefectRepository) Update(_ context.Context, _, _ uuid.UUID, _ domain.DefectPatch) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// FindView devuelve el detalle de un defecto con sus relaciones resueltas.
func (r *DefectRepository) FindView(_ context.Context, _ uuid.UUID) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// ListForProject lista los defectos de un proyecto según el filtro.
func (r *DefectRepository) ListForProject(_ context.Context, _ uuid.UUID, _ domain.DefectFilter) ([]domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

// ValidateFilter verifica que los identificadores del filtro pertenezcan al proyecto.
func (r *DefectRepository) ValidateFilter(_ context.Context, _ uuid.UUID, _ domain.DefectFilter) error {
	return errors.New("not implemented")
}

// HasProjectDefects indica si el proyecto tiene al menos un defecto.
func (r *DefectRepository) HasProjectDefects(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

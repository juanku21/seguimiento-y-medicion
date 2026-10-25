// Package repository implementa con GORM la persistencia del módulo de registro de esfuerzo
// (Principio III).
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// EffortRepository implementa domain.EffortRepository sobre la tabla effort_entries. Firma mínima de
// T015: la implementación corresponde a T026.
type EffortRepository struct{}

// NewEffortRepository compone el repositorio sobre la conexión recibida. Firma mínima de T015: la
// implementación corresponde a T026.
func NewEffortRepository(db *gorm.DB) *EffortRepository {
	return nil
}

// FindAccessible resuelve el acceso a un registro para RequireEffortEntryAccess. Firma mínima de
// T015: la implementación corresponde a T050.
func (r *EffortRepository) FindAccessible(ctx context.Context, entryID, userID uuid.UUID) (*domain.EntryAccess, error) {
	return nil, errors.New("not implemented")
}

// Create registra el alta de un registro de esfuerzo en una transacción. Firma mínima de T015: la
// implementación corresponde a T026.
func (r *EffortRepository) Create(ctx context.Context, storyID, authorID uuid.UUID, input domain.EntryInput, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// Update modifica un registro propio en una transacción. Firma mínima de T015: la implementación
// corresponde a T050.
func (r *EffortRepository) Update(ctx context.Context, entryID, actorID uuid.UUID, patch domain.EntryPatch, now time.Time, location *time.Location, grace time.Duration) (*domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// Delete elimina un registro propio en una transacción. Firma mínima de T015: la implementación
// corresponde a T050.
func (r *EffortRepository) Delete(ctx context.Context, entryID, actorID uuid.UUID, now time.Time, grace time.Duration) error {
	return errors.New("not implemented")
}

// ListForStory devuelve los registros de una historia. Firma mínima de T015: la implementación
// corresponde a T036.
func (r *EffortRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// ListForProject devuelve los registros de un proyecto según el filtro. Firma mínima de T015: la
// implementación corresponde a T059.
func (r *EffortRepository) ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) ([]domain.EntryView, error) {
	return nil, errors.New("not implemented")
}

// ValidateFilter verifica el filtro contra el proyecto. Firma mínima de T015: la implementación
// corresponde a T059.
func (r *EffortRepository) ValidateFilter(ctx context.Context, projectID uuid.UUID, filter domain.EntryFilter) error {
	return errors.New("not implemented")
}

// HasProjectEffort informa si el proyecto tiene algún registro de esfuerzo. Firma mínima de T015:
// la implementación corresponde a T059.
func (r *EffortRepository) HasProjectEffort(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

// ComparisonRows devuelve el factor vigente del proyecto y las filas de la comparación. Firma mínima
// de T015: la implementación corresponde a T067.
func (r *EffortRepository) ComparisonRows(ctx context.Context, projectID uuid.UUID, storyID *uuid.UUID) (int, []domain.ComparisonRow, error) {
	return 0, nil, errors.New("not implemented")
}

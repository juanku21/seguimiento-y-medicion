// Package repository persiste las entidades del módulo de sprints con GORM e implementa el puerto
// de esfuerzo que lee los registros de la feature 006.
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SprintEffort implementa domain.SprintEffort con un EXISTS sobre effort_entries.sprint_id, sin tocar
// sprints/service (research.md sección 16). La clave foránea de effort_entries hacia sprints.id es el
// respaldo: si un registro se ancla entre esta verificación y el borrado, SprintRepository.Delete
// traduce gorm.ErrForeignKeyViolated a domain.ErrSprintHasEffort para responder el mismo 409.
type SprintEffort struct {
	db *gorm.DB
}

// NewSprintEffort crea el puerto de esfuerzo de los sprints sobre la conexión recibida.
func NewSprintEffort(db *gorm.DB) *SprintEffort {
	return &SprintEffort{db: db}
}

// HasEffort informa si existe al menos un registro de esfuerzo anclado al sprint.
func (e *SprintEffort) HasEffort(ctx context.Context, sprintID uuid.UUID) (bool, error) {
	var exists bool
	err := e.db.WithContext(ctx).
		Raw("SELECT EXISTS (SELECT 1 FROM effort_entries WHERE sprint_id = ?)", sprintID).
		Scan(&exists).Error
	if err != nil {
		return false, fmt.Errorf("leer si el sprint tiene esfuerzo: %w", err)
	}
	return exists, nil
}

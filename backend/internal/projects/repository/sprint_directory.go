package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// SprintDirectory implementa domain.SprintDirectory sobre los sprints de la spec 004 (research.md
// sección 9). Por ahora guarda la conexión sin usarla: la lectura real del sprint activo la
// implementa la tarea T049 de specs/004-sprint-management, sin tocar projects/service.
type SprintDirectory struct {
	db *gorm.DB
}

// NewSprintDirectory crea el directorio de sprints sobre la conexión recibida, que es la misma
// sesión que usan los demás repositorios del módulo.
func NewSprintDirectory(db *gorm.DB) *SprintDirectory {
	return &SprintDirectory{db: db}
}

// FindActiveSprint todavía informa que ningún proyecto tiene sprint activo y devuelve (nil, nil),
// que FR-043 trata como un resultado válido y no como un error. T049 reemplaza este cuerpo por la
// lectura del sprint active del proyecto.
func (d *SprintDirectory) FindActiveSprint(_ context.Context, _ uuid.UUID) (*domain.ActiveSprint, error) {
	return nil, nil
}

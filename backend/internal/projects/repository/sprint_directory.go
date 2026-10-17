package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// SprintDirectory implementa domain.SprintDirectory mientras la feature
// specs/004-sprint-management no exista (research.md sección 9). Cuando esa feature llegue, reemplaza
// el cuerpo de este archivo con la lectura real del sprint activo, sin tocar projects/service.
type SprintDirectory struct{}

// NewSprintDirectory crea el directorio de sprints. No recibe dependencias porque hoy no consulta
// ninguna tabla.
func NewSprintDirectory() *SprintDirectory {
	return &SprintDirectory{}
}

// FindActiveSprint informa que ningún proyecto tiene sprint activo y devuelve (nil, nil). No es un
// dato inventado: en el estado actual del producto no existen sprints, y FR-043 manda tratar esa
// ausencia como un resultado válido y no como un error.
func (d *SprintDirectory) FindActiveSprint(_ context.Context, _ uuid.UUID) (*domain.ActiveSprint, error) {
	return nil, nil
}

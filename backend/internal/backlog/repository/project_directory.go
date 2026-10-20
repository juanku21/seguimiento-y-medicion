package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectDirectory implementa domain.ProjectDirectory leyendo solo la columna status de projects
// (research.md sección 14). Firma mínima para las pruebas de integración de T015; los campos y la
// implementación corresponden a T020.
type ProjectDirectory struct{}

// NewProjectDirectory crea el puerto de proyectos sobre la conexión recibida. Firma mínima.
func NewProjectDirectory(db *gorm.DB) *ProjectDirectory {
	return &ProjectDirectory{}
}

// IsFinished indica si el proyecto está Finalizado. Firma mínima; la implementación corresponde a
// T020.
func (d *ProjectDirectory) IsFinished(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

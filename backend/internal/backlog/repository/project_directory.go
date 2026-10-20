package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectDirectory implementa domain.ProjectDirectory leyendo solo la columna status de la tabla
// projects de la 002, sin depender de su ProjectRepository (research.md sección 14).
type ProjectDirectory struct {
	db *gorm.DB
}

// NewProjectDirectory crea el puerto de proyectos sobre la conexión recibida.
func NewProjectDirectory(db *gorm.DB) *ProjectDirectory {
	return &ProjectDirectory{db: db}
}

// IsFinished indica si el proyecto está Finalizado: selecciona únicamente projects.status y lo
// compara con ProjectStatusFinished. Un proyecto inexistente no tiene un estado que informar, así
// que se devuelve el error de la consulta (gorm.ErrRecordNotFound envuelto) en lugar de suponer un
// valor; en el flujo HTTP la membresía ya verificada por RequireMembership garantiza que existe.
func (d *ProjectDirectory) IsFinished(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var project projectsdomain.Project
	err := d.db.WithContext(ctx).
		Model(&projectsdomain.Project{}).
		Select("status").
		Where("id = ?", projectID).
		Take(&project).Error
	if err != nil {
		return false, fmt.Errorf("leer el estado del proyecto: %w", err)
	}
	return project.Status == projectsdomain.ProjectStatusFinished, nil
}

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// SprintDirectory implementa domain.SprintDirectory sobre los sprints de la spec 004 (research.md
// sección 17.1), leyendo el sprint active del proyecto sobre el modelo sprintsdomain.Sprint.
type SprintDirectory struct {
	db *gorm.DB
}

// NewSprintDirectory crea el directorio de sprints sobre la conexión recibida, que es la misma
// sesión que usan los demás repositorios del módulo.
func NewSprintDirectory(db *gorm.DB) *SprintDirectory {
	return &SprintDirectory{db: db}
}

// FindActiveSprint lee en una sola consulta el identificador y el nombre del sprint active del
// proyecto. Si no hay ninguno devuelve (nil, nil), que FR-043 trata como un resultado válido y no
// como un error; por eso se escanea a un slice en lugar de usar First, que lo informaría como
// registro inexistente.
func (d *SprintDirectory) FindActiveSprint(ctx context.Context, projectID uuid.UUID) (*domain.ActiveSprint, error) {
	var rows []domain.ActiveSprint
	err := d.db.WithContext(ctx).
		Model(&sprintsdomain.Sprint{}).
		Select("id, name").
		Where("project_id = ? AND status = ?", projectID, sprintsdomain.SprintStatusActive).
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

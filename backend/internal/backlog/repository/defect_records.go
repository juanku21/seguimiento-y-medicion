package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DefectRecords implementa domain.DefectRecords mientras la feature specs/007-defect-tracking no
// exista (research.md sección 11). Cuando esa feature llegue, reemplaza el cuerpo de este archivo
// con la lectura real de los defectos asociados, sin tocar backlog/service.
type DefectRecords struct {
	db *gorm.DB
}

// NewDefectRecords crea el puerto de registros de defectos sobre la conexión recibida.
func NewDefectRecords(db *gorm.DB) *DefectRecords {
	return &DefectRecords{db: db}
}

// HasDefects informa que la historia no tiene defectos asociados; todavía no lee
// user_stories.ever_had_defects (T022).
func (r *DefectRecords) HasDefects(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

package repository

import (
	"context"

	"github.com/google/uuid"
)

// DefectRecords implementa domain.DefectRecords mientras la feature specs/007-defect-tracking no
// exista (research.md sección 11). Cuando esa feature llegue, reemplaza el cuerpo de este archivo
// con la lectura real de los defectos asociados, sin tocar backlog/service.
type DefectRecords struct{}

// NewDefectRecords crea el puerto de registros de defectos. No recibe dependencias porque hoy no
// consulta ninguna tabla.
func NewDefectRecords() *DefectRecords {
	return &DefectRecords{}
}

// HasDefects informa que la historia no tiene defectos asociados. No es un dato inventado: en el
// estado actual del producto no existe el registro de defectos.
func (r *DefectRecords) HasDefects(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

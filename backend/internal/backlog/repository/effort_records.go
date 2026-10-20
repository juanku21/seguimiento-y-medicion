package repository

import (
	"context"

	"github.com/google/uuid"
)

// EffortRecords implementa domain.EffortRecords mientras la feature specs/006-effort-tracking no
// exista (research.md sección 11). Cuando esa feature llegue, reemplaza el cuerpo de este archivo
// con la lectura real del esfuerzo registrado, sin tocar backlog/service.
type EffortRecords struct{}

// NewEffortRecords crea el puerto de registros de esfuerzo. No recibe dependencias porque hoy no
// consulta ninguna tabla.
func NewEffortRecords() *EffortRecords {
	return &EffortRecords{}
}

// HasEffort informa que la historia no tiene esfuerzo registrado. No es un dato inventado: en el
// estado actual del producto no existe el registro de esfuerzo.
func (r *EffortRecords) HasEffort(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

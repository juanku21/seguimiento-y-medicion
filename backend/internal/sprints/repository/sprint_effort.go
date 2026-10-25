// Package repository persiste las entidades del módulo de sprints con GORM e implementa el puerto
// hacia la feature que todavía no existe.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SprintEffort implementa domain.SprintEffort mientras la feature specs/006-effort-tracking no
// exista (research.md sección 16). Cuando esa feature llegue, reemplaza el cuerpo de este archivo con
// la lectura real del esfuerzo registrado en el sprint, sin tocar sprints/service. Además, su tabla de
// registros de esfuerzo tendrá una clave foránea hacia sprints.id con RESTRICT: si un registro llega
// entre esta verificación y el borrado, el repositorio debe traducir gorm.ErrForeignKeyViolated a
// domain.ErrSprintHasEffort, para responder el mismo 409.
type SprintEffort struct{}

// NewSprintEffort crea el puerto de esfuerzo de los sprints. No recibe dependencias porque hoy no
// consulta ninguna tabla. Firma mínima de T017: la implementación corresponde a T031.
func NewSprintEffort(_ *gorm.DB) *SprintEffort {
	return &SprintEffort{}
}

// HasEffort informa que el sprint no tiene esfuerzo registrado. No es un dato inventado: en el estado
// actual del producto no existe el registro de esfuerzo.
func (e *SprintEffort) HasEffort(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, nil
}

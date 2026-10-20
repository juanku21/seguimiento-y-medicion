// Package repository persiste las entidades del módulo de backlog con GORM e implementa los puertos
// hacia las features que todavía no existen.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// SprintAssignments implementa domain.SprintAssignments mientras la feature
// specs/004-sprint-management no exista (research.md sección 11). Cuando esa feature llegue,
// reemplaza el cuerpo de este archivo con la lectura real de la asignación a sprints, sin tocar
// backlog/service.
type SprintAssignments struct{}

// NewSprintAssignments crea el puerto de asignaciones a sprints. No recibe dependencias porque hoy
// no consulta ninguna tabla.
func NewSprintAssignments() *SprintAssignments {
	return &SprintAssignments{}
}

// Find informa que la historia no está comprometida en un sprint abierto ni estuvo nunca en un
// sprint. No es un dato inventado: en el estado actual del producto no existen sprints.
func (a *SprintAssignments) Find(_ context.Context, _ uuid.UUID) (domain.SprintAssignment, error) {
	return domain.SprintAssignment{}, nil
}

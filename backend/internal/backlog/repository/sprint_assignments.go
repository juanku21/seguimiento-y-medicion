// Package repository persiste las entidades del módulo de backlog con GORM e implementa los puertos
// hacia las features que todavía no existen.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// SprintAssignments implementa domain.SprintAssignments leyendo la asignación de historias a
// sprints de specs/004-sprint-management (research.md secciones 9 y 17.2).
type SprintAssignments struct {
	db *gorm.DB
}

// NewSprintAssignments crea el puerto de asignaciones a sprints sobre la conexión recibida.
func NewSprintAssignments(db *gorm.DB) *SprintAssignments {
	return &SprintAssignments{db: db}
}

// Find informa si la historia está comprometida en un sprint abierto y si estuvo alguna vez en un
// sprint. Firma mínima de T029: la lectura real la implementa T033.
func (a *SprintAssignments) Find(_ context.Context, _ uuid.UUID) (domain.SprintAssignment, error) {
	return domain.SprintAssignment{}, nil
}

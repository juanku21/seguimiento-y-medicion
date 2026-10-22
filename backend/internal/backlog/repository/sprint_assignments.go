// Package repository persiste las entidades del módulo de backlog con GORM e implementa los puertos
// hacia las features que todavía no existen.
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
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
// sprint, en una sola consulta sobre user_stories: CommittedToOpenSprint es un EXISTS sobre los
// compromisos abiertos de sprint_stories (outcome IS NULL) y EverAssigned es la columna
// ever_committed (data-model.md sección 10). Una historia inexistente no tiene asignación que
// informar, así que se devuelve el error de la consulta (gorm.ErrRecordNotFound envuelto).
func (a *SprintAssignments) Find(ctx context.Context, storyID uuid.UUID) (domain.SprintAssignment, error) {
	db := a.db.WithContext(ctx)
	openCommitment := db.
		Model(&sprintsdomain.SprintStory{}).
		Select("1").
		Where("story_id = ? AND outcome IS NULL", storyID)
	var assignment domain.SprintAssignment
	err := db.
		Model(&domain.UserStory{}).
		Select("EXISTS (?) AS committed_to_open_sprint, ever_committed AS ever_assigned", openCommitment).
		Where("id = ?", storyID).
		Take(&assignment).Error
	if err != nil {
		return domain.SprintAssignment{}, fmt.Errorf("leer la asignación de la historia a sprints: %w", err)
	}
	return assignment, nil
}

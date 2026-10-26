package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// AnchoredSprint es el sprint asociado a un registro de esfuerzo, con lo necesario para decidir si el
// registro todavía es escribible y para mostrarlo en las listas. ClosedAt es nil mientras el sprint no
// está Cerrado (data-model.md secciones 6.2 y 8).
type AnchoredSprint struct {
	ID       uuid.UUID
	Name     string
	Status   sprintsdomain.SprintStatus
	ClosedAt *time.Time
}

// GraceOpen informa si el plazo de gracia de un sprint cerrado en closedAt sigue vigente en now: el
// plazo está vigente mientras now sea estrictamente anterior a closedAt + grace y se cuenta cumplido
// al alcanzarse (RN-23, data-model.md sección 6.2).
func GraceOpen(closedAt, now time.Time, grace time.Duration) bool {
	return now.Before(closedAt.Add(grace))
}

// EnsureEntryWritable decide si un registro de esfuerzo anclado a sprint todavía se puede modificar o
// eliminar en now con la tabla de data-model.md sección 6.2 (FR-020, FR-021, RN-09, RN-10).
func EnsureEntryWritable(sprint *AnchoredSprint, now time.Time, grace time.Duration) error {
	return errors.New("not implemented")
}

package domain

import (
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

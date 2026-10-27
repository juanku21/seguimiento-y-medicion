package domain

import (
	"errors"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EnsureSprintUsableForDetection verifica que un sprint pueda ser de detección (RN-08): solo
// los sprints activos o cerrados. Firma mínima: la lógica la aporta la tarea de implementación.
func EnsureSprintUsableForDetection(status sprintsdomain.SprintStatus) error {
	return errors.New("not implemented")
}

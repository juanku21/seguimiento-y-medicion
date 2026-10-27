package domain

import (
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EnsureSprintUsableForDetection verifica que un sprint pueda ser de detección (RN-08): solo
// los sprints activos o cerrados.
func EnsureSprintUsableForDetection(status sprintsdomain.SprintStatus) error {
	// Un sprint planificado todavía no se ejecutó, por lo que no puede detectar defectos.
	if status == sprintsdomain.SprintStatusPlanned {
		return ErrDetectionSprintPlanned
	}
	return nil
}

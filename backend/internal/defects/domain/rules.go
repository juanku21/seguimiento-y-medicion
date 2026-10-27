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

// EnsureDefectEditable verifica que un defecto admita la modificación de sus datos (FR-036, RN-22).
func EnsureDefectEditable(status DefectStatus) error {
	// Un defecto descartado es de solo lectura.
	if status == DefectStatusDiscarded {
		return ErrDefectDiscarded
	}
	return nil
}

// EnsureLinksCorrectable verifica que la historia relacionada y el sprint de detección de un defecto
// puedan corregirse (FR-038, RN-16).
func EnsureLinksCorrectable(status DefectStatus) error {
	// Los vínculos solo se corrigen mientras el defecto sigue abierto.
	if status != DefectStatusOpen {
		return ErrLinksFrozen
	}
	return nil
}

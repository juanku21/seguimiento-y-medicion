package domain

import (
	"errors"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EnsureReportable rechaza el sprint Planificado (RN-04); la implementación llega con T015.
func EnsureReportable(status sprintsdomain.SprintStatus) error {
	return errors.New("not implemented")
}

// IsPartial indica si el reporte incluye algún sprint Activo (RN-03); la implementación llega con T015.
func IsPartial(scope ReportScope, sprints []SprintRef) bool {
	return false
}

// SortStories ordena las historias in situ por prioridad, título y story_id (RN-22); la
// implementación llega con T015.
func SortStories(lines []StoryLine) {}

// StoryTitle devuelve el título de la instantánea si existe (nil si la historia no está en ella) y,
// si no, el vigente (FR-021); la implementación llega con T015.
func StoryTitle(snapshot *string, current string) string {
	return ""
}

// StoriesState calcula el estado de la sección de historias a partir de sus grupos (RN-11); la
// implementación llega con T015.
func StoriesState(groups []StoryGroup) SectionState {
	return ""
}

package domain

import (
	"bytes"
	"cmp"
	"slices"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// EnsureReportable rechaza el sprint Planificado porque todavía no tiene nada que reportar (RN-04);
// el sprint Activo y el Cerrado se aceptan.
func EnsureReportable(status sprintsdomain.SprintStatus) error {
	if status == sprintsdomain.SprintStatusPlanned {
		return ErrSprintNotStarted
	}
	return nil
}

// IsPartial indica si el reporte incluye algún sprint Activo, cuyos datos todavía pueden cambiar
// (RN-03). El alcance no altera la regla: aplica igual a un sprint y a todo el proyecto.
func IsPartial(scope ReportScope, sprints []SprintRef) bool {
	return slices.ContainsFunc(sprints, func(s SprintRef) bool {
		return s.Status == sprintsdomain.SprintStatusActive
	})
}

// SortStories ordena las historias in situ por prioridad (high, medium, low), después por título y
// por último por story_id, para que el orden sea estable (RN-22).
func SortStories(lines []StoryLine) {
	slices.SortStableFunc(lines, func(a, b StoryLine) int {
		return cmp.Or(
			cmp.Compare(a.Priority.Rank(), b.Priority.Rank()),
			cmp.Compare(a.Title, b.Title),
			bytes.Compare(a.StoryID[:], b.StoryID[:]),
		)
	})
}

// StoryTitle devuelve el título de la instantánea si la historia está en ella (puntero no nulo) y,
// si no, el título vigente (FR-021).
func StoryTitle(snapshot *string, current string) string {
	if snapshot != nil {
		return *snapshot
	}
	return current
}

// StoriesState devuelve la sección de historias vacía si ningún grupo tiene historias y completa en
// caso contrario (RN-11).
func StoriesState(groups []StoryGroup) SectionState {
	for _, g := range groups {
		if len(g.Stories) > 0 {
			return SectionComplete
		}
	}
	return SectionEmpty
}

// ReportableSprints filtra los sprints Activo y Cerrado y conserva el orden recibido (RN-05, RN-07).
func ReportableSprints(sprints []SprintRef) []SprintRef {
	result := []SprintRef{}
	for _, s := range sprints {
		if EnsureReportable(s.Status) == nil {
			result = append(result, s)
		}
	}
	return result
}

// ReportPeriod devuelve el período con el menor inicio y el mayor fin; es nil sin sprints.
func ReportPeriod(sprints []SprintRef) *Period {
	if len(sprints) == 0 {
		return nil
	}
	// Se recorre el resto de los sprints ampliando el período inicial.
	period := Period{StartDate: sprints[0].StartDate, EndDate: sprints[0].EndDate}
	for _, s := range sprints[1:] {
		if s.StartDate.Before(period.StartDate) {
			period.StartDate = s.StartDate
		}
		if s.EndDate.After(period.EndDate) {
			period.EndDate = s.EndDate
		}
	}
	return &period
}

// EffortState devuelve la sección de esfuerzo vacía si no hay filas en el alcance (RN-11).
func EffortState(rows []EffortRow) SectionState {
	return stateByCount(len(rows))
}

// DefectsState devuelve la sección de defectos vacía si el listado está vacío (RN-11); los totales
// se conservan aparte.
func DefectsState(defects []DefectLine) SectionState {
	return stateByCount(len(defects))
}

// stateByCount traduce la cantidad de elementos de una sección en su estado: vacía si es cero.
func stateByCount(count int) SectionState {
	if count == 0 {
		return SectionEmpty
	}
	return SectionComplete
}

// ProjectMetricsState devuelve la sección de métricas vacía solo en el alcance project sin sprints
// reportables (RN-11).
func ProjectMetricsState(scope ReportScope, sprints []SprintRef) SectionState {
	// Solo el alcance project puede quedar sin sprints reportables.
	if scope == ScopeProject && len(ReportableSprints(sprints)) == 0 {
		return SectionEmpty
	}
	return SectionComplete
}

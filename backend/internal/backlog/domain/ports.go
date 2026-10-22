package domain

import (
	"context"

	"github.com/google/uuid"
)

// SprintAssignment es lo que informa SprintAssignments sobre una historia (data-model.md sección 7).
// No es un modelo persistido. CommittedToOpenSprint indica si está comprometida en un sprint abierto,
// lo que impide quitarle los Story Points o el último criterio (FR-018, FR-022, RN-18). EverAssigned
// indica si estuvo alguna vez en un sprint, lo que se muestra en el detalle e impide eliminarla
// (FR-039, FR-041, RN-16).
type SprintAssignment struct {
	CommittedToOpenSprint bool
	EverAssigned          bool
}

// SprintAssignments es el puerto hacia la asignación de historias a sprints. El dato lo produce
// specs/004-sprint-management: la implementación de backlog/repository lee los compromisos abiertos
// de sprint_stories y la marca ever_committed de la historia (research.md de la 004, secciones 9 y
// 17.2).
type SprintAssignments interface {
	Find(ctx context.Context, storyID uuid.UUID) (SprintAssignment, error)
}

// EffortRecords es el puerto hacia el registro de esfuerzo: indica si la historia tiene esfuerzo
// registrado, lo que impide eliminarla (FR-041, FR-043). El dato lo producirá
// specs/006-effort-tracking; hasta entonces la implementación informa que no hay esfuerzo.
type EffortRecords interface {
	HasEffort(ctx context.Context, storyID uuid.UUID) (bool, error)
}

// DefectRecords es el puerto hacia el registro de defectos: indica si la historia tiene defectos
// asociados, lo que impide eliminarla (FR-041, FR-043). El dato lo producirá
// specs/007-defect-tracking; hasta entonces la implementación informa que no hay defectos.
type DefectRecords interface {
	HasDefects(ctx context.Context, storyID uuid.UUID) (bool, error)
}

// EstimationSessions es el puerto hacia las sesiones de Planning Poker: indica si la historia tiene
// al menos una sesión de estimación, contando tanto las Finalizadas como las Canceladas, lo que
// impide eliminarla (FR-041, FR-043, RC-10). El dato lo producirá specs/005-planning-poker; hasta
// entonces la implementación informa que no hay sesiones.
type EstimationSessions interface {
	HasSessions(ctx context.Context, storyID uuid.UUID) (bool, error)
}

// ProjectDirectory es el puerto hacia los proyectos: indica si el proyecto está Finalizado, para que
// la creación de historias aplique la regla de solo lectura (FR-048, RN-15). El dato ya lo produce
// specs/002-project-members: la implementación de backlog/repository lee desde el primer día solo
// la columna status de la tabla projects (research.md sección 14).
type ProjectDirectory interface {
	IsFinished(ctx context.Context, projectID uuid.UUID) (bool, error)
}

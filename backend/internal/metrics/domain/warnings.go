package domain

import "github.com/google/uuid"

// WarningKind clasifica una advertencia en una de las dos clases de FR-038.
type WarningKind string

const (
	// WarningPlanning señala un problema de planificación.
	WarningPlanning WarningKind = "planning"
	// WarningDataInconsistency señala una inconsistencia en los datos.
	WarningDataInconsistency WarningKind = "data_inconsistency"
)

// WarningCode identifica el motivo concreto de una advertencia.
type WarningCode string

const (
	// CodeUnestimatedBacklogStories indica historias del backlog sin estimar.
	CodeUnestimatedBacklogStories WarningCode = "unestimated_backlog_stories"
	// CodeUnestimatedStoryInSprint indica una historia sin estimar dentro de un sprint.
	CodeUnestimatedStoryInSprint WarningCode = "unestimated_story_in_sprint"
	// CodeMissingFrozenFactor indica un sprint sin factor congelado.
	CodeMissingFrozenFactor WarningCode = "missing_frozen_factor"
	// CodeMissingCloseTime indica un sprint cerrado sin fecha de cierre.
	CodeMissingCloseTime WarningCode = "missing_close_time"
)

// Warning es una advertencia de cálculo; los mensajes de interfaz viven en delivery.
type Warning struct {
	Kind WarningKind
	Code WarningCode
	// SprintID está presente en las tres advertencias de inconsistencia.
	SprintID *uuid.UUID
	// StoryID solo está presente en CodeUnestimatedStoryInSprint.
	StoryID *uuid.UUID
	// Count solo está presente en CodeUnestimatedBacklogStories.
	Count *int
}

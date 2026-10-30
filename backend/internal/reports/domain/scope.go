// Package domain declara las entidades, los tipos de valor, las reglas y las interfaces del módulo
// de reportes; no depende de ninguna otra capa.
package domain

// ReportScope es el alcance de un reporte; se guarda como texto en la columna
// report_generations.scope y no como enum de PostgreSQL.
type ReportScope string

// Alcances posibles de un reporte.
const (
	ScopeSprint  ReportScope = "sprint"
	ScopeProject ReportScope = "project"
)

// SectionState es el estado de una sección del reporte o de uno de sus grupos; un grupo solo usa
// complete y empty.
type SectionState string

// Estados posibles de una sección.
const (
	SectionComplete    SectionState = "complete"
	SectionEmpty       SectionState = "empty"
	SectionUnavailable SectionState = "unavailable"
)

// StoryResult es el estado de una historia dentro de un grupo: los tres primeros valores son el
// estado vigente y los dos últimos, el resultado congelado de un sprint cerrado.
type StoryResult string

// Estados posibles de una historia en el reporte.
const (
	StoryResultPending      StoryResult = "pending"
	StoryResultInProgress   StoryResult = "in_progress"
	StoryResultCompleted    StoryResult = "completed"
	StoryResultNotCompleted StoryResult = "not_completed"
)

// PokerState es el estado de la estimación colaborativa de una historia en un grupo.
type PokerState string

// Estados posibles de la estimación colaborativa.
const (
	PokerAgreed      PokerState = "agreed"
	PokerNoAgreement PokerState = "no_agreement"
	PokerNone        PokerState = "none"
)

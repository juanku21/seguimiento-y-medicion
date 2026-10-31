package domain

import (
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	defectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	metricsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Report es el documento derivado que se arma en cada pedido y no se persiste. Partial indica que
// incluye un sprint en curso.
type Report struct {
	Scope       ReportScope
	Partial     bool
	Header      Header
	Stories     StoriesSection
	Estimations EstimationsSection
	Effort      EffortSection
	Metrics     MetricsSection
	Defects     DefectsSection
}

// Header es el encabezado del reporte. Sprint es nulo en el alcance project y Period es nulo si no
// hay sprints reportables.
type Header struct {
	Project     ProjectInfo
	Sprint      *SprintRef
	Period      *Period
	GeneratedAt time.Time
	GeneratedBy UserInfo
}

// ProjectInfo son los datos del proyecto que muestra el encabezado.
type ProjectInfo struct {
	ID     uuid.UUID
	Name   string
	Status projectsdomain.ProjectStatus
}

// UserInfo son los datos del titular de la sesión que genera el reporte.
type UserInfo struct {
	ID       uuid.UUID
	FullName string
}

// SprintRef es la referencia a un sprint con los datos que el reporte necesita; ClosedAt solo existe
// en un sprint cerrado.
type SprintRef struct {
	ID        uuid.UUID
	Name      string
	Status    sprintsdomain.SprintStatus
	StartDate time.Time
	EndDate   time.Time
	ClosedAt  *time.Time
}

// Period es el período que cubre el reporte, con fechas a medianoche UTC.
type Period struct {
	StartDate time.Time
	EndDate   time.Time
}

// StoriesSection es la sección de historias.
type StoriesSection struct {
	State  SectionState
	Groups []StoryGroup
}

// StoryGroup agrupa las historias de un sprint; Sprint es nulo en el grupo de no planificadas.
type StoryGroup struct {
	Sprint  *SprintRef
	State   SectionState
	Stories []StoryLine
}

// StoryLine es una historia dentro de un grupo; StoryPoints es nulo si no está estimada.
type StoryLine struct {
	StoryID     uuid.UUID
	Title       string
	Priority    backlogdomain.StoryPriority
	StoryPoints *int
	Status      StoryResult
}

// EstimationsSection es la sección de estimaciones.
type EstimationsSection struct {
	State  SectionState
	Groups []EstimationGroup
}

// EstimationGroup agrupa las estimaciones de un sprint; Sprint es nulo en el grupo de no
// planificadas.
type EstimationGroup struct {
	Sprint *SprintRef
	State  SectionState
	Items  []EstimationLine
}

// EstimationLine es la estimación de una historia; Session solo existe con Poker igual a agreed.
type EstimationLine struct {
	StoryID     uuid.UUID
	Title       string
	StoryPoints *int
	Poker       PokerState
	Session     *PokerSummary
}

// PokerSummary resume la sesión de Planning Poker que acordó el valor.
type PokerSummary struct {
	Rounds            int
	AgreedStoryPoints int
	FinishedAt        time.Time
}

// SessionRecord es una sesión de Planning Poker tal como la lee el repositorio; Rounds se cuenta en
// la misma consulta.
type SessionRecord struct {
	ID                uuid.UUID
	Status            pokerdomain.SessionStatus
	CreatedAt         time.Time
	ClosedAt          *time.Time
	AgreedStoryPoints *int
	Rounds            int
}

// EffortSection es la sección de esfuerzo; los totales están en centésimas de hora.
type EffortSection struct {
	State           SectionState
	Groups          []EffortGroup
	ByMember        []MemberEffort
	TotalHundredths int
}

// EffortGroup agrupa el esfuerzo de un sprint.
type EffortGroup struct {
	Sprint          SprintRef
	State           SectionState
	Stories         []StoryEffort
	TotalHundredths int
}

// StoryEffort es el esfuerzo de una historia dentro de un sprint, en centésimas de hora.
type StoryEffort struct {
	StoryID    uuid.UUID
	Title      string
	Hundredths int
}

// MemberEffort es el esfuerzo de un integrante en todo el alcance, en centésimas de hora.
type MemberEffort struct {
	UserID     uuid.UUID
	FullName   string
	Hundredths int
}

// EffortRow es una fila agregada de esfuerzo por sprint, historia y usuario, tal como la lee el
// repositorio.
type EffortRow struct {
	SprintID     uuid.UUID
	StoryID      uuid.UUID
	CurrentTitle string
	UserID       uuid.UUID
	FullName     string
	Hundredths   int
}

// MetricsSection es la sección de métricas: Sprint y SprintWarnings en el alcance sprint, Project en
// el alcance project.
type MetricsSection struct {
	State          SectionState
	Sprint         *metricsdomain.SprintMetrics
	SprintWarnings []metricsdomain.Warning
	Project        *metricsdomain.ProjectMetrics
}

// DefectsSection es la sección de defectos; Totals sale de las métricas.
type DefectsSection struct {
	State   SectionState
	Defects []DefectLine
	Totals  *DefectTotals
}

// DefectLine es un defecto del listado; ResolvedSprint es nulo si no se resolvió.
type DefectLine struct {
	DefectID       uuid.UUID
	Description    string
	Severity       defectsdomain.DefectSeverity
	Status         defectsdomain.DefectStatus
	Story          StoryRef
	DetectedSprint SprintName
	ResolvedSprint *SprintName
}

// StoryRef identifica una historia por su identificador y su título.
type StoryRef struct {
	ID    uuid.UUID
	Title string
}

// SprintName identifica un sprint por su identificador y su nombre.
type SprintName struct {
	ID   uuid.UUID
	Name string
}

// DefectTotals son los totales de defectos detectados y resueltos que informa el módulo de métricas.
type DefectTotals struct {
	Detected int
	Resolved int
}

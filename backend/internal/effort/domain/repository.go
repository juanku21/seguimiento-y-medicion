package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EntryAccess es el tipo de lectura que arma FindAccessible para el middleware
// RequireEffortEntryAccess (data-model.md sección 8). No es un modelo persistido: sale de una sola
// consulta que une el registro con su historia, el proyecto de la historia y la membresía vigente
// del titular de la sesión. AuthorID es quien registró el esfuerzo, para la regla de autoría, y
// ProjectFinished indica si el proyecto está Finalizado, para aplicar la regla de solo lectura sin
// una segunda lectura.
type EntryAccess struct {
	EntryID         uuid.UUID
	StoryID         uuid.UUID
	ProjectID       uuid.UUID
	AuthorID        uuid.UUID
	ProjectFinished bool
}

// UserRef identifica al autor de un registro tal como lo leen las consultas de esfuerzo: solo el
// identificador y el nombre completo, seleccionados con un JOIN a users para que el hash de la
// contraseña nunca entre al módulo.
type UserRef struct {
	ID       uuid.UUID
	FullName string
}

// StoryRef identifica la historia de un registro con su identificador y su título.
type StoryRef struct {
	ID    uuid.UUID
	Title string
}

// EntryView es el tipo de lectura de un registro de esfuerzo en las listas y en la respuesta de las
// escrituras (data-model.md sección 8). No es un modelo persistido: lleva el registro, su autor, su
// historia y su sprint asociado, que es nil si el registro no tiene sprint. Entry.Story, Entry.User
// y Entry.Sprint nunca se precargan: esos datos viajan en Story, Author y Sprint. Definitive no se
// persiste ni lo lee el repositorio: lo deriva el servicio con su reloj y el plazo de gracia
// (FR-020).
type EntryView struct {
	Entry      EffortEntry
	Author     UserRef
	Story      StoryRef
	Sprint     *AnchoredSprint
	Definitive bool
}

// MemberEffort son las horas que un integrante registró sobre una historia, sumadas en centésimas
// de hora, con su nombre completo (FR-032).
type MemberEffort struct {
	UserID          uuid.UUID
	FullName        string
	HoursHundredths int
}

// StoryEffort es el esfuerzo de una historia: el total en centésimas de hora, el desglose por
// integrante y los registros (FR-030 a FR-032).
type StoryEffort struct {
	TotalHundredths int
	ByMember        []MemberEffort
	Entries         []EntryView
}

// EntryFilter son los filtros combinables de la consulta de esfuerzo de un proyecto (FR-036 a
// FR-039). Cada puntero nil significa "sin filtrar por ese campo"; WithoutSprint pide solo los
// registros sin sprint, y From y To acotan la fecha del registro, ambos extremos incluidos (FR-038).
type EntryFilter struct {
	UserID        *uuid.UUID
	StoryID       *uuid.UUID
	SprintID      *uuid.UUID
	WithoutSprint bool
	From          *time.Time
	To            *time.Time
}

// ComparisonRow es una fila de la comparación entre horas estimadas y reales (FR-046 a FR-051):
// la historia, sus Story Points (nil si no está estimada) y las horas reales en centésimas de hora.
type ComparisonRow struct {
	StoryID          uuid.UUID
	Title            string
	StoryPoints      *int
	ActualHundredths int
}

// EntryInput son los datos del alta de un registro que delivery entrega al servicio ya normalizados
// y validados. EntryDate va a medianoche UTC, igual que la columna date de effort_entries.
type EntryInput struct {
	EntryDate       time.Time
	Activity        string
	HoursHundredths int
}

// EntryPatch son los datos de una modificación parcial de un registro, ya normalizados y validados:
// cada puntero nil significa que ese campo no cambia.
type EntryPatch struct {
	EntryDate       *time.Time
	Activity        *string
	HoursHundredths *int
}

// EffortRepository declara solo las operaciones sobre effort_entries que usa esta feature
// (data-model.md sección 9; Principio III). FindAccessible resuelve RequireEffortEntryAccess. Las
// escrituras evalúan las reglas puras del dominio dentro de su transacción, con el orden de bloqueos
// de research.md sección 10, y reciben el instante now, la zona location en la que se calcula el día
// de hoy y el plazo de gracia grace. ListForStory devuelve los registros de una historia y
// ListForProject los de un proyecto según el filtro, que ValidateFilter verifica contra el proyecto;
// HasProjectEffort distingue el proyecto sin esfuerzo del filtro sin resultados. ComparisonRows
// devuelve el factor de horas por Story Point vigente del proyecto y las filas de la comparación,
// de una sola historia si storyID no es nil. Ninguna operación devuelve gorm.ErrRecordNotFound ni
// (nil, nil): si el registro no existe o es ajeno, devuelve ErrEntryNotFound.
type EffortRepository interface {
	FindAccessible(ctx context.Context, entryID, userID uuid.UUID) (*EntryAccess, error)
	Create(ctx context.Context, storyID, authorID uuid.UUID, input EntryInput, now time.Time, location *time.Location, grace time.Duration) (*EntryView, error)
	Update(ctx context.Context, entryID, actorID uuid.UUID, patch EntryPatch, now time.Time, location *time.Location, grace time.Duration) (*EntryView, error)
	Delete(ctx context.Context, entryID, actorID uuid.UUID, now time.Time, grace time.Duration) error
	ListForStory(ctx context.Context, storyID uuid.UUID) ([]EntryView, error)
	ListForProject(ctx context.Context, projectID uuid.UUID, filter EntryFilter) ([]EntryView, error)
	ValidateFilter(ctx context.Context, projectID uuid.UUID, filter EntryFilter) error
	HasProjectEffort(ctx context.Context, projectID uuid.UUID) (bool, error)
	ComparisonRows(ctx context.Context, projectID uuid.UUID, storyID *uuid.UUID) (factorHundredths int, rows []ComparisonRow, err error)
}

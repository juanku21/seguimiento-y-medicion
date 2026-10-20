package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StoryAccess es el tipo de lectura que arma FindAccessible para el middleware RequireStoryAccess
// (data-model.md sección 6; research.md sección 3). No es un modelo persistido: sale de una sola
// consulta que une la historia con la membresía vigente del titular de la sesión y con su proyecto.
// ProjectFinished indica si el proyecto está Finalizado, para aplicar la regla de solo lectura
// (FR-048) sin una segunda lectura.
type StoryAccess struct {
	StoryID         uuid.UUID
	ProjectID       uuid.UUID
	ProjectFinished bool
}

// StoryListItem es el tipo de lectura de un elemento del listado del backlog (FR-033). No es un
// modelo persistido: lo arma List en una sola consulta que cuenta los criterios de cada historia.
// StoryPoints nil significa "sin estimar", y la condición de lista para planificar no se guarda acá:
// se calcula con la regla de Readiness a partir de StoryPoints y CriteriaCount.
type StoryListItem struct {
	ID            uuid.UUID
	Title         string
	Priority      StoryPriority
	Status        StoryStatus
	StoryPoints   *int
	CriteriaCount int
	CreatedAt     time.Time
}

// StoryAuthor es el autor de una historia tal como lo lee FindDetail: solo el identificador y el
// nombre completo, seleccionados con un JOIN a users para que el hash de la contraseña nunca entre
// al módulo (data-model.md sección 6). Se conserva aunque el autor haya dejado de ser integrante
// (FR-050).
type StoryAuthor struct {
	ID       uuid.UUID
	FullName string
}

// StoryDetail es el tipo de lectura del detalle de una historia y de la respuesta de las escrituras
// (FR-039). No es un modelo persistido: lo arma FindDetail con la historia, los textos de sus
// criterios en el orden de su posición y el autor. Story.CreatedBy nunca se precarga: el autor viaja
// en CreatedBy.
type StoryDetail struct {
	Story              UserStory
	AcceptanceCriteria []string
	CreatedBy          StoryAuthor
}

// StoryFilter es el filtro del listado que arma delivery a partir de la consulta (FR-034). Una lista
// vacía significa que ese criterio no filtra; los valores de una misma lista se combinan con "o" y
// las dos listas entre sí con "y".
type StoryFilter struct {
	Statuses   []StoryStatus
	Priorities []StoryPriority
}

// StoryChanges son los cambios de un PATCH que delivery entrega al servicio (FR-020; research.md
// sección 6). Un campo nil significa ausente y deja el valor vigente. Description necesita además
// DescriptionSet, porque presente con valor nil significa borrar la descripción. AcceptanceCriteria
// presente reemplaza la lista completa, y presente como lista vacía la vacía. No incluye estado,
// proyecto, autor ni momento de creación, que no se modifican desde el backlog (FR-024, FR-027).
type StoryChanges struct {
	Title              *string
	DescriptionSet     bool
	Description        *string
	Priority           *StoryPriority
	AcceptanceCriteria *[]string
}

// StoryRepository declara solo las operaciones sobre user_stories y acceptance_criteria que usa
// esta feature (data-model.md sección 9; Principio III). FindAccessible resuelve RequireStoryAccess
// y Create persiste la historia con sus criterios en una transacción. FindDetail devuelve la
// historia, sus criterios ordenados y su autor. List devuelve el backlog filtrado de un proyecto con
// el conteo de criterios en una consulta, y HasStories distingue el backlog vacío del filtro sin
// resultados. Update toma un bloqueo de fila y escribe solo las columnas presentes y, si vienen, los
// criterios; UpdateStoryPoints guarda la estimación, y Delete borra criterios e historia en una
// transacción. No se declaran cambio de estado, cambio de proyecto, búsqueda por texto, paginación
// ni listados fuera de un proyecto. Cada operación devuelve el error de negocio y nunca
// gorm.ErrRecordNotFound ni (nil, nil): si la historia no existe o es ajena, devuelve
// ErrStoryNotFound.
type StoryRepository interface {
	FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*StoryAccess, error)
	Create(ctx context.Context, story *UserStory, criteria []string) error
	FindDetail(ctx context.Context, storyID uuid.UUID) (*StoryDetail, error)
	List(ctx context.Context, projectID uuid.UUID, filter StoryFilter) ([]StoryListItem, error)
	HasStories(ctx context.Context, projectID uuid.UUID) (bool, error)
	Update(ctx context.Context, storyID uuid.UUID, changes StoryChanges) error
	UpdateStoryPoints(ctx context.Context, storyID uuid.UUID, storyPoints *int) error
	Delete(ctx context.Context, storyID uuid.UUID) error
}

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// SprintAccess es el tipo de lectura que arma FindAccessible para el middleware RequireSprintAccess
// (data-model.md sección 8; research.md sección 3). No es un modelo persistido: sale de una sola
// consulta que une el sprint con la membresía vigente del titular de la sesión y con su proyecto.
// ProjectFinished indica si el proyecto está Finalizado, para aplicar la regla de solo lectura
// (FR-059) sin una segunda lectura.
type SprintAccess struct {
	SprintID        uuid.UUID
	ProjectID       uuid.UUID
	ProjectFinished bool
}

// SprintInput son los datos de creación y modificación de un sprint que delivery entrega al servicio
// ya normalizados y validados (data-model.md sección 8). StartDate y EndDate van a medianoche UTC,
// igual que las columnas date de sprints.
type SprintInput struct {
	Name      string
	Goal      string
	StartDate time.Time
	EndDate   time.Time
}

// SprintResults son los totales de la instantánea de cierre (data-model.md sección 7).
// CommittedStoryPoints sale de sprints.committed_story_points y los demás se agregan de las filas
// congeladas de sprint_stories en un único fragmento de consulta compartido por List y FindDetail.
type SprintResults struct {
	CommittedStoryPoints int
	PlannedStoryPoints   int
	CompletedStoryPoints int
	PlannedStoryCount    int
	CompletedStoryCount  int
}

// SprintListItem es el tipo de lectura de un elemento del listado y del historial de sprints de un
// proyecto (FR-050, FR-051). No es un modelo persistido: lo arma List con los datos del sprint, la
// cantidad de historias comprometidas (las planificadas al cierre en un sprint Cerrado) y, solo si
// el sprint está Cerrado, los totales de la instantánea; en otro caso Results es nil.
type SprintListItem struct {
	Sprint     Sprint
	StoryCount int
	Results    *SprintResults
}

// SprintUserRef identifica al autor o a quien cerró un sprint tal como lo lee FindDetail: solo el
// identificador y el nombre completo, seleccionados con un JOIN a users para que el hash de la
// contraseña nunca entre al módulo (research.md sección 19).
type SprintUserRef struct {
	ID       uuid.UUID
	FullName string
}

// SprintStoryView es una historia del detalle de un sprint (US6-8). En un sprint abierto lleva los
// datos vigentes de user_stories y Outcome es nil; en uno Cerrado lleva los congelados de
// sprint_stories. StoryPoints nil significa "sin estimar" y CompletedAt solo tiene valor si la
// historia se completó dentro del sprint.
type SprintStoryView struct {
	StoryID     uuid.UUID
	Title       string
	Priority    backlogdomain.StoryPriority
	StoryPoints *int
	Status      backlogdomain.StoryStatus
	CompletedAt *time.Time
	Outcome     *StoryOutcome
}

// SprintDetail es el tipo de lectura del detalle de un sprint y de la respuesta de las escrituras
// (FR-053). No es un modelo persistido: lo arma FindDetail con el sprint, su autor, quien lo cerró
// (nil si no está Cerrado), sus historias y, si está Cerrado, los totales de la instantánea; el
// factor de horas por Story Point copiado al cerrar viaja en Sprint.HoursPerStoryPointHundredths.
// Sprint.CreatedBy y Sprint.ClosedBy nunca se precargan: los usuarios viajan en CreatedBy y ClosedBy.
type SprintDetail struct {
	Sprint    Sprint
	CreatedBy SprintUserRef
	ClosedBy  *SprintUserRef
	Stories   []SprintStoryView
	Results   *SprintResults
}

// StorySprintHistoryItem es un elemento del historial de sprints de una historia (FR-055), ordenado
// por start_date y luego por el identificador del sprint. Outcome vale completed o not_completed en
// los sprints Cerrados y nil en el sprint abierto en el que está la historia.
type StorySprintHistoryItem struct {
	SprintID     uuid.UUID
	SprintName   string
	SprintStatus SprintStatus
	StartDate    time.Time
	EndDate      time.Time
	Outcome      *StoryOutcome
	CompletedAt  *time.Time
}

// SprintRepository declara solo las operaciones sobre sprints y sprint_stories que usa esta feature
// (data-model.md sección 9; Principio III). FindAccessible resuelve RequireSprintAccess. Las
// escrituras toman sus bloqueos y evalúan las reglas puras con los datos releídos dentro de la misma
// transacción: Create y Update rechazan la superposición con SprintOverlapError, Start rechaza un
// segundo sprint activo con ActiveSprintExistsError y congela los Story Points comprometidos, Close
// congela las filas de sprint_stories, devuelve a pending las historias no completadas y copia el
// factor del proyecto, AssignStories rechaza el lote completo con StoriesNotAdmittedError, y Delete y
// RemoveStories devuelven las historias a pending. ChangeStoryProgress aplica la transición de la
// historia dentro del sprint Activo. Las escrituras que fijan momentos reciben el instante now, que
// el servicio toma del reloj inyectado. List devuelve los sprints de un proyecto filtrados por
// estado, y HasSprints distingue el proyecto sin sprints del filtro sin resultados. FindDetail
// devuelve el detalle de un sprint y ListForStory el historial de sprints de una historia. No se
// declaran cambio de proyecto, reapertura ni listados fuera de un proyecto. Ninguna operación
// devuelve gorm.ErrRecordNotFound ni (nil, nil): si el sprint no existe o es ajeno, devuelve
// ErrSprintNotFound.
type SprintRepository interface {
	FindAccessible(ctx context.Context, sprintID, userID uuid.UUID) (*SprintAccess, error)
	Create(ctx context.Context, sprint *Sprint) error
	Update(ctx context.Context, sprintID uuid.UUID, input SprintInput) error
	Delete(ctx context.Context, sprintID uuid.UUID) error
	Start(ctx context.Context, sprintID uuid.UUID, now time.Time) error
	Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error
	AssignStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID, now time.Time) error
	RemoveStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID) error
	ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error
	List(ctx context.Context, projectID uuid.UUID, statuses []SprintStatus) ([]SprintListItem, error)
	HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error)
	FindDetail(ctx context.Context, sprintID uuid.UUID) (*SprintDetail, error)
	ListForStory(ctx context.Context, storyID uuid.UUID) ([]StorySprintHistoryItem, error)
}

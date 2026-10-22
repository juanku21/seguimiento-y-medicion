package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// errNotImplemented es el error de las firmas mínimas que crearon las pruebas de integración de T013;
// cada operación se implementa en la tarea de su historia (T017 en adelante).
var errNotImplemented = errors.New("not implemented")

// SprintRepository implementa domain.SprintRepository con GORM sobre las tablas sprints y
// sprint_stories. Create y FindDetail están implementadas; las demás operaciones conservan la firma
// mínima de T013 y se implementan en la tarea de su historia.
type SprintRepository struct {
	db *gorm.DB
}

// NewSprintRepository crea el repositorio de sprints sobre la conexión recibida.
func NewSprintRepository(db *gorm.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

// sprintAccessRow es la fila que lee FindAccessible: los identificadores del sprint y de su proyecto,
// y el estado del proyecto que llega por el JOIN a projects con el alias project_status.
type sprintAccessRow struct {
	SprintID      uuid.UUID
	ProjectID     uuid.UUID
	ProjectStatus projectsdomain.ProjectStatus
}

// FindAccessible resuelve el acceso a un sprint para RequireSprintAccess con una sola consulta
// (research.md sección 3): une sprints con la membresía vigente del usuario en project_members y con
// projects. Si no hay fila, el sprint no existe o el usuario no es integrante de su proyecto, y ambos
// casos se reportan igual con ErrSprintNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil). El
// estado del proyecto se compara con ProjectStatusFinished para la regla de solo lectura (FR-059).
func (r *SprintRepository) FindAccessible(ctx context.Context, sprintID, userID uuid.UUID) (*domain.SprintAccess, error) {
	var row sprintAccessRow
	err := r.db.WithContext(ctx).
		Table("sprints").
		Select(`sprints.id AS sprint_id, sprints.project_id AS project_id,
			projects.status AS project_status`).
		Joins(`JOIN project_members ON project_members.project_id = sprints.project_id
			AND project_members.user_id = ?`, userID).
		Joins("JOIN projects ON projects.id = sprints.project_id").
		Where("sprints.id = ?", sprintID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso al sprint: %w", err)
	}
	return &domain.SprintAccess{
		SprintID:        row.SprintID,
		ProjectID:       row.ProjectID,
		ProjectFinished: row.ProjectStatus == projectsdomain.ProjectStatusFinished,
	}, nil
}

// Create registra el sprint en una transacción (research.md sección 4). Primero toma un bloqueo
// FOR UPDATE sobre la fila del proyecto y lee su estado: así las escrituras de calendario de un mismo
// proyecto quedan serializadas y un proyecto Finalizado se rechaza con ErrSprintsReadOnly (FR-059).
// Después busca, entre todos los sprints del proyecto y cualquiera sea su estado, uno cuyo período se
// superponga con el nuevo, con la traducción de PeriodsOverlap a desigualdades estrictas; si lo hay,
// devuelve SprintOverlapError con el de inicio más temprano (y luego menor id), para que el mensaje
// sea determinista (FR-005); se lee con Find y LIMIT 1 para que la ausencia de conflicto, que es el
// caso normal, no pase por gorm.ErrRecordNotFound. Si no, inserta el sprint; CreatedAt y UpdatedAt
// los completa GORM.
func (r *SprintRepository) Create(ctx context.Context, sprint *domain.Sprint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project projectsdomain.Project
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("status").
			Where("id = ?", sprint.ProjectID).
			Take(&project).Error
		if err != nil {
			return fmt.Errorf("bloquear el proyecto del sprint: %w", err)
		}
		if project.Status == projectsdomain.ProjectStatusFinished {
			return domain.ErrSprintsReadOnly
		}

		var conflicts []domain.ConflictingSprint
		err = tx.
			Model(&domain.Sprint{}).
			Select("id, name").
			Where("project_id = ?", sprint.ProjectID).
			Where("start_date < ? AND ? < end_date", sprint.EndDate, sprint.StartDate).
			Order("start_date ASC, id ASC").
			Limit(1).
			Find(&conflicts).Error
		if err != nil {
			return fmt.Errorf("buscar sprints superpuestos: %w", err)
		}
		if len(conflicts) > 0 {
			return &domain.SprintOverlapError{Sprint: conflicts[0]}
		}

		if err := tx.Create(sprint).Error; err != nil {
			return fmt.Errorf("insertar sprint: %w", err)
		}
		return nil
	})
}

// Update modifica un sprint planificado. Firma mínima.
func (r *SprintRepository) Update(ctx context.Context, sprintID uuid.UUID, input domain.SprintInput) error {
	return errNotImplemented
}

// Delete elimina un sprint planificado. Firma mínima.
func (r *SprintRepository) Delete(ctx context.Context, sprintID uuid.UUID) error {
	return errNotImplemented
}

// Start inicia un sprint. Firma mínima.
func (r *SprintRepository) Start(ctx context.Context, sprintID uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// Close cierra un sprint. Firma mínima.
func (r *SprintRepository) Close(ctx context.Context, sprintID, closedBy uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// AssignStories compromete historias en un sprint. Firma mínima.
func (r *SprintRepository) AssignStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID, now time.Time) error {
	return errNotImplemented
}

// RemoveStories quita historias de un sprint. Firma mínima.
func (r *SprintRepository) RemoveStories(ctx context.Context, sprintID uuid.UUID, storyIDs []uuid.UUID) error {
	return errNotImplemented
}

// ChangeStoryProgress cambia el estado de una historia del sprint activo. Firma mínima.
func (r *SprintRepository) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus, now time.Time) error {
	return errNotImplemented
}

// List devuelve los sprints de un proyecto. Firma mínima.
func (r *SprintRepository) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) ([]domain.SprintListItem, error) {
	return nil, errNotImplemented
}

// HasSprints indica si el proyecto tiene sprints. Firma mínima.
func (r *SprintRepository) HasSprints(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errNotImplemented
}

// sprintDetailRow es la fila que lee FindDetail: las columnas del sprint más el identificador y el
// nombre completo del autor, que llegan por el JOIN a users con los alias author_id y
// author_full_name.
type sprintDetailRow struct {
	domain.Sprint  `gorm:"embedded"`
	AuthorID       uuid.UUID
	AuthorFullName string
}

// FindDetail lee el detalle del sprint en dos consultas (research.md sección 19). La primera une
// sprints con users y de users selecciona solo id y full_name, de modo que password_hash nunca entra
// al módulo; si no hay fila devuelve ErrSprintNotFound, nunca gorm.ErrRecordNotFound ni (nil, nil).
// La segunda lee las historias vivas del sprint, uniendo sprint_stories con user_stories para tomar
// los datos vigentes de cada historia, ordenadas por prioridad Alta, Media, Baja, luego por momento
// de compromiso y por último por id; sin historias devuelve una lista vacía, nunca nil. ClosedBy y
// Results quedan en nil: el detalle de un sprint Cerrado se completa en la historia de cierre.
func (r *SprintRepository) FindDetail(ctx context.Context, sprintID uuid.UUID) (*domain.SprintDetail, error) {
	db := r.db.WithContext(ctx)

	var row sprintDetailRow
	err := db.
		Table("sprints").
		Select("sprints.*, users.id AS author_id, users.full_name AS author_full_name").
		Joins("JOIN users ON users.id = sprints.created_by_id").
		Where("sprints.id = ?", sprintID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar el detalle del sprint: %w", err)
	}

	stories := []domain.SprintStoryView{}
	err = db.
		Table("sprint_stories").
		Select("user_stories.id AS story_id, user_stories.title, user_stories.priority, "+
			"user_stories.story_points, user_stories.status, sprint_stories.completed_at").
		Joins("JOIN user_stories ON user_stories.id = sprint_stories.story_id").
		Where("sprint_stories.sprint_id = ?", sprintID).
		Order("CASE user_stories.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
			"sprint_stories.committed_at ASC, user_stories.id ASC").
		Scan(&stories).Error
	if err != nil {
		return nil, fmt.Errorf("leer las historias del sprint: %w", err)
	}

	return &domain.SprintDetail{
		Sprint: row.Sprint,
		CreatedBy: domain.SprintUserRef{
			ID:       row.AuthorID,
			FullName: row.AuthorFullName,
		},
		Stories: stories,
	}, nil
}

// ListForStory devuelve el historial de sprints de una historia. Firma mínima.
func (r *SprintRepository) ListForStory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	return nil, errNotImplemented
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// StoryRepository implementa domain.StoryRepository con GORM sobre user_stories y
// acceptance_criteria. Create, FindDetail, List, HasStories, FindAccessible, Update y
// UpdateStoryPoints están implementadas; las demás operaciones las implementan las historias que las
// usan.
type StoryRepository struct {
	db *gorm.DB
}

// NewStoryRepository crea el repositorio de historias sobre la conexión recibida.
func NewStoryRepository(db *gorm.DB) *StoryRepository {
	return &StoryRepository{db: db}
}

// storyDetailRow es la fila que lee FindDetail: las columnas de la historia más el identificador y
// el nombre completo del autor, que llegan por el JOIN a users con los alias author_id y
// author_full_name.
type storyDetailRow struct {
	domain.UserStory `gorm:"embedded"`
	AuthorID         uuid.UUID
	AuthorFullName   string
}

// storyAccessRow es la fila que lee FindAccessible: los identificadores de la historia y de su
// proyecto, y el estado del proyecto que llega por el JOIN a projects con el alias project_status.
type storyAccessRow struct {
	StoryID       uuid.UUID
	ProjectID     uuid.UUID
	ProjectStatus projectsdomain.ProjectStatus
}

// FindAccessible resuelve el acceso a una historia para RequireStoryAccess con una sola consulta
// (research.md sección 3): une user_stories con la membresía vigente del usuario en project_members
// y con projects. Si no hay fila, la historia no existe o el usuario no es integrante de su
// proyecto, y ambos casos se reportan igual con ErrStoryNotFound, nunca gorm.ErrRecordNotFound ni
// (nil, nil). El estado del proyecto se compara con ProjectStatusFinished para la regla de solo
// lectura (FR-048).
func (r *StoryRepository) FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*domain.StoryAccess, error) {
	var row storyAccessRow
	err := r.db.WithContext(ctx).
		Table("user_stories").
		Select(`user_stories.id AS story_id, user_stories.project_id AS project_id,
			projects.status AS project_status`).
		Joins(`JOIN project_members ON project_members.project_id = user_stories.project_id
			AND project_members.user_id = ?`, userID).
		Joins("JOIN projects ON projects.id = user_stories.project_id").
		Where("user_stories.id = ?", storyID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrStoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolver el acceso a la historia: %w", err)
	}
	return &domain.StoryAccess{
		StoryID:         row.StoryID,
		ProjectID:       row.ProjectID,
		ProjectFinished: row.ProjectStatus == projectsdomain.ProjectStatusFinished,
	}, nil
}

// Create persiste la historia y sus criterios en una sola transacción, de modo que si falla
// cualquier INSERT no queda ni la historia ni ninguno de sus criterios (FR-013). Cada criterio
// recibe un UUID nuevo y la posición de su índice en la lista (0..n-1); si no hay criterios solo se
// inserta la historia. CreatedAt y UpdatedAt los completa GORM.
func (r *StoryRepository) Create(ctx context.Context, story *domain.UserStory, criteria []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(story).Error; err != nil {
			return fmt.Errorf("insertar historia: %w", err)
		}
		return insertCriteria(tx, story.ID, criteria)
	})
}

// FindDetail lee el detalle de la historia en dos consultas. La primera une user_stories con users
// y de users selecciona solo id y full_name, de modo que
// password_hash nunca entra al módulo; si no hay fila devuelve ErrStoryNotFound, nunca
// gorm.ErrRecordNotFound ni (nil, nil). La segunda lee los textos de los criterios ordenados por
// position; sin criterios devuelve una lista vacía, nunca nil.
func (r *StoryRepository) FindDetail(ctx context.Context, storyID uuid.UUID) (*domain.StoryDetail, error) {
	db := r.db.WithContext(ctx)

	var row storyDetailRow
	err := db.
		Table("user_stories").
		Select("user_stories.*, users.id AS author_id, users.full_name AS author_full_name").
		Joins("JOIN users ON users.id = user_stories.created_by_id").
		Where("user_stories.id = ?", storyID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrStoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar el detalle de la historia: %w", err)
	}

	criteria := []string{}
	err = db.
		Model(&domain.AcceptanceCriterion{}).
		Where("story_id = ?", storyID).
		Order("position ASC").
		Pluck("text", &criteria).Error
	if err != nil {
		return nil, fmt.Errorf("leer los criterios de aceptación de la historia: %w", err)
	}

	return &domain.StoryDetail{
		Story:              row.UserStory,
		AcceptanceCriteria: criteria,
		CreatedBy: domain.StoryAuthor{
			ID:       row.AuthorID,
			FullName: row.AuthorFullName,
		},
	}, nil
}

// List devuelve el backlog de un proyecto en una sola consulta sobre user_stories, con una
// subconsulta correlacionada que cuenta los criterios de cada historia (research.md sección 9). Cada
// lista no vacía del filtro agrega una condición IN, y GORM combina las condiciones con AND
// (FR-034). El orden es prioridad Alta, Media, Baja, luego momento de creación y por último id, que
// hace el orden reproducible (FR-032). Sin resultados devuelve una lista vacía, nunca nil.
func (r *StoryRepository) List(ctx context.Context, projectID uuid.UUID, filter domain.StoryFilter) ([]domain.StoryListItem, error) {
	query := r.db.WithContext(ctx).
		Table("user_stories").
		Select("user_stories.id, user_stories.title, user_stories.priority, user_stories.status, "+
			"user_stories.story_points, user_stories.created_at, "+
			"(SELECT COUNT(*) FROM acceptance_criteria WHERE acceptance_criteria.story_id = user_stories.id) AS criteria_count").
		Where("user_stories.project_id = ?", projectID)
	if len(filter.Statuses) > 0 {
		query = query.Where("user_stories.status IN ?", filter.Statuses)
	}
	if len(filter.Priorities) > 0 {
		query = query.Where("user_stories.priority IN ?", filter.Priorities)
	}

	items := []domain.StoryListItem{}
	err := query.
		Order("CASE user_stories.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, " +
			"user_stories.created_at ASC, user_stories.id ASC").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("listar las historias del proyecto: %w", err)
	}
	return items, nil
}

// HasStories indica si el proyecto tiene alguna historia con un SELECT 1 ... LIMIT 1, sin contar
// todas las filas (research.md sección 9). Sin filas, Scan deja found en 0 y no devuelve error.
func (r *StoryRepository) HasStories(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var found int
	err := r.db.WithContext(ctx).
		Raw("SELECT 1 FROM user_stories WHERE project_id = ? LIMIT 1", projectID).
		Scan(&found).Error
	if err != nil {
		return false, fmt.Errorf("comprobar si el proyecto tiene historias: %w", err)
	}
	return found == 1, nil
}

// Update aplica los cambios presentes de un PATCH en una transacción (research.md sección 7). Primero
// toma un bloqueo de fila sobre la historia con SELECT ... FOR UPDATE, que serializa dos ediciones
// simultáneas de la misma historia; si no hay fila devuelve ErrStoryNotFound. Después escribe con
// UpdateColumns solo las columnas presentes más updated_at, nunca el registro entero, para no pisar
// el cambio de otro integrante en otro campo (FR-025). Una descripción presente con valor nil se
// guarda como NULL. Si vienen criterios, reemplaza la lista completa: borra los de la historia e
// inserta los nuevos con posiciones 0..n-1 (research.md sección 5); la lista vacía solo borra.
func (r *StoryRepository) Update(ctx context.Context, storyID uuid.UUID, changes domain.StoryChanges) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked domain.UserStory
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", storyID).
			Take(&locked).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrStoryNotFound
		}
		if err != nil {
			return fmt.Errorf("bloquear la historia: %w", err)
		}

		columns := map[string]any{"updated_at": time.Now()}
		if changes.Title != nil {
			columns["title"] = *changes.Title
		}
		if changes.DescriptionSet {
			columns["description"] = changes.Description
		}
		if changes.Priority != nil {
			columns["priority"] = *changes.Priority
		}
		err = tx.
			Model(&domain.UserStory{}).
			Where("id = ?", storyID).
			UpdateColumns(columns).Error
		if err != nil {
			return fmt.Errorf("actualizar la historia: %w", err)
		}

		if changes.AcceptanceCriteria == nil {
			return nil
		}
		return replaceCriteria(tx, storyID, *changes.AcceptanceCriteria)
	})
}

// replaceCriteria borra los criterios de la historia e inserta la lista nueva dentro de la
// transacción recibida. Con la lista vacía solo borra.
func replaceCriteria(tx *gorm.DB, storyID uuid.UUID, criteria []string) error {
	err := tx.
		Where("story_id = ?", storyID).
		Delete(&domain.AcceptanceCriterion{}).Error
	if err != nil {
		return fmt.Errorf("borrar criterios de aceptación: %w", err)
	}
	return insertCriteria(tx, storyID, criteria)
}

// insertCriteria inserta los criterios de la historia dentro de la transacción recibida, asignando a
// cada uno un UUID nuevo y la posición de su índice en la lista (0..n-1). Con la lista vacía no
// ejecuta ningún INSERT. Lo comparten Create y replaceCriteria.
func insertCriteria(tx *gorm.DB, storyID uuid.UUID, criteria []string) error {
	if len(criteria) == 0 {
		return nil
	}
	rows := make([]domain.AcceptanceCriterion, 0, len(criteria))
	for i, text := range criteria {
		rows = append(rows, domain.AcceptanceCriterion{
			ID:       uuid.New(),
			StoryID:  storyID,
			Position: i,
			Text:     text,
		})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("insertar criterios de aceptación: %w", err)
	}
	return nil
}

// UpdateStoryPoints guarda la estimación de la historia con un único UPDATE que escribe solo
// story_points y updated_at (FR-017), de modo que ninguna otra columna se toca. Un storyPoints nil
// se guarda como NULL, que significa "sin estimar". UpdateColumns evita los hooks y la escritura
// automática de GORM, por eso updated_at se fija acá de forma explícita. Si ninguna fila cambia, la
// historia no existe y se devuelve ErrStoryNotFound.
func (r *StoryRepository) UpdateStoryPoints(ctx context.Context, storyID uuid.UUID, storyPoints *int) error {
	result := r.db.WithContext(ctx).
		Model(&domain.UserStory{}).
		Where("id = ?", storyID).
		UpdateColumns(map[string]any{
			"story_points": storyPoints,
			"updated_at":   time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("guardar la estimación de la historia: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrStoryNotFound
	}
	return nil
}

// Delete borra la historia y sus criterios en una transacción. Firma mínima.
func (r *StoryRepository) Delete(ctx context.Context, storyID uuid.UUID) error {
	return errors.New("not implemented")
}

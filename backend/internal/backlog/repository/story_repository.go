package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// StoryRepository implementa domain.StoryRepository con GORM sobre user_stories y
// acceptance_criteria. Create y FindDetail están implementadas (T021); las demás operaciones las
// implementan las historias que las usan.
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

// FindAccessible resuelve el acceso a una historia para RequireStoryAccess. Firma mínima.
func (r *StoryRepository) FindAccessible(ctx context.Context, storyID, userID uuid.UUID) (*domain.StoryAccess, error) {
	return nil, errors.New("not implemented")
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
		if len(criteria) == 0 {
			return nil
		}
		rows := make([]domain.AcceptanceCriterion, 0, len(criteria))
		for i, text := range criteria {
			rows = append(rows, domain.AcceptanceCriterion{
				ID:       uuid.New(),
				StoryID:  story.ID,
				Position: i,
				Text:     text,
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("insertar criterios de aceptación: %w", err)
		}
		return nil
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

// List devuelve el backlog filtrado de un proyecto. Firma mínima.
func (r *StoryRepository) List(ctx context.Context, projectID uuid.UUID, filter domain.StoryFilter) ([]domain.StoryListItem, error) {
	return nil, errors.New("not implemented")
}

// HasStories indica si el proyecto tiene alguna historia. Firma mínima.
func (r *StoryRepository) HasStories(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

// Update aplica los cambios presentes de un PATCH. Firma mínima.
func (r *StoryRepository) Update(ctx context.Context, storyID uuid.UUID, changes domain.StoryChanges) error {
	return errors.New("not implemented")
}

// UpdateStoryPoints guarda la estimación de la historia. Firma mínima.
func (r *StoryRepository) UpdateStoryPoints(ctx context.Context, storyID uuid.UUID, storyPoints *int) error {
	return errors.New("not implemented")
}

// Delete borra la historia y sus criterios en una transacción. Firma mínima.
func (r *StoryRepository) Delete(ctx context.Context, storyID uuid.UUID) error {
	return errors.New("not implemented")
}

// Package repository implementa la persistencia del módulo de métricas con GORM.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// MetricsRepository lee los datos crudos de métricas.
type MetricsRepository struct {
	db *gorm.DB
}

// NewMetricsRepository crea el repositorio sobre la conexión indicada.
func NewMetricsRepository(db *gorm.DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

// readOnly ejecuta fn en una transacción REPEATABLE READ de solo lectura, de modo que todas las
// lecturas de una misma operación ven una única instantánea consistente (RN-02).
func (r *MetricsRepository) readOnly(ctx context.Context, fn func(tx *gorm.DB) error) error {
	opts := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	return r.db.WithContext(ctx).Transaction(fn, opts)
}

// LoadProject carga los datos del proyecto (firma mínima, pendiente de la historia que la usa).
func (r *MetricsRepository) LoadProject(ctx context.Context, projectID uuid.UUID) (*domain.ProjectData, error) {
	return nil, errors.New("not implemented")
}

// LoadSprint carga un sprint del proyecto con sus historias y su esfuerzo, y devuelve además el
// factor vigente del proyecto en centésimas de hora. Un sprint inexistente o de otro proyecto
// produce ErrSprintNotFound.
func (r *MetricsRepository) LoadSprint(ctx context.Context, projectID, sprintID uuid.UUID) (*domain.SprintData, int, error) {
	var data *domain.SprintData
	var factor int

	err := r.readOnly(ctx, func(tx *gorm.DB) error {
		// Factor vigente del proyecto.
		if err := tx.Raw("SELECT hours_per_story_point_hundredths FROM projects WHERE id = ?", projectID).
			Scan(&factor).Error; err != nil {
			return fmt.Errorf("leer el factor vigente del proyecto: %w", err)
		}

		// Sprint acotado al proyecto: otro proyecto equivale a inexistente.
		var sprint sprintsdomain.Sprint
		result := tx.Raw("SELECT * FROM sprints WHERE id = ? AND project_id = ?", sprintID, projectID).Scan(&sprint)
		if result.Error != nil {
			return fmt.Errorf("leer el sprint: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return domain.ErrSprintNotFound
		}

		stories, err := loadSprintStories(tx, sprint)
		if err != nil {
			return err
		}

		// Esfuerzo anclado al sprint, en centésimas de hora.
		var actual int
		if err := tx.Raw("SELECT COALESCE(SUM(hours_hundredths), 0) FROM effort_entries WHERE sprint_id = ?", sprintID).
			Scan(&actual).Error; err != nil {
			return fmt.Errorf("sumar el esfuerzo del sprint: %w", err)
		}

		data = &domain.SprintData{
			ID:                     sprint.ID,
			Name:                   sprint.Name,
			Status:                 sprint.Status,
			StartDate:              sprint.StartDate,
			EndDate:                sprint.EndDate,
			ClosedAt:               sprint.ClosedAt,
			FrozenFactorHundredths: sprint.HoursPerStoryPointHundredths,
			Stories:                stories,
			ActualHundredths:       actual,
			// Los defectos se cargan en US5.
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return data, factor, nil
}

// storyRow es una fila de historia comprometida tal como la devuelve la consulta.
type storyRow struct {
	StoryID     uuid.UUID
	Title       string
	StoryPoints *int
	Completed   bool
}

// loadSprintStories lee las historias del sprint: la instantánea congelada en un Cerrado y los
// compromisos abiertos con datos vigentes en el resto (research.md sección 8).
func loadSprintStories(tx *gorm.DB, sprint sprintsdomain.Sprint) ([]domain.SprintStoryData, error) {
	var rows []storyRow
	var err error
	if sprint.Status == sprintsdomain.SprintStatusClosed {
		err = tx.Raw(`SELECT ss.story_id AS story_id, COALESCE(ss.title_at_close, '') AS title,
				ss.story_points_at_close AS story_points, (ss.outcome = 'completed') AS completed
			FROM sprint_stories ss WHERE ss.sprint_id = ?`, sprint.ID).Scan(&rows).Error
	} else {
		err = tx.Raw(`SELECT ss.story_id AS story_id, us.title AS title,
				us.story_points AS story_points, (us.status = 'completed') AS completed
			FROM sprint_stories ss JOIN user_stories us ON us.id = ss.story_id
			WHERE ss.sprint_id = ? AND ss.outcome IS NULL`, sprint.ID).Scan(&rows).Error
	}
	if err != nil {
		return nil, fmt.Errorf("leer las historias del sprint: %w", err)
	}

	// Conversión de las filas a los datos de dominio.
	stories := make([]domain.SprintStoryData, 0, len(rows))
	for _, row := range rows {
		stories = append(stories, domain.SprintStoryData{
			StoryID:     row.StoryID,
			Title:       row.Title,
			StoryPoints: row.StoryPoints,
			Completed:   row.Completed,
		})
	}
	return stories, nil
}

// LoadActiveSprint carga el sprint Activo (firma mínima, pendiente de la historia que la usa).
func (r *MetricsRepository) LoadActiveSprint(ctx context.Context, projectID uuid.UUID) (*domain.SprintData, int, error) {
	return nil, 0, errors.New("not implemented")
}

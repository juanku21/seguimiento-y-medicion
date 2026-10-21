// Package repository persiste las entidades del módulo de proyectos con GORM e implementa los
// puertos hacia otros módulos: el resumen del backlog de la spec 003 y el directorio de sprints, que
// sigue siendo provisorio mientras la spec 004 no exista.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// BacklogSummaryProvider implementa domain.BacklogSummaryProvider sobre el backlog de la spec 003
// (research.md sección 13), leyendo las historias de usuario con una única consulta agrupada.
type BacklogSummaryProvider struct {
	db *gorm.DB
}

// NewBacklogSummaryProvider crea el proveedor del resumen del backlog sobre la conexión recibida.
func NewBacklogSummaryProvider(db *gorm.DB) *BacklogSummaryProvider {
	return &BacklogSummaryProvider{db: db}
}

// statusSummaryRow es una fila de la consulta agrupada: un estado, la cantidad de historias en él y
// la suma de sus Story Points estimados.
type statusSummaryRow struct {
	Status      string
	Total       int
	StoryPoints int
}

// Summarize agrupa por estado las historias del proyecto en una sola consulta: cuenta todas y suma
// solo las estimadas, porque SUM ignora los Story Points nulos y COALESCE lleva a 0 el estado que
// no tiene ninguna estimada (FR-045). Solo aparecen los estados con historias, y el mapa se
// devuelve vacío y no nil para que un backlog sin historias se serialice como objeto vacío (FR-044).
func (p *BacklogSummaryProvider) Summarize(ctx context.Context, projectID uuid.UUID) (domain.BacklogSummary, error) {
	var rows []statusSummaryRow
	err := p.db.WithContext(ctx).
		Model(&backlogdomain.UserStory{}).
		Select("status, COUNT(*) AS total, COALESCE(SUM(story_points), 0) AS story_points").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return domain.BacklogSummary{}, err
	}

	// Se vuelcan las filas al resumen acumulando el total de Story Points estimados.
	summary := domain.BacklogSummary{CountsByState: make(map[string]int, len(rows))}
	for _, row := range rows {
		summary.CountsByState[row.Status] = row.Total
		summary.EstimatedStoryPoints += row.StoryPoints
	}
	return summary, nil
}

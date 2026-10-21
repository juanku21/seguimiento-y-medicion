// Package repository persiste las entidades del módulo de proyectos con GORM e implementa los
// puertos hacia los módulos que todavía no existen.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// BacklogSummaryProvider implementa domain.BacklogSummaryProvider sobre el backlog de la spec 003
// (research.md sección 13). Por ahora conserva el cuerpo que informa vacío; la consulta agrupada
// sobre user_stories llega con T074.
type BacklogSummaryProvider struct {
	db *gorm.DB
}

// NewBacklogSummaryProvider crea el proveedor del resumen del backlog sobre la conexión recibida.
func NewBacklogSummaryProvider(db *gorm.DB) *BacklogSummaryProvider {
	return &BacklogSummaryProvider{db: db}
}

// Summarize informa un backlog vacío para cualquier proyecto: CountsByState vacío y
// EstimatedStoryPoints en 0. El mapa se devuelve vacío y no nil para que se serialice como objeto
// vacío y no como null.
func (p *BacklogSummaryProvider) Summarize(_ context.Context, _ uuid.UUID) (domain.BacklogSummary, error) {
	return domain.BacklogSummary{
		CountsByState:        map[string]int{},
		EstimatedStoryPoints: 0,
	}, nil
}

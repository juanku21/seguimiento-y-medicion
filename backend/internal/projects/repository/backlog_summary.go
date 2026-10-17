// Package repository persiste las entidades del módulo de proyectos con GORM e implementa los
// puertos hacia los módulos que todavía no existen.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// BacklogSummaryProvider implementa domain.BacklogSummaryProvider mientras la feature
// specs/003-product-backlog no exista (research.md sección 9). Cuando esa feature llegue, reemplaza
// el cuerpo de este archivo con la lectura real del backlog, sin tocar projects/service.
type BacklogSummaryProvider struct{}

// NewBacklogSummaryProvider crea el proveedor del resumen del backlog. No recibe dependencias porque
// hoy no consulta ninguna tabla.
func NewBacklogSummaryProvider() *BacklogSummaryProvider {
	return &BacklogSummaryProvider{}
}

// Summarize informa un backlog vacío para cualquier proyecto: CountsByState vacío y
// EstimatedStoryPoints en 0. No es un dato inventado: en el estado actual del producto ningún
// proyecto tiene historias, que es lo que FR-044 manda informar. El mapa se devuelve vacío y no nil
// para que se serialice como objeto vacío y no como null.
func (p *BacklogSummaryProvider) Summarize(_ context.Context, _ uuid.UUID) (domain.BacklogSummary, error) {
	return domain.BacklogSummary{
		CountsByState:        map[string]int{},
		EstimatedStoryPoints: 0,
	}, nil
}

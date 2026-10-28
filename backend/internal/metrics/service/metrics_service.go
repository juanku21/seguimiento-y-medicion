// Package service implementa la lógica de aplicación del módulo de métricas: toma los datos del
// repositorio y delega el cálculo en las funciones puras del dominio (Principio III).
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// MetricsService calcula las métricas de un proyecto y de sus sprints (firma mínima, pendiente de T014).
type MetricsService struct {
	repo  domain.MetricsRepository
	grace time.Duration
	now   func() time.Time
}

// NewMetricsService crea el servicio con el repositorio, el plazo de gracia y el reloj indicados
// (firma mínima, pendiente de T014).
func NewMetricsService(repo domain.MetricsRepository, grace time.Duration, now func() time.Time) *MetricsService {
	return &MetricsService{repo: repo, grace: grace, now: now}
}

// SprintMetrics devuelve las métricas de un sprint del proyecto (firma mínima, pendiente de T014).
func (s *MetricsService) SprintMetrics(ctx context.Context, projectID, sprintID uuid.UUID) (domain.SprintMetrics, []domain.Warning, error) {
	return domain.SprintMetrics{}, nil, errors.New("not implemented")
}

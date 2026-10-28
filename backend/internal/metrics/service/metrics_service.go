// Package service implementa la lógica de aplicación del módulo de métricas: toma los datos del
// repositorio y delega el cálculo en las funciones puras del dominio (Principio III).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
)

// MetricsService calcula las métricas de un proyecto y de sus sprints. Es también el punto de
// entrada que reutiliza el módulo de reportes dentro del proceso.
type MetricsService struct {
	repo  domain.MetricsRepository
	grace time.Duration
	now   func() time.Time
}

// NewMetricsService crea el servicio con el repositorio, el plazo de gracia para considerar un
// sprint cerrado como definitivo y el reloj (inyectable para las pruebas).
func NewMetricsService(repo domain.MetricsRepository, grace time.Duration, now func() time.Time) *MetricsService {
	return &MetricsService{repo: repo, grace: grace, now: now}
}

// SprintMetrics devuelve las métricas de un sprint del proyecto. Propaga ErrSprintNotFound sin
// perderlo para que la capa de entrega lo traduzca con errors.Is.
func (s *MetricsService) SprintMetrics(ctx context.Context, projectID, sprintID uuid.UUID) (domain.SprintMetrics, []domain.Warning, error) {
	// Se toma el instante actual una sola vez para que toda la consulta sea consistente.
	now := s.now()

	data, factor, err := s.repo.LoadSprint(ctx, projectID, sprintID)
	if err != nil {
		return domain.SprintMetrics{}, nil, fmt.Errorf("cargando el sprint %s: %w", sprintID, err)
	}
	// Defensa ante un repositorio que devuelva nil sin error.
	if data == nil {
		return domain.SprintMetrics{}, nil, fmt.Errorf("cargando el sprint %s: %w", sprintID, domain.ErrSprintNotFound)
	}

	// El cálculo es una función pura del dominio sobre los datos ya cargados.
	metrics, warnings := domain.CalculateSprint(*data, factor, now, s.grace)
	return metrics, warnings, nil
}

// ActiveSprintMetrics devuelve las métricas del sprint Activo del proyecto, calculadas con los datos
// y el factor vigentes. Devuelve nil sin error si el proyecto no tiene un sprint Activo (FR-016).
func (s *MetricsService) ActiveSprintMetrics(ctx context.Context, projectID uuid.UUID) (*domain.SprintMetrics, []domain.Warning, error) {
	// Se toma el instante actual una sola vez para que toda la consulta sea consistente.
	now := s.now()

	data, factor, err := s.repo.LoadActiveSprint(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("cargando el sprint activo del proyecto %s: %w", projectID, err)
	}
	// Sin sprint Activo no hay nada que calcular.
	if data == nil {
		return nil, nil, nil
	}

	// El cálculo es una función pura del dominio sobre los datos ya cargados.
	metrics, warnings := domain.CalculateSprint(*data, factor, now, s.grace)
	return &metrics, warnings, nil
}

// ProjectMetrics devuelve las métricas del proyecto (por ahora, la velocidad y sus advertencias).
func (s *MetricsService) ProjectMetrics(ctx context.Context, projectID uuid.UUID) (domain.ProjectMetrics, error) {
	// Se toma el instante actual una sola vez para que toda la consulta sea consistente.
	now := s.now()

	data, err := s.loadProject(ctx, projectID)
	if err != nil {
		return domain.ProjectMetrics{}, err
	}
	return domain.CalculateProject(*data, now, s.grace), nil
}

// ProjectSeries devuelve las seis series por sprint del proyecto y las advertencias de sus sprints.
func (s *MetricsService) ProjectSeries(ctx context.Context, projectID uuid.UUID) (domain.ProjectSeries, []domain.Warning, error) {
	// Se toma el instante actual una sola vez para que toda la consulta sea consistente.
	now := s.now()

	data, err := s.loadProject(ctx, projectID)
	if err != nil {
		return domain.ProjectSeries{}, nil, err
	}
	series, warnings := domain.BuildSeries(*data, now, s.grace)
	return series, warnings, nil
}

// loadProject carga los datos del proyecto y trata un resultado nil sin error como un fallo del
// repositorio, porque la membresía ya garantizó que el proyecto existe.
func (s *MetricsService) loadProject(ctx context.Context, projectID uuid.UUID) (*domain.ProjectData, error) {
	data, err := s.repo.LoadProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("cargando el proyecto %s: %w", projectID, err)
	}
	if data == nil {
		return nil, fmt.Errorf("cargando el proyecto %s: el repositorio no devolvió datos", projectID)
	}
	return data, nil
}

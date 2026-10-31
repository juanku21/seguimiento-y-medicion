// Package service contiene la lógica de negocio del módulo de reportes (Principio III).
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/reports/domain"
)

// ReportService arma los reportes de sprint y de proyecto a partir de las lecturas del repositorio
// y de las métricas del módulo de métricas.
type ReportService struct {
	repo        domain.ReportRepository
	generations domain.GenerationRepository
	metrics     domain.MetricsSource
	now         func() time.Time
}

// NewReportService crea el servicio con sus puertos y el reloj.
func NewReportService(repo domain.ReportRepository, generations domain.GenerationRepository, metrics domain.MetricsSource, now func() time.Time) *ReportService {
	return &ReportService{repo: repo, generations: generations, metrics: metrics, now: now}
}

// SprintReport arma el reporte de un sprint del proyecto.
func (s *ReportService) SprintReport(ctx context.Context, projectID, sprintID, userID uuid.UUID) (domain.Report, error) {
	return domain.Report{}, errors.New("no implementado")
}

// ProjectReport arma el reporte de todo el proyecto.
func (s *ReportService) ProjectReport(ctx context.Context, projectID, userID uuid.UUID) (domain.Report, error) {
	return domain.Report{}, errors.New("no implementado")
}

// Package service contiene la lógica de negocio del módulo de defectos (Principio III).
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectService orquesta los casos de uso de los defectos. Firma mínima: la lógica la aporta la
// tarea de implementación.
type DefectService struct {
	repo domain.DefectRepository
	now  func() time.Time
}

// NewDefectService crea el servicio con el repositorio y el reloj recibidos.
func NewDefectService(repo domain.DefectRepository, now func() time.Time) *DefectService {
	return &DefectService{repo: repo, now: now}
}

// Create registra un defecto en el proyecto atribuido al actor.
func (s *DefectService) Create(_ context.Context, _, _ uuid.UUID, _ domain.DefectInput) (*domain.DefectView, error) {
	return nil, errors.New("not implemented")
}

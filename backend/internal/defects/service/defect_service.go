// Package service contiene la lógica de negocio del módulo de defectos (Principio III).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectService orquesta los casos de uso de los defectos.
type DefectService struct {
	repo domain.DefectRepository
	now  func() time.Time
}

// NewDefectService crea el servicio con el repositorio y el reloj recibidos.
func NewDefectService(repo domain.DefectRepository, now func() time.Time) *DefectService {
	return &DefectService{repo: repo, now: now}
}

// Create registra un defecto en el proyecto atribuido al actor (el titular de la sesión).
func (s *DefectService) Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.DefectInput) (*domain.DefectView, error) {
	// Se toma el instante del reloj una sola vez, para que todas las marcas del registro coincidan.
	now := s.now()

	// El repositorio aplica las reglas de persistencia y devuelve el defecto con sus referencias.
	view, err := s.repo.Create(ctx, projectID, actorID, input, now)
	if err != nil {
		return nil, fmt.Errorf("registrar el defecto: %w", err)
	}
	return view, nil
}

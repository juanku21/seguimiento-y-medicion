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

// ChangeStatus cambia el estado de un defecto atribuyendo el cambio al actor (el titular de la sesión)
// y fechándolo con el instante del reloj. El repositorio decide la transición sobre el estado vigente.
func (s *DefectService) ChangeStatus(ctx context.Context, defectID, actorID uuid.UUID, change domain.StatusChange) (*domain.DefectView, error) {
	view, err := s.repo.ChangeStatus(ctx, defectID, actorID, change, s.now())
	if err != nil {
		return nil, fmt.Errorf("cambiar el estado del defecto: %w", err)
	}
	return view, nil
}

// Get devuelve el detalle de un defecto con sus relaciones resueltas.
func (s *DefectService) Get(ctx context.Context, defectID uuid.UUID) (*domain.DefectView, error) {
	view, err := s.repo.FindView(ctx, defectID)
	if err != nil {
		return nil, fmt.Errorf("leer el defecto: %w", err)
	}
	return view, nil
}

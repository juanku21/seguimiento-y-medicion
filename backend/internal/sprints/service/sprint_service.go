// Package service resuelve las reglas de negocio de los sprints sobre el repositorio y los puertos
// declarados en domain (Principio III).
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// SprintService resuelve las operaciones sobre los sprints de un proyecto. Depende del repositorio,
// del puerto de esfuerzo, de la zona horaria única del sistema y de un reloj inyectado que permite
// fijar los instantes en las pruebas.
type SprintService struct {
	repo     domain.SprintRepository
	effort   domain.SprintEffort
	location *time.Location
	now      func() time.Time
}

// NewSprintService compone el servicio con su firma definitiva, para que no cambie en las historias
// siguientes (T018).
func NewSprintService(repo domain.SprintRepository, effort domain.SprintEffort, location *time.Location, now func() time.Time) *SprintService {
	return &SprintService{repo: repo, effort: effort, location: location, now: now}
}

// SprintView es la vista de detalle de un sprint que devuelven las operaciones del servicio.
type SprintView struct {
	Detail *domain.SprintDetail
}

// Create registra un sprint en el proyecto atribuido al actor y devuelve su detalle. Firma mínima
// para las pruebas de T012: la regla se implementa en T018.
func (s *SprintService) Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.SprintInput) (*SprintView, error) {
	return nil, errors.New("not implemented")
}

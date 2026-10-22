// Package service resuelve las reglas de negocio de los sprints sobre el repositorio y los puertos
// declarados en domain (Principio III).
package service

import (
	"context"
	"fmt"
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

// NewSprintService compone el servicio con el repositorio, el puerto de esfuerzo, la zona horaria y
// el reloj. La firma es la definitiva de la feature: las historias siguientes usan las mismas
// dependencias sin cambiarla.
func NewSprintService(repo domain.SprintRepository, effort domain.SprintEffort, location *time.Location, now func() time.Time) *SprintService {
	return &SprintService{repo: repo, effort: effort, location: location, now: now}
}

// SprintView es la vista de detalle de un sprint que devuelven las operaciones del servicio. Las
// historias siguientes le agregan los datos derivados que arma buildDetail.
type SprintView struct {
	Detail *domain.SprintDetail
}

// Create registra un sprint en el proyecto atribuido al actor y devuelve su detalle (FR-006, FR-060).
// El sprint nace planned con un UUID nuevo y sin datos de inicio ni de cierre, sin consultar la
// propiedad del proyecto: cualquier integrante crea sprints (RN-01). El repositorio rechaza el
// proyecto Finalizado con ErrSprintsReadOnly y el período superpuesto con SprintOverlapError; el
// servicio los envuelve con %w para que delivery los reconozca con errors.Is y errors.As, y en ese
// caso no lee el detalle.
func (s *SprintService) Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.SprintInput) (*SprintView, error) {
	sprint := &domain.Sprint{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        input.Name,
		Goal:        input.Goal,
		StartDate:   input.StartDate,
		EndDate:     input.EndDate,
		Status:      domain.SprintStatusPlanned,
		CreatedByID: actorID,
	}
	if err := s.repo.Create(ctx, sprint); err != nil {
		return nil, fmt.Errorf("crear sprint: %w", err)
	}

	return s.buildDetail(ctx, sprint.ID)
}

// buildDetail arma la vista de un sprint a partir de su detalle persistido; la respuesta de toda
// escritura y de la consulta del detalle pasa por acá. Si el sprint no existe, el error envuelve
// ErrSprintNotFound del repositorio. Es el punto donde se agregan los datos derivados de la vista,
// como los días restantes calculados con la zona horaria y el reloj inyectados.
func (s *SprintService) buildDetail(ctx context.Context, sprintID uuid.UUID) (*SprintView, error) {
	detail, err := s.repo.FindDetail(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("leer detalle de sprint: %w", err)
	}

	return &SprintView{Detail: detail}, nil
}

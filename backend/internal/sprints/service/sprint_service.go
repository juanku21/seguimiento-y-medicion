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

// AssignStories compromete en el sprint del acceso un lote de historias, todo o nada, y devuelve el
// detalle actualizado (FR-017 a FR-021). Primero aplica la solo lectura del proyecto Finalizado y
// después el estado del sprint; superados ambos controles delega en el repositorio con el instante
// del reloj inyectado y propaga sus rechazos envueltos con %w, como StoriesNotAdmittedError.
func (s *SprintService) AssignStories(ctx context.Context, access *domain.SprintAccess, storyIDs []uuid.UUID) (*SprintView, error) {
	if _, err := s.ensureEditable(ctx, access); err != nil {
		return nil, err
	}
	if err := s.repo.AssignStories(ctx, access.SprintID, storyIDs, s.now()); err != nil {
		return nil, fmt.Errorf("asignar historias al sprint: %w", err)
	}

	return s.buildDetail(ctx, access.SprintID)
}

// RemoveStories quita del sprint del acceso un lote de historias, que vuelven al backlog como
// pending, y devuelve el detalle actualizado (FR-022, FR-023). Aplica los mismos controles que
// AssignStories y propaga envueltos los rechazos del repositorio, como ErrCompletedStoryRemoval.
func (s *SprintService) RemoveStories(ctx context.Context, access *domain.SprintAccess, storyIDs []uuid.UUID) (*SprintView, error) {
	if _, err := s.ensureEditable(ctx, access); err != nil {
		return nil, err
	}
	if err := s.repo.RemoveStories(ctx, access.SprintID, storyIDs); err != nil {
		return nil, fmt.Errorf("quitar historias del sprint: %w", err)
	}

	return s.buildDetail(ctx, access.SprintID)
}

// Start inicia el sprint del acceso y devuelve su detalle actualizado (FR-026 a FR-032). Aplica la
// solo lectura del proyecto Finalizado y el rechazo del sprint cerrado, y después valida la
// transición hacia active sin llegar al repositorio si no corresponde (RN-02). Superados los
// controles delega en el repositorio con el instante del reloj inyectado, que queda como inicio
// real, y propaga envueltos sus rechazos, como ActiveSprintExistsError (RN-06). No toca el estado
// del proyecto (RN-07).
func (s *SprintService) Start(ctx context.Context, access *domain.SprintAccess) (*SprintView, error) {
	detail, err := s.ensureEditable(ctx, access)
	if err != nil {
		return nil, err
	}
	if err := detail.Sprint.Status.ValidateTransition(domain.SprintStatusActive); err != nil {
		return nil, err
	}
	if err := s.repo.Start(ctx, access.SprintID, s.now()); err != nil {
		return nil, fmt.Errorf("iniciar sprint: %w", err)
	}

	return s.buildDetail(ctx, access.SprintID)
}

// Detail devuelve el detalle del sprint del acceso con sus historias vivas (FR-053). Es una lectura:
// no aplica la solo lectura del proyecto Finalizado ni el estado del sprint, que cualquier integrante
// puede consultar. La consulta del sprint (US6) le agrega la instantánea y los días restantes.
func (s *SprintService) Detail(ctx context.Context, access *domain.SprintAccess) (*SprintView, error) {
	return s.buildDetail(ctx, access.SprintID)
}

// ensureEditable aplica las guardas que comparten las escrituras sobre un sprint existente, sea
// sobre sus historias o sobre su estado: el proyecto Finalizado se rechaza con ErrSprintsReadOnly
// sin consultar el repositorio (RN-19) y el sprint cerrado con ErrSprintClosed. El repositorio
// repite ambos controles dentro de su transacción. Devuelve el detalle leído para que quien lo
// necesite, como Start, valide la transición de estado sin releerlo.
func (s *SprintService) ensureEditable(ctx context.Context, access *domain.SprintAccess) (*domain.SprintDetail, error) {
	if access.ProjectFinished {
		return nil, domain.ErrSprintsReadOnly
	}
	detail, err := s.repo.FindDetail(ctx, access.SprintID)
	if err != nil {
		return nil, fmt.Errorf("leer sprint a modificar: %w", err)
	}
	if detail.Sprint.Status == domain.SprintStatusClosed {
		return nil, domain.ErrSprintClosed
	}

	return detail, nil
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

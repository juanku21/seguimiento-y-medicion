// Package service resuelve las reglas de negocio de los sprints sobre el repositorio y los puertos
// declarados en domain (Principio III).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
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
// historias siguientes le agregan los datos derivados que arma buildDetail. HoursPerStoryPoint es
// el factor de horas por Story Point congelado al cierre, con la representación del factor del
// proyecto de la spec 002; es nil si el sprint no está cerrado (FR-043).
type SprintView struct {
	Detail             *domain.SprintDetail
	HoursPerStoryPoint *float64
	Schedule           *ScheduleView
}

// ScheduleView son los días restantes de un sprint active hasta su fecha de fin, o los días que
// lleva vencido, calculados con Schedule, el reloj y la zona inyectados (FR-053, RN-23). Exactamente
// uno de los dos valores es distinto de nil; la vista de un sprint planned o closed no lo lleva.
type ScheduleView struct {
	RemainingDays *int
	OverdueDays   *int
}

// EmptyReason explica por qué el listado de sprints vino vacío, para que la pantalla distinga el
// proyecto sin sprints del filtro sin coincidencias (FR-054). Viaja como texto en el campo
// emptyReason del contrato; su ausencia (nil) significa que hay resultados.
type EmptyReason string

// Motivos posibles de un listado vacío: el proyecto no tiene sprints, aunque se haya filtrado, o
// los tiene pero ninguno cumple el filtro de estado.
const (
	EmptyReasonNoSprints EmptyReason = "no_sprints"
	EmptyReasonNoMatches EmptyReason = "no_matches"
)

// SprintListView es el resultado del listado: los sprints en el orden de FR-052, nunca nil, y el
// motivo del vacío, nil cuando hay resultados.
type SprintListView struct {
	Sprints     []domain.SprintListItem
	EmptyReason *EmptyReason
}

// List devuelve los sprints del proyecto filtrados por estado, con el motivo del vacío cuando no
// hay resultados (FR-050 a FR-054). La consulta de existencia solo se hace si el resultado filtrado
// viene vacío, para distinguir el proyecto sin sprints (no_sprints, aunque haya filtro) del filtro
// sin coincidencias (no_matches); la lista devuelta nunca es nil (research.md sección 10).
func (s *SprintService) List(ctx context.Context, projectID uuid.UUID, statuses []domain.SprintStatus) (*SprintListView, error) {
	items, err := s.repo.List(ctx, projectID, statuses)
	if err != nil {
		return nil, fmt.Errorf("listar sprints: %w", err)
	}
	if len(items) > 0 {
		return &SprintListView{Sprints: items}, nil
	}

	// Resultado vacío: se consulta si el proyecto tiene algún sprint para elegir el motivo.
	hasSprints, err := s.repo.HasSprints(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("verificar existencia de sprints: %w", err)
	}
	reason := EmptyReasonNoSprints
	if hasSprints {
		reason = EmptyReasonNoMatches
	}

	return &SprintListView{Sprints: []domain.SprintListItem{}, EmptyReason: &reason}, nil
}

// StoryHistory devuelve los sprints en los que estuvo una historia, del más antiguo al más reciente,
// con su resultado en cada sprint Cerrado (FR-055). Es una lectura sin reglas de negocio: el acceso
// a la historia lo resuelve el middleware. La lista devuelta nunca es nil.
func (s *SprintService) StoryHistory(ctx context.Context, storyID uuid.UUID) ([]domain.StorySprintHistoryItem, error) {
	items, err := s.repo.ListForStory(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("listar sprints de historia: %w", err)
	}
	if items == nil {
		items = []domain.StorySprintHistoryItem{}
	}

	return items, nil
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

// Close cierra el sprint del acceso atribuido al actor y devuelve su detalle con la instantánea
// (FR-039 a FR-047). Aplica la solo lectura del proyecto Finalizado y el rechazo del sprint cerrado
// (RN-18, RN-19); después, solo un sprint active se cierra, y uno planned se rechaza con
// ErrSprintNotActive sin llegar al repositorio (RN-02; data-model.md sección 4). Superados los
// controles delega en el repositorio con el actor como closedBy y el instante del reloj inyectado,
// que queda como momento real de cierre, y propaga envueltos sus rechazos. No toca el estado del
// proyecto (RN-07).
func (s *SprintService) Close(ctx context.Context, access *domain.SprintAccess, actorID uuid.UUID) (*SprintView, error) {
	detail, err := s.ensureEditable(ctx, access)
	if err != nil {
		return nil, err
	}
	if detail.Sprint.Status != domain.SprintStatusActive {
		return nil, domain.ErrSprintNotActive
	}
	if err := s.repo.Close(ctx, access.SprintID, actorID, s.now()); err != nil {
		return nil, fmt.Errorf("cerrar sprint: %w", err)
	}

	return s.buildDetail(ctx, access.SprintID)
}

// ChangeStoryProgress cambia el estado de una historia del sprint activo con el instante del reloj
// (FR-033 a FR-038). Las guardas dependen de la historia bloqueada, así que las decide el
// repositorio dentro de su transacción: historia fuera del sprint activo, sprint cerrado, transición
// no admitida y proyecto Finalizado (RN-11, RN-18, RN-19). El servicio le pasa el instante del reloj
// inyectado, una sola vez, y propaga sus rechazos envueltos con %w para que delivery los reconozca
// con errors.Is.
func (s *SprintService) ChangeStoryProgress(ctx context.Context, storyID uuid.UUID, to backlogdomain.StoryStatus) error {
	if err := s.repo.ChangeStoryProgress(ctx, storyID, to, s.now()); err != nil {
		return fmt.Errorf("cambiar avance de historia: %w", err)
	}

	return nil
}

// FindOpenCommitment devuelve el compromiso abierto de la historia, o nil sin error si no tiene
// ninguno. Es una lectura sin reglas de negocio: delivery la usa después de ChangeStoryProgress para
// armar la respuesta del avance con el sprint y la fecha de completado registrada.
func (s *SprintService) FindOpenCommitment(ctx context.Context, storyID uuid.UUID) (*domain.SprintStory, error) {
	commitment, err := s.repo.FindOpenCommitment(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("leer compromiso abierto de historia: %w", err)
	}

	return commitment, nil
}

// Detail devuelve el detalle del sprint del acceso (FR-053). Es una lectura: no aplica la solo
// lectura del proyecto Finalizado ni el estado del sprint, que cualquier integrante puede consultar.
// La vista lleva la instantánea si el sprint está cerrado y los días restantes o vencidos si está
// active, ambos agregados por buildDetail.
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
// como los días restantes calculados con la zona horaria y el reloj inyectados. En un sprint closed
// FindDetail ya entrega las historias desde las filas congeladas, los totales de la instantánea
// (sprintResultsSelect) y quien lo cerró; acá se agrega el factor de horas por Story Point a partir
// de las centésimas copiadas al cerrar.
func (s *SprintService) buildDetail(ctx context.Context, sprintID uuid.UUID) (*SprintView, error) {
	detail, err := s.repo.FindDetail(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("leer detalle de sprint: %w", err)
	}

	view := &SprintView{Detail: detail}
	hundredths := detail.Sprint.HoursPerStoryPointHundredths
	if detail.Sprint.Status == domain.SprintStatusClosed && hundredths != nil {
		hours := float64(*hundredths) / 100
		view.HoursPerStoryPoint = &hours
	}

	// Solo un sprint active lleva los días restantes o vencidos, contados en la zona del sistema
	// con el instante del reloj inyectado (FR-053, RN-23).
	if detail.Sprint.Status == domain.SprintStatusActive {
		remaining, overdue := domain.Schedule(detail.Sprint.EndDate, s.now(), s.location)
		view.Schedule = &ScheduleView{RemainingDays: remaining, OverdueDays: overdue}
	}

	return view, nil
}

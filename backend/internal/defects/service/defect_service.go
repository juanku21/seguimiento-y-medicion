// Package service contiene la lógica de negocio del módulo de defectos (Principio III).
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
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

// Update aplica una modificación parcial de campos a un defecto atribuyéndola al actor (el titular de la
// sesión). El repositorio decide las reglas de edición y de vínculos sobre el estado vigente.
func (s *DefectService) Update(ctx context.Context, defectID, actorID uuid.UUID, patch domain.DefectPatch) (*domain.DefectView, error) {
	view, err := s.repo.Update(ctx, defectID, actorID, patch)
	if err != nil {
		return nil, fmt.Errorf("modificar el defecto: %w", err)
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

// EmptyReason explica por qué un listado de defectos vino vacío, para que la pantalla distinga el
// proyecto sin defectos de los filtros sin coincidencias. Viaja como texto en el campo emptyReason del
// contrato; su ausencia (nil) significa que hay resultados.
type EmptyReason string

// Motivos posibles de un listado vacío: no hay ningún defecto o los hay pero ninguno cumple el filtro.
const (
	EmptyReasonNoDefects EmptyReason = "no_defects"
	EmptyReasonNoMatches EmptyReason = "no_matches"
)

// DefectListView es el resultado de un listado: los defectos en el orden del repositorio, nunca nil, y
// el motivo del vacío, nil cuando hay resultados.
type DefectListView struct {
	Defects     []domain.DefectView
	EmptyReason *EmptyReason
}

// ListForProject lista los defectos del proyecto según el filtro. Primero verifica que el filtro
// corresponda al proyecto: el rechazo se envuelve con %w para que delivery lo reconozca con errors.As.
// Solo con la lista vacía se consulta si el proyecto tiene algún defecto para elegir el motivo.
func (s *DefectService) ListForProject(ctx context.Context, projectID uuid.UUID, filter domain.DefectFilter) (*DefectListView, error) {
	if err := s.repo.ValidateFilter(ctx, projectID, filter); err != nil {
		return nil, fmt.Errorf("validar el filtro de defectos: %w", err)
	}

	defects, err := s.repo.ListForProject(ctx, projectID, filter)
	if err != nil {
		return nil, fmt.Errorf("listar los defectos del proyecto: %w", err)
	}
	if len(defects) > 0 {
		return &DefectListView{Defects: defects}, nil
	}

	// Resultado vacío: se consulta si el proyecto tiene algún defecto para elegir el motivo.
	hasDefects, err := s.repo.HasProjectDefects(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("verificar si el proyecto tiene defectos: %w", err)
	}
	reason := EmptyReasonNoDefects
	if hasDefects {
		reason = EmptyReasonNoMatches
	}
	return &DefectListView{Defects: []domain.DefectView{}, EmptyReason: &reason}, nil
}

// ListForStory lista los defectos de la historia a la que el actor ya accedió. Una historia sin
// defectos siempre se explica con no_defects: este listado no admite filtros.
func (s *DefectService) ListForStory(ctx context.Context, access *backlogdomain.StoryAccess) (*DefectListView, error) {
	filter := domain.DefectFilter{StoryID: &access.StoryID}
	defects, err := s.repo.ListForProject(ctx, access.ProjectID, filter)
	if err != nil {
		return nil, fmt.Errorf("listar los defectos de la historia: %w", err)
	}
	if len(defects) > 0 {
		return &DefectListView{Defects: defects}, nil
	}
	reason := EmptyReasonNoDefects
	return &DefectListView{Defects: []domain.DefectView{}, EmptyReason: &reason}, nil
}

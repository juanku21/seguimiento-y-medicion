// Package service reúne los casos de uso del módulo de proyectos.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectService reúne los casos de uso de los proyectos sobre los repositorios de proyectos y de
// integrantes y los puertos de backlog y de sprints del dominio.
type ProjectService struct {
	projects domain.ProjectRepository
	members  domain.ProjectMemberRepository
	backlog  domain.BacklogSummaryProvider
	sprints  domain.SprintDirectory
}

// NewProjectService compone el servicio con los repositorios de proyectos y de integrantes y los dos
// puertos.
func NewProjectService(projects domain.ProjectRepository, members domain.ProjectMemberRepository, backlog domain.BacklogSummaryProvider, sprints domain.SprintDirectory) *ProjectService {
	return &ProjectService{projects: projects, members: members, backlog: backlog, sprints: sprints}
}

// CreateProjectInput son los datos de un proyecto nuevo ya validados y normalizados por delivery
// antes de llegar al servicio (Principio VII). NameNormalized llega calculado para que la regla de
// normalización del nombre viva en un solo lugar (RN-05), la descripción ya es nula si quedó vacía
// (FR-003) y el factor está expresado en centésimas de hora (RN-07).
type CreateProjectInput struct {
	Name                         string
	NameNormalized               string
	Description                  *string
	StartDate                    time.Time
	PlannedEndDate               time.Time
	HoursPerStoryPointHundredths int
}

// Create crea el proyecto del propietario en estado planned y sin fecha de finalización real, junto
// con la membresía que lo deja como primer integrante (FR-007, FR-008, RN-01), y devuelve su vista.
// Los dos registros se persisten en una sola transacción del repositorio (FR-010); un nombre ya
// usado por el mismo propietario llega como ErrProjectNameTaken y se propaga envuelto para que
// delivery lo reconozca con errors.Is (FR-004, FR-011).
func (s *ProjectService) Create(ctx context.Context, ownerID uuid.UUID, input CreateProjectInput) (*ProjectView, error) {
	project := &domain.Project{
		ID:                           uuid.New(),
		OwnerID:                      ownerID,
		Name:                         input.Name,
		NameNormalized:               input.NameNormalized,
		Description:                  input.Description,
		StartDate:                    input.StartDate,
		PlannedEndDate:               input.PlannedEndDate,
		ActualEndDate:                nil,
		HoursPerStoryPointHundredths: input.HoursPerStoryPointHundredths,
		Status:                       domain.ProjectStatusPlanned,
	}
	ownerMembership := &domain.ProjectMember{
		ID:        uuid.New(),
		ProjectID: project.ID,
		UserID:    ownerID,
	}
	if err := s.projects.Create(ctx, project, ownerMembership); err != nil {
		return nil, fmt.Errorf("crear proyecto: %w", err)
	}

	return s.buildView(ctx, project)
}

// Get devuelve la vista de estado del proyecto indicado (FR-042). La membresía de quien consulta ya
// la verificó delivery antes de llegar acá (FR-049), así que el servicio solo lee el proyecto y arma
// su vista con buildView. Un proyecto que ya no existe llega como ErrProjectNotFound y se propaga
// envuelto para que delivery lo reconozca con errors.Is.
func (s *ProjectService) Get(ctx context.Context, projectID uuid.UUID) (*ProjectView, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("buscar proyecto: %w", err)
	}

	return s.buildView(ctx, project)
}

// UpdateProjectInput son los cinco datos editables del proyecto, ya validados y normalizados por
// delivery con la misma validación que la creación (FR-012, FR-013); por eso comparte la forma de
// CreateProjectInput.
type UpdateProjectInput = CreateProjectInput

// Update modifica los cinco datos editables del proyecto indicado y devuelve su vista (FR-012 a
// FR-018). La propiedad ya la verificó delivery y la entrada llega validada con las mismas reglas
// que la creación (FR-013). Un proyecto finalizado se rechaza con ErrProjectReadOnly antes de
// escribir (FR-018, RN-11). El estado, la fecha de finalización real, la identidad y el propietario
// no se tocan (FR-016), y solo se escribe el proyecto, así que los sprints cerrados conservan su
// factor (FR-017, RN-15). Un nombre usado por otro proyecto del propietario llega del repositorio
// como ErrProjectNameTaken sin cambios parciales y se propaga envuelto (FR-014, FR-015).
func (s *ProjectService) Update(ctx context.Context, projectID uuid.UUID, input UpdateProjectInput) (*ProjectView, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("buscar proyecto: %w", err)
	}
	if project.Status == domain.ProjectStatusFinished {
		return nil, domain.ErrProjectReadOnly
	}

	// Se copia el proyecto antes de aplicar los cambios para no alterar el leído si el UPDATE falla.
	updated := *project
	updated.Name = input.Name
	updated.NameNormalized = input.NameNormalized
	updated.Description = input.Description
	updated.StartDate = input.StartDate
	updated.PlannedEndDate = input.PlannedEndDate
	updated.HoursPerStoryPointHundredths = input.HoursPerStoryPointHundredths
	if err := s.projects.Update(ctx, &updated); err != nil {
		return nil, fmt.Errorf("modificar proyecto: %w", err)
	}

	return s.buildView(ctx, &updated)
}

// emptyProjectListMessage es la indicación en español que acompaña al listado vacío, para que la
// ausencia de proyectos no se lea como un error (FR-034).
const emptyProjectListMessage = "Todavía no participás en ningún proyecto."

// ProjectList es el listado de proyectos que el servicio entrega a delivery para armar
// ProjectListResponse (FR-032). Projects nunca es nil y Message solo existe cuando la lista está
// vacía (FR-034).
type ProjectList struct {
	Projects []domain.ProjectListItem
	Message  *string
}

// List devuelve los proyectos de los que el usuario es integrante (FR-031 a FR-035). El filtro por
// membresía, el conteo de integrantes, la marca de propietario (comparación con projects.owner_id)
// y el orden los resuelve ListForMember en una sola consulta; el servicio solo agrega la indicación
// de lista vacía. Un fallo del repositorio se propaga con su contexto.
func (s *ProjectService) List(ctx context.Context, userID uuid.UUID) (*ProjectList, error) {
	items, err := s.projects.ListForMember(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listar proyectos: %w", err)
	}

	list := &ProjectList{Projects: items}
	if len(items) == 0 {
		message := emptyProjectListMessage
		list.Projects = []domain.ProjectListItem{}
		list.Message = &message
	}
	return list, nil
}

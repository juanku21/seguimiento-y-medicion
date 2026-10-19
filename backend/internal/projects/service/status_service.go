package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// StatusService resuelve el cambio de estado del proyecto y el registro de la fecha de finalización
// real (FR-037 a FR-041). Va separado de ProjectService porque depende del puerto de sprints, de la
// zona horaria del sistema y de un reloj inyectado que permite fijar el instante del cambio.
type StatusService struct {
	projects domain.ProjectRepository
	sprints  domain.SprintDirectory
	location *time.Location
	now      func() time.Time
}

// NewStatusService compone el servicio con el repositorio de proyectos, el puerto de sprints, la zona
// horaria única del sistema (APP_TIMEZONE) y el reloj del que se deriva la fecha del cambio.
func NewStatusService(projects domain.ProjectRepository, sprints domain.SprintDirectory, location *time.Location, now func() time.Time) *StatusService {
	return &StatusService{
		projects: projects,
		sprints:  sprints,
		location: location,
		now:      now,
	}
}

// ChangeStatus hace avanzar el proyecto al estado destino y devuelve el proyecto actualizado. Un
// proyecto finalizado rechaza todo cambio con ErrProjectReadOnly antes de validar la transición
// (FR-040, RN-10), de modo que la fecha real ya registrada nunca se toca. Para finalizar, se consulta
// el sprint activo y, si existe, se rechaza sin escribir (FR-041, RN-16); si no, se fija la fecha
// real con la fecha de calendario del instante del cambio en la zona del sistema (FR-039).
func (s *StatusService) ChangeStatus(ctx context.Context, projectID uuid.UUID, to domain.ProjectStatus) (*domain.Project, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("buscar proyecto: %w", err)
	}
	if project.Status == domain.ProjectStatusFinished {
		return nil, domain.ErrProjectReadOnly
	}
	if err := project.Status.ValidateTransition(to); err != nil {
		return nil, err
	}

	// La finalización exige que no haya sprint activo y registra la fecha real del cambio.
	if to == domain.ProjectStatusFinished {
		active, err := s.sprints.FindActiveSprint(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("consultar sprint activo: %w", err)
		}
		if active != nil {
			return nil, domain.ErrActiveSprintBlocksFinish
		}
		actualEnd := s.today()
		project.ActualEndDate = &actualEnd
	}

	project.Status = to
	if err := s.projects.UpdateStatus(ctx, project); err != nil {
		return nil, fmt.Errorf("guardar cambio de estado: %w", err)
	}
	return project, nil
}

// today devuelve la fecha de calendario del instante actual en la zona del sistema, como medianoche
// UTC sin hora significativa, que es como se representan las columnas date (research.md sección 4).
func (s *StatusService) today() time.Time {
	year, month, day := s.now().In(s.location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

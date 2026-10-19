package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// StatusService resuelve el cambio de estado del proyecto y el registro de la fecha de finalización
// real (FR-037 a FR-041). Firma mínima de T071; la regla la implementa T075.
type StatusService struct{}

// NewStatusService compone el servicio con el repositorio de proyectos, el puerto de sprints, la zona
// horaria única del sistema y el reloj del que se deriva la fecha del cambio. Firma mínima de T071.
func NewStatusService(projects domain.ProjectRepository, sprints domain.SprintDirectory, location *time.Location, now func() time.Time) *StatusService {
	return &StatusService{}
}

// ChangeStatus hace avanzar el proyecto al estado destino y devuelve el proyecto actualizado. Firma
// mínima de T071; la regla la implementa T075.
func (s *StatusService) ChangeStatus(ctx context.Context, projectID uuid.UUID, to domain.ProjectStatus) (*domain.Project, error) {
	return nil, errors.New("not implemented")
}

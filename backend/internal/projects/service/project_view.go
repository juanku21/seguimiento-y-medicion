package service

import (
	"context"
	"fmt"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// ProjectView es el estado completo de un proyecto que el servicio entrega a delivery para armar
// ProjectDetail (FR-042). ActiveSprint es nil cuando el proyecto no tiene sprint activo, que no es
// un error (FR-043). EstimatedHoursHundredths son las horas estimadas del backlog en centésimas
// enteras, que delivery divide por 100 una sola vez al serializar (RN-08). ReadOnly es verdadero
// cuando el proyecto está finalizado (FR-018). No decide qué se muestra a cada integrante: la
// omisión del correo la resuelve delivery.
type ProjectView struct {
	Project                  *domain.Project
	Members                  []domain.MemberView
	ActiveSprint             *domain.ActiveSprint
	Backlog                  domain.BacklogSummary
	EstimatedHoursHundredths int
	ReadOnly                 bool
}

// buildView completa la vista del proyecto con los integrantes que lista el repositorio de
// integrantes, el resumen del backlog y el sprint activo que informan los dos puertos, y las horas
// estimadas calculadas con el factor del proyecto (FR-030, FR-042 a FR-045). Un fallo del
// repositorio o de cualquiera de los puertos se propaga con su contexto.
func (s *ProjectService) buildView(ctx context.Context, project *domain.Project) (*ProjectView, error) {
	members, err := s.members.ListMembers(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("listar integrantes del proyecto: %w", err)
	}
	backlog, err := s.backlog.Summarize(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("resumir backlog del proyecto: %w", err)
	}
	activeSprint, err := s.sprints.FindActiveSprint(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("buscar sprint activo del proyecto: %w", err)
	}

	return &ProjectView{
		Project:      project,
		Members:      members,
		ActiveSprint: activeSprint,
		Backlog:      backlog,
		EstimatedHoursHundredths: EstimatedHoursHundredths(
			backlog.EstimatedStoryPoints,
			project.HoursPerStoryPointHundredths,
		),
		ReadOnly: project.Status == domain.ProjectStatusFinished,
	}, nil
}

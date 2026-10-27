package delivery

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
)

// DefectService declara lo que el manejador necesita del servicio de defectos.
type DefectService interface {
	// Create registra un defecto en el proyecto atribuido al actor.
	Create(ctx context.Context, projectID, actorID uuid.UUID, input domain.DefectInput) (*domain.DefectView, error)
}

// Handler expone los endpoints HTTP de los defectos. Firma mínima: los manejadores los implementa la
// tarea correspondiente.
type Handler struct {
	svc DefectService
}

// NewHandler crea el manejador sobre el servicio recibido.
func NewHandler(svc DefectService) *Handler {
	return &Handler{svc: svc}
}

// createDefect atiende POST /api/v1/projects/:projectId/defects.
func (h *Handler) createDefect(_ *gin.Context) {}

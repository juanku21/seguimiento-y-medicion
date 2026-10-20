package delivery

import (
	"encoding/json"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// StoryInput es el cuerpo de la creación de una historia (contracts/openapi.yaml). Los Story Points
// llegan como json.Number para validarlos sobre su texto literal; nil representa "sin estimar"
// (research.md sección 4). Firma mínima para las pruebas de T013; T023 completa la documentación
// del contrato para Swagger.
type StoryInput struct {
	Title              string               `json:"title"`
	Description        *string              `json:"description"`
	Priority           domain.StoryPriority `json:"priority"`
	StoryPoints        *json.Number         `json:"storyPoints"`
	AcceptanceCriteria []string             `json:"acceptanceCriteria"`
}

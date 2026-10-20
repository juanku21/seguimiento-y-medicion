package delivery

import (
	authdelivery "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/delivery"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// ValidatedStory son los datos de una historia ya validados y normalizados: título y criterios
// recortados, descripción nula si queda vacía y Story Points nulos si la historia está sin estimar.
type ValidatedStory struct {
	Title              string
	Description        *string
	Priority           domain.StoryPriority
	StoryPoints        *int
	AcceptanceCriteria []string
}

// ValidateStoryInput aplica las reglas RN-03 a RN-08 a la entrada de la creación y devuelve los
// datos normalizados junto con un FieldError por cada campo inválido. Firma mínima para las pruebas
// de T013; la implementación corresponde a T019.
func ValidateStoryInput(req StoryInput) (ValidatedStory, []authdelivery.FieldError) {
	return ValidatedStory{}, nil
}

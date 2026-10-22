package domain

import (
	"errors"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// ValidateStoryProgress decide si una historia del sprint Activo puede pasar del estado from al
// estado to (data-model.md sección 5; FR-033, FR-038). Firma mínima de T052: la regla la implementa
// T057.
func ValidateStoryProgress(from, to backlogdomain.StoryStatus) error {
	return errors.New("not implemented")
}

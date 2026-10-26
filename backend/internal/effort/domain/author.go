package domain

import (
	"errors"

	"github.com/google/uuid"
)

// EnsureAuthor verifica que quien intenta modificar o eliminar un registro de esfuerzo sea su autor;
// el propietario del proyecto no es excepción (FR-022, RN-11, data-model.md sección 6.6).
func EnsureAuthor(authorID, actorID uuid.UUID) error {
	return errors.New("not implemented")
}

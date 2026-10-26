package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de EnsureAuthor (T041, RN-11) con la regla de data-model.md sección
// 6.6: solo el autor de un registro lo modifica o lo elimina; cualquier otra persona, incluido el
// propietario del proyecto, recibe ErrNotEntryAuthor.

// El autor del registro puede modificarlo o eliminarlo (006/US3, RN-11).
func TestEnsureAuthor_Author(t *testing.T) {
	authorID := uuid.New()

	assert.NoError(t, EnsureAuthor(authorID, authorID))
}

// Otro integrante del proyecto no puede modificar ni eliminar el registro (006/US3, RN-11).
func TestEnsureAuthor_OtherMember(t *testing.T) {
	authorID, memberID := uuid.New(), uuid.New()

	assert.ErrorIs(t, EnsureAuthor(authorID, memberID), ErrNotEntryAuthor)
}

// El propietario del proyecto tampoco puede modificar ni eliminar un registro ajeno (006/US3, RN-11).
func TestEnsureAuthor_ProjectOwner(t *testing.T) {
	authorID, ownerID := uuid.New(), uuid.New()

	assert.ErrorIs(t, EnsureAuthor(authorID, ownerID), ErrNotEntryAuthor)
}

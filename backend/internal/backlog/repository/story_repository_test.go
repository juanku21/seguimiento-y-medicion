package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración de StoryRepository.Delete frente a defectos (T012, US1). Reutilizan el
// fixture de sprint_assignments_test.go y corren contra la base de test.

// Si un defecto se registra después de que el servicio consultó HasDefects y la marca todavía está en
// false (la carrera de research.md sección 10), Delete traduce la violación de fk_defects_story a
// DeletionBlockedError con el motivo defects, y la historia no se borra (RC-05).
func TestStoryDelete_DefectRace(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusActive,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	f.seedDefect(t, storyID, sprintID)

	err := repository.NewStoryRepository(f.db).Delete(context.Background(), storyID)

	var blocked *backlogdomain.DeletionBlockedError
	require.ErrorAs(t, err, &blocked, "debe devolver DeletionBlockedError y no un error genérico")
	assert.Equal(t, []backlogdomain.DeletionBlocker{backlogdomain.DeletionBlockerDefects}, blocked.Reasons)
	var count int64
	require.NoError(t, f.db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "la historia no debe borrarse")
}

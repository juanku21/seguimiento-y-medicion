package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	defectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/defects/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del puerto DefectRecords de la 003 reemplazado por la 007 (T012, US1). Corren
// contra la base de test y reutilizan el fixture de sprint_assignments_test.go: siembran la historia y
// los defectos tal como los deja el alta, con ever_had_defects en true, para verificar que HasDefects
// lee esa marca y no la existencia actual de defectos (RC-05, RN-19).

// seedDefect inserta un defecto abierto de la historia, detectado en el sprint indicado y atribuido al
// autor del fixture, directamente en defects sin tocar ever_had_defects, y devuelve su identificador.
func (f assignmentsFixture) seedDefect(t *testing.T, storyID, sprintID uuid.UUID) uuid.UUID {
	t.Helper()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	defect := defectsdomain.Defect{
		ID:                uuid.New(),
		StoryID:           storyID,
		Description:       "El formulario pierde los datos al volver atrás",
		Severity:          defectsdomain.DefectSeverityHigh,
		Status:            defectsdomain.DefectStatusOpen,
		DetectedSprintID:  sprintID,
		CreatedByID:       f.authorID,
		UpdatedByID:       f.authorID,
		StatusChangedByID: f.authorID,
		StatusChangedAt:   now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	require.NoError(t, f.db.Create(&defect).Error, "no se pudo sembrar el defecto")
	return defect.ID
}

// markEverHadDefects deja ever_had_defects en true, como lo hace el alta del defecto.
func markEverHadDefects(t *testing.T, db *gorm.DB, storyID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).
		UpdateColumn("ever_had_defects", true).Error, "no se pudo marcar ever_had_defects")
}

// Una historia sin defectos ni la marca ever_had_defects no tiene defectos (007/US1, RN-19).
func TestDefectRecords_SinDefectos(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)

	got, err := repository.NewDefectRecords(f.db).HasDefects(context.Background(), storyID)

	require.NoError(t, err)
	assert.False(t, got, "sin defectos, HasDefects debe ser false")
}

// Una historia con un defecto y ever_had_defects en true, como la deja el alta, tiene defectos
// (007/US1, RN-19).
func TestDefectRecords_ConDefecto(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusActive,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	f.seedDefect(t, storyID, sprintID)
	markEverHadDefects(t, f.db, storyID)

	got, err := repository.NewDefectRecords(f.db).HasDefects(context.Background(), storyID)

	require.NoError(t, err)
	assert.True(t, got, "con un defecto y ever_had_defects, HasDefects debe ser true")
}

// Después de mover el único defecto a otra historia, la historia original sigue marcada y HasDefects
// sigue en true, porque lee ever_had_defects y no la existencia actual de defectos (RC-05, RN-19).
func TestDefectRecords_TrasMoverElUnicoDefecto(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)
	otherStoryID := f.seedStory(t, false)
	sprintID := f.seedSprint(t, sprintsdomain.SprintStatusActive,
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	defectID := f.seedDefect(t, storyID, sprintID)
	markEverHadDefects(t, f.db, storyID)
	require.NoError(t, f.db.Model(&defectsdomain.Defect{}).Where("id = ?", defectID).
		UpdateColumn("story_id", otherStoryID).Error, "no se pudo mover el defecto")

	got, err := repository.NewDefectRecords(f.db).HasDefects(context.Background(), storyID)

	require.NoError(t, err)
	assert.True(t, got, "tras mover el único defecto, HasDefects debe seguir en true")
}

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/domain"
	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/repository"
	effortdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/effort/domain"
)

// Pruebas de integración del puerto EffortRecords de la 003 reemplazado por la 006 (T017, US1). Corren
// contra la base de test y reutilizan el fixture de sprint_assignments_test.go: siembran la historia y
// los registros de effort_entries tal como los deja el alta, con ever_had_effort en true, para
// verificar que HasEffort lee esa marca y no la existencia actual de registros (RN-22), y que las
// claves foráneas NO ACTION de effort_entries impiden borrar en la base una historia o un usuario con
// registros (RN-21).

// seedEffortEntry inserta un registro de esfuerzo de la historia, del autor indicado y sin sprint,
// directamente en effort_entries, y devuelve su identificador.
func seedEffortEntry(t *testing.T, db *gorm.DB, storyID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	entry := effortdomain.EffortEntry{
		ID:              uuid.New(),
		StoryID:         storyID,
		UserID:          userID,
		EntryDate:       time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Activity:        "Desarrollo del alta de clientes",
		HoursHundredths: 300,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, db.Create(&entry).Error, "no se pudo sembrar el registro de esfuerzo")
	return entry.ID
}

// markEverHadEffort deja ever_had_effort en true, como lo hace el alta del registro.
func markEverHadEffort(t *testing.T, db *gorm.DB, storyID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).
		UpdateColumn("ever_had_effort", true).Error, "no se pudo marcar ever_had_effort")
}

// seedEffortAuthor inserta un integrante que solo es autor de registros, sin proyectos ni historias
// propias, para que su borrado no choque con otra clave foránea que la de effort_entries.
func seedEffortAuthor(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	user := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Bruno Díaz",
		Email:        "bruno@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&user).Error, "no se pudo sembrar el autor de los registros")
	return user.ID
}

// Una historia sin registros de esfuerzo ni la marca ever_had_effort no tiene esfuerzo
// (006/US1, RN-22).
func TestEffortRecords_SinRegistros(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)

	got, err := repository.NewEffortRecords(f.db).HasEffort(context.Background(), storyID)

	require.NoError(t, err)
	assert.False(t, got, "sin registros, HasEffort debe ser false")
}

// Una historia con un registro de esfuerzo y ever_had_effort en true, como la deja el alta, tiene
// esfuerzo (006/US1, RN-22).
func TestEffortRecords_ConRegistro(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)
	seedEffortEntry(t, f.db, storyID, f.authorID)
	markEverHadEffort(t, f.db, storyID)

	got, err := repository.NewEffortRecords(f.db).HasEffort(context.Background(), storyID)

	require.NoError(t, err)
	assert.True(t, got, "con un registro y ever_had_effort, HasEffort debe ser true")
}

// Después de borrar todos los registros de la historia, HasEffort sigue en true porque lee
// ever_had_effort y no la existencia actual de registros (006/US1, RN-22).
func TestEffortRecords_TrasBorrarTodosLosRegistros(t *testing.T) {
	f := newAssignmentsFixture(t)
	storyID := f.seedStory(t, false)
	seedEffortEntry(t, f.db, storyID, f.authorID)
	seedEffortEntry(t, f.db, storyID, f.authorID)
	markEverHadEffort(t, f.db, storyID)
	require.NoError(t, f.db.Where("story_id = ?", storyID).Delete(&effortdomain.EffortEntry{}).Error,
		"no se pudieron borrar los registros")

	got, err := repository.NewEffortRecords(f.db).HasEffort(context.Background(), storyID)

	require.NoError(t, err)
	assert.True(t, got, "tras borrar todos los registros, HasEffort debe seguir en true")
}

// El DELETE directo en la base de una historia o de un usuario con registros de esfuerzo falla por la
// clave foránea NO ACTION de effort_entries, y el registro sigue existiendo (006/US1, RN-21).
func TestEffortRecords_BorradoEnBaseBloqueado(t *testing.T) {
	t.Run("historia con registros", func(t *testing.T) {
		f := newAssignmentsFixture(t)
		storyID := f.seedStory(t, false)
		entryID := seedEffortEntry(t, f.db, storyID, f.authorID)

		err := f.db.Where("id = ?", storyID).Delete(&backlogdomain.UserStory{}).Error

		assert.ErrorIs(t, err, gorm.ErrForeignKeyViolated)
		var count int64
		require.NoError(t, f.db.Model(&effortdomain.EffortEntry{}).Where("id = ?", entryID).Count(&count).Error)
		assert.Equal(t, int64(1), count, "el registro no debe borrarse como efecto colateral")
	})

	t.Run("usuario con registros", func(t *testing.T) {
		f := newAssignmentsFixture(t)
		storyID := f.seedStory(t, false)
		authorID := seedEffortAuthor(t, f.db)
		entryID := seedEffortEntry(t, f.db, storyID, authorID)

		err := f.db.Where("id = ?", authorID).Delete(&authdomain.User{}).Error

		assert.ErrorIs(t, err, gorm.ErrForeignKeyViolated)
		var count int64
		require.NoError(t, f.db.Model(&effortdomain.EffortEntry{}).Where("id = ?", entryID).Count(&count).Error)
		assert.Equal(t, int64(1), count, "el registro no debe borrarse como efecto colateral")
	})
}

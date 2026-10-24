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
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/platform/testsupport"
	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas de integración del puerto EstimationSessions reemplazado por la 005 (T086, US8). Corren
// contra la base de test y siembran directamente filas de estimation_sessions en cada estado, para
// verificar que HasSessions informa si la historia tiene al menos una sesión, en cualquier estado, y
// que no cuenta las de otra historia (RN-33, SC-023, research.md sección 15.1).

// sessionsFixture agrupa la conexión de test, la facilitadora, el proyecto y el puerto bajo prueba.
type sessionsFixture struct {
	db            *gorm.DB
	facilitatorID uuid.UUID
	projectID     uuid.UUID
	port          *repository.EstimationSessions
}

// newSessionsFixture abre la base de test vacía, siembra una usuaria y un proyecto en curso suyo y
// compone el puerto sobre esa conexión.
func newSessionsFixture(t *testing.T) sessionsFixture {
	t.Helper()
	db, _ := testsupport.OpenDatabase(t)
	facilitator := authdomain.User{
		ID:           uuid.New(),
		FullName:     "Ana Pérez",
		Email:        "ana@example.com",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
	}
	require.NoError(t, db.Create(&facilitator).Error, "no se pudo sembrar la usuaria")
	project := projectsdomain.Project{
		ID:                           uuid.New(),
		OwnerID:                      facilitator.ID,
		Name:                         "portal de clientes",
		NameNormalized:               "portal de clientes",
		StartDate:                    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		PlannedEndDate:               time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		HoursPerStoryPointHundredths: 650,
		Status:                       projectsdomain.ProjectStatusInProgress,
	}
	require.NoError(t, db.Create(&project).Error, "no se pudo sembrar el proyecto")
	return sessionsFixture{db: db, facilitatorID: facilitator.ID, projectID: project.ID, port: repository.NewEstimationSessions(db)}
}

// seedStory inserta una historia pending del proyecto con el título recibido y devuelve su
// identificador.
func (f sessionsFixture) seedStory(t *testing.T, title string) uuid.UUID {
	t.Helper()
	story := backlogdomain.UserStory{
		ID:          uuid.New(),
		ProjectID:   f.projectID,
		Title:       title,
		Priority:    backlogdomain.StoryPriorityMedium,
		Status:      backlogdomain.StoryStatusPending,
		CreatedByID: f.facilitatorID,
	}
	require.NoError(t, f.db.Create(&story).Error, "no se pudo sembrar la historia")
	return story.ID
}

// seedSession inserta una sesión de la historia en el estado indicado, con los campos de cierre
// coherentes con ese estado: valor acordado si terminó finished y cierre manual si terminó canceled.
func (f sessionsFixture) seedSession(t *testing.T, storyID uuid.UUID, status pokerdomain.SessionStatus) {
	t.Helper()
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)
	session := pokerdomain.EstimationSession{
		ID:             uuid.New(),
		StoryID:        storyID,
		FacilitatorID:  f.facilitatorID,
		Status:         status,
		OpenedByID:     f.facilitatorID,
		LastActivityAt: now,
	}
	if status != pokerdomain.SessionStatusOpen {
		closedBy := f.facilitatorID
		session.ClosedByID = &closedBy
		session.ClosedAt = &now
	}
	if status == pokerdomain.SessionStatusFinished {
		agreed := 5
		session.AgreedStoryPoints = &agreed
	}
	require.NoError(t, f.db.Create(&session).Error, "no se pudo sembrar la sesión")
}

// Escenario: Eliminación bloqueada (005/US8)
// El puerto que consulta la 003 antes de eliminar una historia informa si tiene historial de
// estimación: sin sesiones responde false; con una sola sesión open, finished o canceled responde
// true, porque cualquier estado bloquea la eliminación; y las sesiones de otra historia del mismo
// proyecto no cuentan (RN-33, SC-023).
func TestUS8_EliminacionBloqueada(t *testing.T) {
	// Una historia sin ninguna sesión de estimación se puede eliminar.
	t.Run("sin sesiones", func(t *testing.T) {
		f := newSessionsFixture(t)
		storyID := f.seedStory(t, "Alta de clientes")

		has, err := f.port.HasSessions(context.Background(), storyID)

		require.NoError(t, err)
		assert.False(t, has, "una historia sin sesiones no debería tener historial de estimación")
	})

	// Una sola sesión en cualquiera de los tres estados alcanza para bloquear la eliminación.
	for _, status := range []pokerdomain.SessionStatus{
		pokerdomain.SessionStatusOpen,
		pokerdomain.SessionStatusFinished,
		pokerdomain.SessionStatusCanceled,
	} {
		t.Run("con una sesión "+string(status), func(t *testing.T) {
			f := newSessionsFixture(t)
			storyID := f.seedStory(t, "Alta de clientes")
			f.seedSession(t, storyID, status)

			has, err := f.port.HasSessions(context.Background(), storyID)

			require.NoError(t, err)
			assert.True(t, has, "una historia con una sesión %s debería tener historial de estimación", status)
		})
	}

	// Las sesiones de otra historia, en los tres estados, no se cuentan para la historia consultada.
	t.Run("sesiones de otra historia", func(t *testing.T) {
		f := newSessionsFixture(t)
		storyID := f.seedStory(t, "Alta de clientes")
		otherID := f.seedStory(t, "Baja de clientes")
		f.seedSession(t, otherID, pokerdomain.SessionStatusFinished)
		f.seedSession(t, otherID, pokerdomain.SessionStatusCanceled)
		f.seedSession(t, otherID, pokerdomain.SessionStatusOpen)

		has, err := f.port.HasSessions(context.Background(), storyID)

		require.NoError(t, err)
		assert.False(t, has, "las sesiones de otra historia no deberían contar")

		// Control: la otra historia sí tiene historial, para que el false anterior no sea trivial.
		other, err := f.port.HasSessions(context.Background(), otherID)
		require.NoError(t, err)
		assert.True(t, other, "la otra historia debería tener historial de estimación")
	})
}

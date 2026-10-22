package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/repository"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración del puerto SprintDirectory de la 002 (T043, 004/US3). Corren contra la
// base de test y siembran los sprints directamente sobre el modelo sprintsdomain.Sprint, que es el
// que lee el directorio. Verifican que solo el sprint active del proyecto consultado cuenta, y que
// su ausencia se informa como (nil, nil) según FR-043 de la 002.

// seedSprint inserta un sprint del proyecto con el nombre y el estado indicados y devuelve su
// identificador. Cada unidad de slot corre el período 14 días, de modo que los sprints sembrados
// con slots distintos no se superponen.
func (f summaryFixture) seedSprint(t *testing.T, projectID uuid.UUID, name string, status sprintsdomain.SprintStatus, slot int) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 14*slot)
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        name,
		Goal:        "Objetivo de prueba",
		StartDate:   start,
		EndDate:     start.AddDate(0, 0, 13),
		Status:      status,
		CreatedByID: f.authorID,
	}
	require.NoError(t, f.db.Create(&sprint).Error, "no se pudo sembrar el sprint")
	return sprint.ID
}

// TestSprintDirectory_FindActiveSprint verifica que el directorio informe solo el sprint active del
// proyecto consultado.
func TestSprintDirectory_FindActiveSprint(t *testing.T) {
	// Sin ningún sprint el proyecto no tiene sprint activo, y eso no es un error.
	t.Run("sin sprints devuelve nil sin error", func(t *testing.T) {
		f := newSummaryFixture(t)
		projectID := f.seedProject(t, "portal de clientes")

		got, err := repository.NewSprintDirectory(f.db).FindActiveSprint(context.Background(), projectID)

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	// Los sprints Planificados y Cerrados no cuentan como activos.
	t.Run("solo planificados y cerrados devuelve nil sin error", func(t *testing.T) {
		f := newSummaryFixture(t)
		projectID := f.seedProject(t, "portal de clientes")
		f.seedSprint(t, projectID, "Sprint 1", sprintsdomain.SprintStatusClosed, 0)
		f.seedSprint(t, projectID, "Sprint 2", sprintsdomain.SprintStatusPlanned, 1)

		got, err := repository.NewSprintDirectory(f.db).FindActiveSprint(context.Background(), projectID)

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	// Con un sprint Activo entre otros, el directorio devuelve su identificador y su nombre.
	t.Run("con un activo devuelve su ID y su nombre", func(t *testing.T) {
		f := newSummaryFixture(t)
		projectID := f.seedProject(t, "portal de clientes")
		f.seedSprint(t, projectID, "Sprint 1", sprintsdomain.SprintStatusClosed, 0)
		activeID := f.seedSprint(t, projectID, "Sprint 2", sprintsdomain.SprintStatusActive, 1)
		f.seedSprint(t, projectID, "Sprint 3", sprintsdomain.SprintStatusPlanned, 2)

		got, err := repository.NewSprintDirectory(f.db).FindActiveSprint(context.Background(), projectID)

		require.NoError(t, err)
		require.NotNil(t, got, "debería devolver el sprint activo")
		assert.Equal(t, projectsdomain.ActiveSprint{ID: activeID, Name: "Sprint 2"}, *got)
	})

	// El sprint Activo de otro proyecto no se informa como activo del proyecto consultado.
	t.Run("el activo de otro proyecto no cuenta", func(t *testing.T) {
		f := newSummaryFixture(t)
		projectID := f.seedProject(t, "portal de clientes")
		otherID := f.seedProject(t, "app de inventario")
		f.seedSprint(t, projectID, "Sprint 1", sprintsdomain.SprintStatusPlanned, 0)
		f.seedSprint(t, otherID, "Sprint ajeno", sprintsdomain.SprintStatusActive, 0)

		got, err := repository.NewSprintDirectory(f.db).FindActiveSprint(context.Background(), projectID)

		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

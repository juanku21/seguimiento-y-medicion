package delivery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
)

// Pruebas de integración de la edición simultánea de una historia (T059): dos integrantes envían a
// la vez un PATCH /api/v1/stories/{storyId} contra la base smye_test y se compara la fila resultante
// (FR-025, SC-013, research.md sección 7). Cada escenario repite la pareja de solicitudes sobre una
// historia nueva para que el intercalado entre las dos transacciones ocurra de verdad.

// backlogConcurrentRounds es la cantidad de parejas de solicitudes simultáneas por escenario.
const backlogConcurrentRounds = 10

// Listas de criterios que dos integrantes envían a la vez. Sus textos no se repiten entre sí, de modo
// que cualquier mezcla de las dos listas se distingue de cada una completa.
var (
	backlogConcurrentCriteriaA = []string{
		"El PDF lleva el logo del proyecto.",
		"El PDF muestra la fecha de generación.",
		"El PDF se puede imprimir en A4.",
	}
	backlogConcurrentCriteriaB = []string{
		"El reporte se exporta también a CSV.",
		"El CSV usa punto y coma como separador.",
	}
)

// patchStoryTwiceAtOnce envía dos PATCH a la misma historia desde dos goroutines que esperan la
// misma señal de salida, para maximizar la simultaneidad, y devuelve las dos respuestas. Cada
// goroutine escribe en su propio recorder, así que no comparten estado mutable.
func (e backlogTestEnv) patchStoryTwiceAtOnce(t *testing.T, storyID string, firstToken, firstBody, secondToken, secondBody string) (first, second *httptest.ResponseRecorder) {
	t.Helper()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		first = e.patchStory(t, firstToken, storyID, firstBody)
	})
	wg.Go(func() {
		<-start
		second = e.patchStory(t, secondToken, storyID, secondBody)
	})
	close(start)
	wg.Wait()
	return first, second
}

// Escenario: Edición simultánea de campos distintos (003/US5)
// Ana cambia la prioridad y Bruno la descripción de la misma historia a la vez: en cada una de las
// repeticiones las dos solicitudes se aceptan y la historia queda con la prioridad nueva y la
// descripción nueva, sin que una pise a la otra, y con el título intacto (FR-025, SC-013).
func TestUS5_EdicionSimultaneaCamposDistintos(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	priorityBody := `{"priority":"low"}`
	descriptionBody := validationPatchJSON(t, map[string]any{"description": backlogRefinedDescription})

	// Cada repetición usa una historia recién sembrada con prioridad Alta y sin descripción.
	for round := range backlogConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(5), 2)

			priorityRec, descriptionRec := env.patchStoryTwiceAtOnce(t, storyID.String(),
				anaToken, priorityBody, brunoToken, descriptionBody)

			// Las dos modificaciones se aceptan.
			require.Equal(t, http.StatusOK, priorityRec.Code, "PATCH de prioridad; cuerpo: %s", priorityRec.Body.String())
			require.Equal(t, http.StatusOK, descriptionRec.Code, "PATCH de descripción; cuerpo: %s", descriptionRec.Body.String())

			// La fila conserva los dos cambios y no toca las columnas que ninguno envió.
			stored := backlogStoredStory(t, env.db, storyID.String())
			assert.Equal(t, domain.StoryPriorityLow, stored.Priority, "se perdió el cambio de prioridad")
			if assert.NotNil(t, stored.Description, "se perdió el cambio de descripción") {
				assert.Equal(t, backlogRefinedDescription, *stored.Description)
			}
			assert.Equal(t, backlogStoryTitle, stored.Title, "el título no debería cambiar")
			assert.Equal(t, domain.StoryStatusPending, stored.Status, "el estado no debería cambiar")
			assert.Equal(t, []string{backlogFirstCriterion, backlogSecondCriterion},
				backlogCriteriaTexts(t, env.db, storyID.String()), "los criterios no deberían cambiar")
		})
	}
}

// Escenario: Edición simultánea de criterios (003/US5)
// Ana y Bruno reescriben a la vez los criterios de la misma historia con dos listas distintas: en
// cada repetición las dos solicitudes se aceptan y la historia queda con una de las dos listas
// completa, en su orden y con posiciones 0..n-1, nunca con una mezcla ni con más criterios que la
// lista más larga (FR-025, research.md sección 7).
func TestUS5_EdicionSimultaneaDeCriterios(t *testing.T) {
	env := newBacklogTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	brunoID, brunoToken := env.signUp(t, "Bruno Gómez", "bruno@example.com")
	projectID := backlogSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress, brunoID)
	bodyA := validationPatchJSON(t, map[string]any{"acceptanceCriteria": backlogConcurrentCriteriaA})
	bodyB := validationPatchJSON(t, map[string]any{"acceptanceCriteria": backlogConcurrentCriteriaB})
	longest := max(len(backlogConcurrentCriteriaA), len(backlogConcurrentCriteriaB))

	// Cada repetición usa una historia recién sembrada con un criterio que ninguna lista contiene.
	for round := range backlogConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			storyID := backlogSeedEstimable(t, env.db, projectID, anaID, domain.StoryStatusPending, backlogPoints(3), 1)

			recA, recB := env.patchStoryTwiceAtOnce(t, storyID.String(), anaToken, bodyA, brunoToken, bodyB)

			// Las dos modificaciones se aceptan, sin aviso de conflicto para ninguna.
			require.Equal(t, http.StatusOK, recA.Code, "PATCH con la lista A; cuerpo: %s", recA.Body.String())
			require.Equal(t, http.StatusOK, recB.Code, "PATCH con la lista B; cuerpo: %s", recB.Body.String())

			// La historia queda con una de las dos listas completa y nunca con una fusión.
			stored := backlogCriteriaTexts(t, env.db, storyID.String())
			assert.LessOrEqual(t, len(stored), longest, "hay más criterios que en la lista más larga: %q", stored)
			assert.True(t,
				assert.ObjectsAreEqual(backlogConcurrentCriteriaA, stored) ||
					assert.ObjectsAreEqual(backlogConcurrentCriteriaB, stored),
				"los criterios guardados %q no son ni la lista A ni la lista B completas", stored)
		})
	}
}

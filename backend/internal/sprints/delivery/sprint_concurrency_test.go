package delivery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/projects/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración de la creación simultánea de sprints (T013; SC-004, research.md sección 4):
// varias goroutines envían a la vez POST /api/v1/projects/{projectId}/sprints contra la base
// smye_test con períodos que se superponen de a pares, y al terminar se verifica que ningún par de
// sprints guardados del proyecto se superpone. Cada repetición usa un proyecto nuevo para que el
// intercalado entre las transacciones ocurra de verdad. No lleva la etiqueta de un escenario de la
// spec porque verifica un criterio de éxito y no un escenario Given/When/Then.

// Cantidad de repeticiones y de solicitudes simultáneas por repetición. Las solicitudes forman
// pares: los dos períodos de un par se superponen entre sí y no con los de otro par.
const (
	sprintConcurrentRounds   = 5
	sprintConcurrentRequests = 10
)

// sprintConcurrentBodies arma los cuerpos de las solicitudes simultáneas. El par k ocupa una ventana
// propia de 20 días que empieza el 2026-10-01 más 20·k días: su primer sprint dura del día 0 al 10
// de la ventana y el segundo del día 5 al 15, así que se superponen entre sí y nunca con otro par.
func sprintConcurrentBodies() []map[string]any {
	base := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	bodies := make([]map[string]any, 0, sprintConcurrentRequests)
	for i := range sprintConcurrentRequests {
		window := base.AddDate(0, 0, 20*(i/2))
		start := window.AddDate(0, 0, 5*(i%2))
		end := start.AddDate(0, 0, 10)
		bodies = append(bodies, sprintBody(fmt.Sprintf("Sprint %d", i+1), sprintGoal,
			start.Format(time.DateOnly), end.Format(time.DateOnly)))
	}
	return bodies
}

// postSprintsAtOnce envía todas las creaciones desde goroutines que esperan la misma señal de
// salida, para maximizar la simultaneidad, y devuelve las respuestas en el orden de los cuerpos.
// Cada goroutine escribe solo en su posición del slice, así que no comparten estado mutable.
func (e sprintTestEnv) postSprintsAtOnce(t *testing.T, token string, projectID uuid.UUID, bodies []map[string]any) []*httptest.ResponseRecorder {
	t.Helper()
	recs := make([]*httptest.ResponseRecorder, len(bodies))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, body := range bodies {
		wg.Go(func() {
			<-start
			recs[i] = e.postSprint(t, token, projectID, body)
		})
	}
	close(start)
	wg.Wait()
	return recs
}

// Diez creaciones simultáneas en el mismo proyecto, con períodos que se superponen de a pares: en
// cada repetición exactamente una creación de cada par se acepta y la otra responde 409, y entre los
// sprints guardados ningún par cumple PeriodsOverlap (SC-004).
func TestSprintCreate_Concurrency(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")
	bodies := sprintConcurrentBodies()

	// Cada repetición usa un proyecto recién sembrado y sin sprints.
	for round := range sprintConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)

			recs := env.postSprintsAtOnce(t, anaToken, projectID, bodies)

			// Cada respuesta es una creación o un rechazo por superposición, y se acepta una por par.
			created := 0
			for i, rec := range recs {
				switch rec.Code {
				case http.StatusCreated:
					created++
				case http.StatusConflict:
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
			assert.Equal(t, sprintConcurrentRequests/2, created, "debería aceptarse exactamente una creación por par")

			// Los sprints guardados son los aceptados y ningún par de ellos se superpone.
			var stored []domain.Sprint
			require.NoError(t, env.db.Where("project_id = ?", projectID).Find(&stored).Error,
				"no se pudieron leer los sprints del proyecto")
			assert.Len(t, stored, created, "los sprints guardados deberían ser los aceptados")
			for i := range stored {
				for j := i + 1; j < len(stored); j++ {
					a := domain.Period{Start: stored[i].StartDate, End: stored[i].EndDate}
					b := domain.Period{Start: stored[j].StartDate, End: stored[j].EndDate}
					assert.False(t, domain.PeriodsOverlap(a, b), "se superponen %q y %q", stored[i].Name, stored[j].Name)
				}
			}
		})
	}
}

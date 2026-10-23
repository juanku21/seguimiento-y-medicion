package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
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

// sprintRequestsAtOnce ejecuta n solicitudes desde goroutines que esperan la misma señal de salida,
// para maximizar la simultaneidad, y devuelve las respuestas en el orden de los índices. Cada
// goroutine escribe solo en su posición del slice, así que no comparten estado mutable.
func sprintRequestsAtOnce(n int, send func(i int) *httptest.ResponseRecorder) []*httptest.ResponseRecorder {
	recs := make([]*httptest.ResponseRecorder, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			recs[i] = send(i)
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

			recs := sprintRequestsAtOnce(len(bodies), func(i int) *httptest.ResponseRecorder {
				return env.postSprint(t, anaToken, projectID, bodies[i])
			})

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

// Pruebas de integración del inicio simultáneo de sprints (T042; SC-005, research.md sección 5):
// varias goroutines envían a la vez POST /api/v1/sprints/{sprintId}/start sobre sprints Planificados
// distintos del mismo proyecto, y al terminar se verifica que el proyecto tiene un solo sprint
// activo. Igual que la creación simultánea, no lleva la etiqueta de un escenario de la spec.

// sprintSeedConcurrentPlanned siembra en el proyecto diez sprints Planificados con períodos
// consecutivos de 10 días separados por un día libre, así que ninguno se superpone con otro, y
// devuelve sus identificadores.
func sprintSeedConcurrentPlanned(t *testing.T, env sprintTestEnv, projectID, authorID uuid.UUID) []uuid.UUID {
	t.Helper()
	base := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]uuid.UUID, 0, sprintConcurrentRequests)
	for i := range sprintConcurrentRequests {
		start := base.AddDate(0, 0, 11*i)
		end := start.AddDate(0, 0, 10)
		ids = append(ids, sprintSeed(t, env.db, projectID, authorID, fmt.Sprintf("Sprint %d", i+1),
			start.Format(time.DateOnly), end.Format(time.DateOnly)))
	}
	return ids
}

// Diez inicios simultáneos de sprints Planificados distintos del mismo proyecto: en cada repetición
// exactamente uno responde 200 y los otros nueve 409 nombrando al sprint que quedó activo, y en la
// base el proyecto tiene un único sprint active, el aceptado (SC-005, RN-06).
func TestSprintStart_Concurrency(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")

	// Cada repetición usa un proyecto recién sembrado con sus diez sprints Planificados.
	for round := range sprintConcurrentRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			sprintIDs := sprintSeedConcurrentPlanned(t, env, projectID, anaID)

			recs := sprintRequestsAtOnce(len(sprintIDs), func(i int) *httptest.ResponseRecorder {
				return env.startSprint(t, anaToken, sprintIDs[i])
			})

			// Cada respuesta es un inicio o un rechazo por otro sprint activo, y se acepta uno solo.
			var startedIDs []string
			var conflicts []*httptest.ResponseRecorder
			for i, rec := range recs {
				switch rec.Code {
				case http.StatusOK:
					startedIDs = append(startedIDs, sprintIDs[i].String())
				case http.StatusConflict:
					conflicts = append(conflicts, rec)
				default:
					t.Errorf("solicitud %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
			require.Len(t, startedIDs, 1, "debería aceptarse exactamente un inicio")
			assert.Len(t, conflicts, sprintConcurrentRequests-1, "los demás inicios deberían responder 409")

			// En la base hay un único sprint activo y es el que se aceptó.
			var active []domain.Sprint
			require.NoError(t, env.db.Where("project_id = ? AND status = ?", projectID, domain.SprintStatusActive).
				Find(&active).Error, "no se pudieron leer los sprints activos del proyecto")
			require.Len(t, active, 1, "el proyecto debería tener un solo sprint activo")
			assert.Equal(t, startedIDs[0], active[0].ID.String(), "el sprint activo debería ser el aceptado")

			// Cada 409 nombra al sprint que quedó activo.
			for _, rec := range conflicts {
				var body sprintConflictBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "el cuerpo del 409 no es JSON válido: %s", rec.Body.String())
				if assert.NotNil(t, body.Sprint, "el 409 debe indicar el sprint activo: %s", rec.Body.String()) {
					assert.Equal(t, active[0].ID.String(), body.Sprint.ID)
					assert.Equal(t, active[0].Name, body.Sprint.Name)
				}
			}
		})
	}
}

// Pruebas de integración del cierre simultáneo con el avance de historias (T099; research.md
// secciones 13 y 14): una goroutine envía POST /api/v1/sprints/{sprintId}/close sobre el sprint
// Activo mientras otras envían a la vez PUT /api/v1/stories/{storyId}/status para completar sus
// historias En progreso. Ninguna petición puede responder 500 y el resultado debe ser coherente:
// cada avance o entra antes del cierre, y la instantánea lo congela como completado, o se rechaza con
// 409 y la historia vuelve al backlog como no completada. Igual que las demás pruebas de
// concurrencia, no lleva la etiqueta de un escenario de la spec.

// Cantidad de repeticiones y de historias En progreso del sprint Activo que se intentan completar
// durante el cierre. Son más repeticiones que en las demás pruebas porque el interbloqueo depende del
// intercalado y con pocas puede no aparecer.
const (
	sprintCloseProgressRounds       = 20
	sprintConcurrentProgressStories = 10
)

// Cierre de un sprint Activo simultáneo con el completado de sus historias: en cada repetición el
// cierre responde 200, cada avance responde 200 o 409 y nunca 500, y en la base cada historia aceptada
// queda completed con outcome completed y completed_at igual al reloj, mientras cada rechazada vuelve
// a pending con outcome not_completed y sin completed_at (FR-034, FR-041, FR-042).
func TestSprintClose_ConcurrentProgress(t *testing.T) {
	env := newSprintTestEnv(t)
	anaID, anaToken := env.signUp(t, "Ana Pérez", "ana@example.com")

	// Cada repetición usa un proyecto nuevo con un sprint Activo y sus historias En progreso.
	for round := range sprintCloseProgressRounds {
		t.Run(fmt.Sprintf("repeticion_%d", round+1), func(t *testing.T) {
			projectID := sprintSeedProject(t, env.db, anaID, projectsdomain.ProjectStatusInProgress)
			activeID := sprintSeedInStatus(t, env.db, projectID, anaID, "Sprint 1", "2026-10-15", "2026-10-28", domain.SprintStatusActive)
			storyIDs := make([]uuid.UUID, 0, sprintConcurrentProgressStories)
			for i := range sprintConcurrentProgressStories {
				storyID := env.createReadyStory(t, anaToken, projectID, fmt.Sprintf("Historia %d", i+1))
				sprintSeedCommitment(t, env.db, activeID, storyID, sprintDate(t, "2026-10-15"), nil, nil)
				sprintSetStoryStatus(t, env.db, storyID, backlogdomain.StoryStatusInProgress)
				storyIDs = append(storyIDs, storyID)
			}

			// La solicitud 0 cierra el sprint y las demás completan una historia cada una.
			recs := sprintRequestsAtOnce(len(storyIDs)+1, func(i int) *httptest.ResponseRecorder {
				if i == 0 {
					return env.closeSprint(t, anaToken, activeID)
				}
				return env.changeStoryStatus(t, anaToken, storyIDs[i-1], "completed")
			})

			// El cierre se acepta y deja el sprint closed; si no, el resto de la verificación no aplica.
			require.Equal(t, http.StatusOK, recs[0].Code, "el cierre debería aceptarse; cuerpo: %s", recs[0].Body.String())
			require.Equal(t, domain.SprintStatusClosed, sprintStored(t, env.db, activeID.String()).Status,
				"el sprint debería quedar cerrado")

			// Cada avance entró antes del cierre o se rechazó con 409, y la base refleja cuál de los dos.
			for i, storyID := range storyIDs {
				rec := recs[i+1]
				stored := sprintStoredStory(t, env.db, storyID)
				row := sprintStoredCommitment(t, env.db, activeID, storyID)
				switch rec.Code {
				case http.StatusOK:
					assert.Equal(t, backlogdomain.StoryStatusCompleted, stored.Status, "historia %d aceptada", i+1)
					if assert.NotNil(t, row.Outcome, "historia %d: el cierre debe congelar el resultado", i+1) {
						assert.Equal(t, domain.StoryOutcomeCompleted, *row.Outcome, "historia %d aceptada", i+1)
					}
					if assert.NotNil(t, row.CompletedAt, "historia %d: completed_at debe quedar registrado", i+1) {
						assert.True(t, sprintFixedNow.Equal(*row.CompletedAt), "historia %d: completed_at = %v", i+1, *row.CompletedAt)
					}
				case http.StatusConflict:
					assert.Equal(t, backlogdomain.StoryStatusPending, stored.Status, "historia %d rechazada", i+1)
					if assert.NotNil(t, row.Outcome, "historia %d: el cierre debe congelar el resultado", i+1) {
						assert.Equal(t, domain.StoryOutcomeNotCompleted, *row.Outcome, "historia %d rechazada", i+1)
					}
					assert.Nil(t, row.CompletedAt, "historia %d: completed_at debe seguir en null", i+1)
				default:
					t.Errorf("avance de la historia %d: código %d inesperado; cuerpo: %s", i+1, rec.Code, rec.Body.String())
				}
			}
		})
	}
}

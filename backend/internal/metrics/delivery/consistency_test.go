package delivery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas transversales de coherencia de las métricas (SC-016 y SC-019). Son de integración, sobre el
// router real y la base de test, y no llevan etiqueta 008/USn porque no implementan un escenario de
// una historia concreta.

// metricNumber convierte el valor de una métrica disponible en número; detiene la prueba si no lo está.
func metricNumber(t *testing.T, name string, m metricValue) float64 {
	t.Helper()
	require.Equal(t, "available", m.Status, "%s debe estar disponible", name)
	n, err := strconv.ParseFloat(string(m.Value), 64)
	require.NoError(t, err, "%s: value = %s no es un número", name, string(m.Value))
	return n
}

// rawNumber convierte una cifra suelta de la respuesta en número.
func rawNumber(t *testing.T, name string, raw json.RawMessage) float64 {
	t.Helper()
	n, err := strconv.ParseFloat(string(raw), 64)
	require.NoError(t, err, "%s: valor = %q no es un número", name, string(raw))
	return n
}

// Escenario: SC-016 ampliada — ninguna hora registrada queda invisible en las métricas del proyecto
func TestMetrics_NoInvisibleEffort(t *testing.T) {
	f := newMetricsFixture(t)

	// Horas en dos sprints Cerrados y en el Activo.
	f.closedSprintN(t, 0, []int{5, 5, 3}, []int{8}, 142.5)
	f.closedSprintN(t, 1, []int{8, 8, 5}, []int{3}, 130)
	f.startedSprintN(t, 2, []int{5}, []int{8, 5}, 40)
	// Horas en un sprint Planificado y sobre una historia sin sprint.
	f.plannedSprintWith(t, 9)
	backlogStory := f.env.createStory(t, f.ownerToken, f.projectID, "Historia del backlog", 5)
	f.env.logHours(t, f.ownerToken, backlogStory, 18)

	// Total de horas del proyecto leído directamente de la base, en centésimas.
	var totalHundredths int64
	require.NoError(t, f.env.db.Raw(`SELECT COALESCE(SUM(e.hours_hundredths), 0)
		FROM effort_entries e JOIN user_stories s ON s.id = e.story_id
		WHERE s.project_id = ?`, f.projectID).Scan(&totalHundredths).Error)
	require.Equal(t, int64(33950), totalHundredths, "la preparación debe haber cargado 339,50 horas")

	body := f.cumulative(t)

	actual := metricNumber(t, "cumulative.actualHours", body.Cumulative.ActualHours)
	outside := rawNumber(t, "effortOutsideSprintsHours", body.EffortOutsideSprintsHours)
	inPlanned := rawNumber(t, "effortInPlannedSprintsHours", body.EffortInPlannedSprintsHours)
	assert.InDelta(t, float64(totalHundredths)/100, actual+outside+inPlanned, 0.001,
		"reales + fuera de sprint + en Planificados debe ser el total registrado")
	assert.Greater(t, outside, 0.0, "hay horas sin sprint")
	assert.Greater(t, inPlanned, 0.0, "hay horas en un Planificado")
}

// Escenario: SC-019 y RN-29 — 20 consultas con escrituras concurrentes nunca devuelven una respuesta contradictoria
func TestMetrics_Consistency(t *testing.T) {
	f := newMetricsFixture(t)
	f.closedSprintN(t, 0, []int{5, 5, 3}, []int{8}, 20)

	// Sprint Activo con historias pendientes que los escritores completan mientras se consulta.
	points := []int{3, 5, 5, 8, 3, 5}
	stories := make([]uuid.UUID, 0, len(points))
	for i, p := range points {
		stories = append(stories, f.env.createStory(t, f.ownerToken, f.projectID, "Concurrente "+strconv.Itoa(i), p))
	}
	activeID := f.env.createSprint(t, f.ownerToken, f.projectID, "Sprint activo", "2026-10-26", "2026-11-06", stories...)
	f.env.startSprint(t, f.ownerToken, activeID)

	// Días de calendario libres para los escritores, que no se repiten entre goroutines.
	firstDay := f.env.effortDay
	var nextDay atomic.Int64
	start, err := time.Parse(time.DateOnly, metricsProjectStart)
	require.NoError(t, err)

	var serverErrors atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var stopOnce sync.Once
	// Si una verificación detiene la prueba antes de tiempo, igual se frena y se espera a los escritores.
	t.Cleanup(func() {
		stopOnce.Do(func() { close(stop) })
		wg.Wait()
	})

	// Escritores de horas con la 006, uno por historia pendiente.
	for _, storyID := range stories[:3] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				day := int(nextDay.Add(1)) - 1 + firstDay
				if day > 19 {
					return
				}
				rec := f.env.do(t, http.MethodPost, "/api/v1/stories/"+storyID.String()+"/effort-entries", f.ownerToken, map[string]any{
					"entryDate": start.AddDate(0, 0, day).Format(time.DateOnly),
					"activity":  "Horas concurrentes",
					"hours":     2.5,
				})
				if rec.Code >= http.StatusInternalServerError {
					serverErrors.Add(1)
				}
			}
		}()
	}
	// Escritores que completan historias con la 004.
	for _, storyID := range stories[3:] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, status := range []string{"in_progress", "completed"} {
				rec := f.env.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", f.ownerToken,
					map[string]any{"status": status})
				if rec.Code >= http.StatusInternalServerError {
					serverErrors.Add(1)
				}
			}
		}()
	}

	// 20 rondas de lectura del proyecto y del sprint Activo.
	for i := 0; i < 20; i++ {
		var project cumulativeBody
		rec := f.env.getProjectMetrics(t, f.ownerToken, f.projectID.String())
		metricsRequireStatus(t, rec, http.StatusOK, &project)
		c := project.Cumulative
		assert.LessOrEqual(t, metricNumber(t, "cumulative.completedStoryPoints", c.CompletedStoryPoints),
			metricNumber(t, "cumulative.plannedStoryPoints", c.PlannedStoryPoints), "consulta %d: completados > planificados en el proyecto", i)
		assert.InDelta(t, metricNumber(t, "cumulative.actualHours", c.ActualHours)-metricNumber(t, "cumulative.estimatedHours", c.EstimatedHours),
			metricNumber(t, "cumulative.deviationHours", c.DeviationHours), 0.001, "consulta %d: desvío del proyecto", i)

		var sprint sprintMetricsBody
		rec = f.env.getSprintMetrics(t, f.ownerToken, f.projectID.String(), activeID.String())
		metricsRequireStatus(t, rec, http.StatusOK, &sprint)
		assert.LessOrEqual(t, metricNumber(t, "completedStoryPoints", sprint.CompletedStoryPoints),
			metricNumber(t, "plannedStoryPoints", sprint.PlannedStoryPoints), "consulta %d: completados > planificados en el sprint", i)
		assert.InDelta(t, metricNumber(t, "actualHours", sprint.ActualHours)-metricNumber(t, "estimatedHours", sprint.EstimatedHours),
			metricNumber(t, "deviationHours", sprint.DeviationHours), 0.001, "consulta %d: desvío del sprint", i)
	}
	stopOnce.Do(func() { close(stop) })
	wg.Wait()

	assert.Zero(t, serverErrors.Load(), "ninguna escritura concurrente debe devolver 500")
}

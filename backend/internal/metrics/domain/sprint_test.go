package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

var (
	// Instante fijo del cálculo y plazo de gracia de los datos de prueba.
	fixedNow   = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	testGrace  = 48 * time.Hour
	factorSix  = 600
	closedLong = fixedNow.Add(-100 * time.Hour) // gracia vencida
	closedNear = fixedNow.Add(-10 * time.Hour)  // gracia vigente
)

// intPtr devuelve un puntero al entero dado.
func intPtr(v int) *int { return &v }

// timePtr devuelve un puntero al instante dado.
func timePtr(v time.Time) *time.Time { return &v }

// stories arma historias con los puntos dados; las primeras "completed" quedan Completed.
func stories(points []int, completed int) []domain.SprintStoryData {
	out := make([]domain.SprintStoryData, len(points))
	for i, p := range points {
		out[i] = domain.SprintStoryData{
			StoryID:     uuid.New(),
			Title:       "historia",
			StoryPoints: intPtr(p),
			Completed:   i < completed,
		}
	}
	return out
}

// closedSprint arma un sprint cerrado con factor congelado de 6 y cierre dado.
func closedSprint(st []domain.SprintStoryData, actual int, closedAt time.Time) domain.SprintData {
	return domain.SprintData{
		ID:                     uuid.New(),
		Name:                   "Sprint 1",
		Status:                 sprintsdomain.SprintStatusClosed,
		StartDate:              time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndDate:                time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
		ClosedAt:               timePtr(closedAt),
		FrozenFactorHundredths: intPtr(factorSix),
		Stories:                st,
		ActualHundredths:       actual,
	}
}

// finalityOf devuelve un puntero a la finality dada.
func finalityOf(f domain.Finality) *domain.Finality { return &f }

// assertMetric verifica estado, valor y finality de una métrica.
func assertMetric(t *testing.T, name string, got domain.Metric, want domain.Metric, fin domain.Finality) {
	t.Helper()
	assert.Equal(t, want.Status, got.Status, name+": status")
	assert.Equal(t, want.Value, got.Value, name+": value")
	assert.Equal(t, finalityOf(fin), got.Finality, name+": finality")
}

// Escenario: métricas exactas de un sprint cerrado (008/US1-1)
func TestUS1_SprintCerradoNumerosExactos(t *testing.T) {
	// 4 historias, 21 puntos, 2 completadas por 13 puntos, 142,5 h reales, factor 6.
	st := stories([]int{8, 5, 5, 3}, 2)
	s := closedSprint(st, 14250, closedNear)

	m, warnings := domain.CalculateSprint(s, 800, fixedNow, testGrace)

	assert.Equal(t, s.ID, m.ID)
	assert.Equal(t, sprintsdomain.SprintStatusClosed, m.Status)
	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(21), domain.FinalityDefinitive)
	assertMetric(t, "completed", m.CompletedStoryPoints, domain.Available(13), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(12600), domain.FinalityDefinitive)
	assertMetric(t, "actual", m.ActualHours, domain.Available(14250), domain.FinalityProvisional)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(1650), domain.FinalityProvisional)
	assertMetric(t, "deviationPct", m.DeviationPercent, domain.Available(1310), domain.FinalityProvisional)
	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(5000), domain.FinalityDefinitive)
	assertMetric(t, "defectsDetected", m.DefectsDetected, domain.Available(0), domain.FinalityProvisional)
	assertMetric(t, "defectsResolved", m.DefectsResolved, domain.Available(0), domain.FinalityProvisional)
	assert.Empty(t, warnings)
}

// Escenario: desviación negativa por sobrestimación (008/US1-2)
func TestUS1_SprintCerradoDesviacionNegativa(t *testing.T) {
	s := closedSprint(stories([]int{8, 5, 5, 3}, 2), 11000, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "deviation", m.DeviationHours, domain.Available(-1600), domain.FinalityDefinitive)
	assertMetric(t, "deviationPct", m.DeviationPercent, domain.Available(-1270), domain.FinalityDefinitive)
}

// Escenario: el factor congelado prevalece sobre el vigente (008/US1-3)
func TestUS1_SprintCerradoUsaFactorCongelado(t *testing.T) {
	s := closedSprint(stories([]int{8, 5, 5, 3}, 2), 14250, closedNear)

	m, warnings := domain.CalculateSprint(s, 800, fixedNow, testGrace)

	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(12600), domain.FinalityDefinitive)
	assert.Empty(t, warnings)
}

// Escenario: los puntos congelados prevalecen sobre el factor vigente distinto (008/US1-4)
func TestUS1_SprintCerradoUsaPuntosCongelados(t *testing.T) {
	s := closedSprint(stories([]int{5, 8, 5, 3}, 1), 0, closedLong)

	m, _ := domain.CalculateSprint(s, 1300, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(21), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(12600), domain.FinalityDefinitive)
}

// Escenario: factores con dos decimales (008/US1, RN-07)
func TestUS1_SprintCerradoFactorConDosDecimales(t *testing.T) {
	s := closedSprint(stories([]int{8, 5, 5, 3}, 0), 0, closedLong)
	s.FrozenFactorHundredths = intPtr(625)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(13125), domain.FinalityDefinitive)
}

// Escenario: sprint con estimaciones en 0 puntos (008/US1-8)
func TestUS1_SprintCerradoTodoEnCeroPuntos(t *testing.T) {
	s := closedSprint(stories([]int{0, 0, 0}, 0), 1200, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(0), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(0), domain.FinalityDefinitive)
	assertMetric(t, "actual", m.ActualHours, domain.Available(1200), domain.FinalityDefinitive)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(1200), domain.FinalityDefinitive)
	assert.Equal(t, domain.ValueNotCalculable, m.DeviationPercent.Status)
	assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.DeviationPercent.Finality)
}

// Escenario: sprint sin historias planificadas (008/US1-9)
func TestUS1_SprintCerradoSinHistorias(t *testing.T) {
	s := closedSprint(nil, 700, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(0), domain.FinalityDefinitive)
	assertMetric(t, "completed", m.CompletedStoryPoints, domain.Available(0), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(0), domain.FinalityDefinitive)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(700), domain.FinalityDefinitive)
	assert.Equal(t, domain.ValueNotCalculable, m.DeviationPercent.Status)
	assert.Equal(t, domain.ValueNotCalculable, m.StoryCompletionPercent.Status)
	assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.StoryCompletionPercent.Finality)
}

// Escenario: todas las historias completadas (008/US1-10)
func TestUS1_SprintCerradoTodasCompletadas(t *testing.T) {
	s := closedSprint(stories([]int{3, 3, 3, 3}, 4), 0, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(10000), domain.FinalityDefinitive)
}

// Escenario: redondeo solo en el resultado final (008/US1-11)
func TestUS1_SprintCerradoUnaDeTresCompletada(t *testing.T) {
	s := closedSprint(stories([]int{3, 3, 3}, 1), 0, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(3333), domain.FinalityDefinitive)
}

// Escenario: el porcentaje se calcula por cantidad de historias y no por puntos (008/US1, RN-09)
func TestUS1_PorcentajePorCantidadNoPorPuntos(t *testing.T) {
	// Una historia de 13 puntos completada y tres de 1 punto sin completar: 25 % por cantidad.
	s := closedSprint(stories([]int{13, 1, 1, 1}, 1), 0, closedLong)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(2500), domain.FinalityDefinitive)
	assertMetric(t, "completed", m.CompletedStoryPoints, domain.Available(13), domain.FinalityDefinitive)
}

// Escenario: finality de las nueve métricas de un sprint cerrado (008/US1-5, US1-6, RN-23, RN-24)
func TestUS1_SprintFinalityCerrado(t *testing.T) {
	def := domain.FinalityDefinitive
	prov := domain.FinalityProvisional
	tests := []struct {
		kind      domain.MetricKind
		graceOpen bool
		want      domain.Finality
	}{
		{domain.MetricPlannedStoryPoints, true, def},
		{domain.MetricPlannedStoryPoints, false, def},
		{domain.MetricCompletedStoryPoints, true, def},
		{domain.MetricCompletedStoryPoints, false, def},
		{domain.MetricEstimatedHours, true, def},
		{domain.MetricEstimatedHours, false, def},
		{domain.MetricStoryCompletionPercent, true, def},
		{domain.MetricStoryCompletionPercent, false, def},
		{domain.MetricActualHours, true, prov},
		{domain.MetricActualHours, false, def},
		{domain.MetricDeviationHours, true, prov},
		{domain.MetricDeviationHours, false, def},
		{domain.MetricDeviationPercent, true, prov},
		{domain.MetricDeviationPercent, false, def},
		{domain.MetricDefectsDetected, true, prov},
		{domain.MetricDefectsDetected, false, prov},
		{domain.MetricDefectsResolved, true, prov},
		{domain.MetricDefectsResolved, false, prov},
	}
	for _, tt := range tests {
		grace := "vencida"
		if tt.graceOpen {
			grace = "vigente"
		}
		t.Run(string(tt.kind)+"/gracia_"+grace, func(t *testing.T) {
			got := domain.SprintFinality(sprintsdomain.SprintStatusClosed, tt.kind, tt.graceOpen)

			assert.Equal(t, finalityOf(tt.want), got)
			if got != nil {
				assert.NotEqual(t, domain.FinalityPartial, *got)
			}
		})
	}
}

// Escenario: finality de las métricas calculadas según la gracia (008/US1-5, US1-6, RN-23)
func TestUS1_SprintCerradoFinalityPorGracia(t *testing.T) {
	st := stories([]int{8, 5, 5, 3}, 2)

	t.Run("gracia vigente", func(t *testing.T) {
		m, _ := domain.CalculateSprint(closedSprint(st, 14250, closedNear), factorSix, fixedNow, testGrace)

		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.ActualHours.Finality)
		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.DeviationHours.Finality)
		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.DeviationPercent.Finality)
		assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.PlannedStoryPoints.Finality)
	})

	t.Run("gracia vencida", func(t *testing.T) {
		m, _ := domain.CalculateSprint(closedSprint(st, 14250, closedLong), factorSix, fixedNow, testGrace)

		assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.ActualHours.Finality)
		assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.DeviationHours.Finality)
		assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.DeviationPercent.Finality)
		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.DefectsDetected.Finality)
		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.DefectsResolved.Finality)
	})
}

// Escenario: borde exacto del plazo de gracia (008/US1, RN-23)
func TestUS1_SprintCerradoBordeDeGracia(t *testing.T) {
	st := stories([]int{8, 5, 5, 3}, 2)

	t.Run("now igual a closedAt mas gracia es vencido", func(t *testing.T) {
		s := closedSprint(st, 14250, fixedNow.Add(-testGrace))

		m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

		assert.Equal(t, finalityOf(domain.FinalityDefinitive), m.ActualHours.Finality)
	})

	t.Run("un instante antes del borde sigue vigente", func(t *testing.T) {
		s := closedSprint(st, 14250, fixedNow.Add(-testGrace+time.Nanosecond))

		m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

		assert.Equal(t, finalityOf(domain.FinalityProvisional), m.ActualHours.Finality)
	})
}

// Escenario: historia sin puntos en el sprint (008/US1, RN-30)
func TestUS1_HistoriaSinPuntosCuentaComoCero(t *testing.T) {
	st := stories([]int{8, 5, 5, 3}, 2)
	st[3].StoryPoints = nil
	s := closedSprint(st, 14250, closedNear)

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(18), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(10800), domain.FinalityDefinitive)
	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(5000), domain.FinalityDefinitive)
	assertMetric(t, "defectsResolved", m.DefectsResolved, domain.Available(0), domain.FinalityProvisional)
	if assert.Len(t, warnings, 1) {
		w := warnings[0]
		assert.Equal(t, domain.WarningDataInconsistency, w.Kind)
		assert.Equal(t, domain.CodeUnestimatedStoryInSprint, w.Code)
		if assert.NotNil(t, w.SprintID) {
			assert.Equal(t, s.ID, *w.SprintID)
		}
		if assert.NotNil(t, w.StoryID) {
			assert.Equal(t, st[3].StoryID, *w.StoryID)
		}
	}
}

// Escenario: sprint cerrado sin factor congelado (008/US1, RN-30)
func TestUS1_SinFactorCongeladoUsaVigente(t *testing.T) {
	s := closedSprint(stories([]int{8, 5, 5, 3}, 2), 14250, closedNear)
	s.FrozenFactorHundredths = nil

	m, warnings := domain.CalculateSprint(s, 800, fixedNow, testGrace)

	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(16800), domain.FinalityDefinitive)
	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(21), domain.FinalityDefinitive)
	assertMetric(t, "actual", m.ActualHours, domain.Available(14250), domain.FinalityProvisional)
	assertMetric(t, "defectsDetected", m.DefectsDetected, domain.Available(0), domain.FinalityProvisional)
	if assert.Len(t, warnings, 1) {
		assert.Equal(t, domain.CodeMissingFrozenFactor, warnings[0].Code)
		assert.Equal(t, domain.WarningDataInconsistency, warnings[0].Kind)
		if assert.NotNil(t, warnings[0].SprintID) {
			assert.Equal(t, s.ID, *warnings[0].SprintID)
		}
		assert.Nil(t, warnings[0].StoryID)
	}
}

// Escenario: sprint cerrado sin fecha de cierre (008/US1, RN-30)
func TestUS1_SinFechaDeCierreGraciaVencida(t *testing.T) {
	s := closedSprint(stories([]int{8, 5, 5, 3}, 2), 14250, closedNear)
	s.ClosedAt = nil

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(21), domain.FinalityDefinitive)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(12600), domain.FinalityDefinitive)
	assertMetric(t, "actual", m.ActualHours, domain.Available(14250), domain.FinalityDefinitive)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(1650), domain.FinalityDefinitive)
	assertMetric(t, "deviationPct", m.DeviationPercent, domain.Available(1310), domain.FinalityDefinitive)
	assertMetric(t, "defectsDetected", m.DefectsDetected, domain.Available(0), domain.FinalityProvisional)
	if assert.Len(t, warnings, 1) {
		assert.Equal(t, domain.CodeMissingCloseTime, warnings[0].Code)
		assert.Equal(t, domain.WarningDataInconsistency, warnings[0].Kind)
		if assert.NotNil(t, warnings[0].SprintID) {
			assert.Equal(t, s.ID, *warnings[0].SprintID)
		}
	}
}

// activeSprint arma un sprint activo sin cierre; el factor congelado, si se pasa, no debe usarse.
func activeSprint(st []domain.SprintStoryData, actual int) domain.SprintData {
	return domain.SprintData{
		ID:               uuid.New(),
		Name:             "Sprint 2",
		Status:           sprintsdomain.SprintStatusActive,
		StartDate:        time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		EndDate:          time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC),
		Stories:          st,
		ActualHundredths: actual,
	}
}

// Escenario: sprint activo con métricas parciales y números exactos (008/US4-1)
func TestUS4_SprintActivoNumerosExactos(t *testing.T) {
	// 18 puntos planificados, 5 completados, 40 h reales y factor vigente 6.
	s := activeSprint(stories([]int{5, 8, 5}, 1), 4000)
	s.DefectsDetected = 3
	s.DefectsResolved = 2

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assert.Equal(t, s.ID, m.ID)
	assert.Equal(t, sprintsdomain.SprintStatusActive, m.Status)
	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(18), domain.FinalityPartial)
	assertMetric(t, "completed", m.CompletedStoryPoints, domain.Available(5), domain.FinalityPartial)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(10800), domain.FinalityPartial)
	assertMetric(t, "actual", m.ActualHours, domain.Available(4000), domain.FinalityPartial)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(-6800), domain.FinalityPartial)
	assertMetric(t, "deviationPct", m.DeviationPercent, domain.Available(-6296), domain.FinalityPartial)
	assertMetric(t, "completion", m.StoryCompletionPercent, domain.Available(3333), domain.FinalityPartial)
	assertMetric(t, "defectsDetected", m.DefectsDetected, domain.Available(3), domain.FinalityPartial)
	assertMetric(t, "defectsResolved", m.DefectsResolved, domain.Available(2), domain.FinalityPartial)
	assert.Empty(t, warnings)
}

// Escenario: el sprint activo usa el factor vigente y no el congelado (008/US4-3)
func TestUS4_SprintActivoUsaFactorVigente(t *testing.T) {
	s := activeSprint(stories([]int{5, 8, 5}, 1), 4000)
	s.FrozenFactorHundredths = intPtr(factorSix)

	m, warnings := domain.CalculateSprint(s, 800, fixedNow, testGrace)

	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(14400), domain.FinalityPartial)
	assert.Empty(t, warnings)
}

// Escenario: el sprint activo se calcula con los datos actuales (008/US4-2)
func TestUS4_SprintActivoUsaPuntosActuales(t *testing.T) {
	s := activeSprint(stories([]int{5, 8, 5, 8}, 1), 4000)

	m, _ := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(26), domain.FinalityPartial)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(15600), domain.FinalityPartial)
}

// Escenario: sprint activo sin historias (008/US4-5)
func TestUS4_SprintActivoSinHistorias(t *testing.T) {
	m, _ := domain.CalculateSprint(activeSprint(nil, 0), factorSix, fixedNow, testGrace)

	assertMetric(t, "planned", m.PlannedStoryPoints, domain.Available(0), domain.FinalityPartial)
	assertMetric(t, "estimated", m.EstimatedHours, domain.Available(0), domain.FinalityPartial)
	assert.Equal(t, domain.ValueNotCalculable, m.DeviationPercent.Status)
	assert.Equal(t, finalityOf(domain.FinalityPartial), m.DeviationPercent.Finality)
	assert.Equal(t, domain.ValueNotCalculable, m.StoryCompletionPercent.Status)
	assert.Equal(t, finalityOf(domain.FinalityPartial), m.StoryCompletionPercent.Finality)
}

// Escenario: sprint activo sin horas registradas (008/US4-6)
func TestUS4_SprintActivoSinHoras(t *testing.T) {
	m, _ := domain.CalculateSprint(activeSprint(stories([]int{5, 8, 5}, 0), 0), factorSix, fixedNow, testGrace)

	assertMetric(t, "actual", m.ActualHours, domain.Available(0), domain.FinalityPartial)
	assertMetric(t, "deviation", m.DeviationHours, domain.Available(-10800), domain.FinalityPartial)
}

// Escenario: las nueve métricas de un sprint activo son parciales y ninguna provisoria (008/US4-1, RN-04, RN-24)
func TestUS4_SprintFinalityActivo(t *testing.T) {
	kinds := []domain.MetricKind{
		domain.MetricPlannedStoryPoints, domain.MetricCompletedStoryPoints, domain.MetricEstimatedHours,
		domain.MetricActualHours, domain.MetricDeviationHours, domain.MetricDeviationPercent,
		domain.MetricStoryCompletionPercent, domain.MetricDefectsDetected, domain.MetricDefectsResolved,
	}
	for _, kind := range kinds {
		for _, graceOpen := range []bool{true, false} {
			name := string(kind) + "/gracia_vigente"
			if !graceOpen {
				name = string(kind) + "/gracia_vencida"
			}
			t.Run(name, func(t *testing.T) {
				got := domain.SprintFinality(sprintsdomain.SprintStatusActive, kind, graceOpen)

				assert.Equal(t, finalityOf(domain.FinalityPartial), got)
			})
		}
	}
}

// plannedSprintUS4 arma un sprint planificado; no tiene cierre ni factor congelado.
func plannedSprintUS4(st []domain.SprintStoryData) domain.SprintData {
	return domain.SprintData{
		ID:        uuid.New(),
		Name:      "Sprint 3",
		Status:    sprintsdomain.SprintStatusPlanned,
		StartDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
		Stories:   st,
	}
}

// Escenario: sprint planificado informa solo dos métricas, sin estado de valor (008/US4-4)
func TestUS4_SprintPlanificadoSoloDosMetricas(t *testing.T) {
	// 15 puntos planificados y factor vigente 6; los datos de esfuerzo y defectos se ignoran.
	s := plannedSprintUS4(stories([]int{5, 5, 5}, 0))
	s.ActualHundredths = 1000
	s.DefectsDetected = 2
	s.DefectsResolved = 1

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assert.Equal(t, s.ID, m.ID)
	assert.Equal(t, sprintsdomain.SprintStatusPlanned, m.Status)
	assert.Equal(t, domain.Available(15), m.PlannedStoryPoints)
	assert.Nil(t, m.PlannedStoryPoints.Finality)
	assert.Equal(t, domain.Available(9000), m.EstimatedHours)
	assert.Nil(t, m.EstimatedHours.Finality)
	assert.Empty(t, warnings)

	others := map[string]domain.Metric{
		"completed":       m.CompletedStoryPoints,
		"actual":          m.ActualHours,
		"deviation":       m.DeviationHours,
		"deviationPct":    m.DeviationPercent,
		"completion":      m.StoryCompletionPercent,
		"defectsDetected": m.DefectsDetected,
		"defectsResolved": m.DefectsResolved,
	}
	for name, metric := range others {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, domain.NotApplicable(), metric)
			assert.Equal(t, domain.ValueNotApplicable, metric.Status)
			assert.Nil(t, metric.Finality)
			assert.NotEqual(t, domain.Available(0), metric)
		})
	}
}

// Escenario: el sprint planificado usa el factor vigente (008/US4-4)
func TestUS4_SprintPlanificadoUsaFactorVigente(t *testing.T) {
	s := plannedSprintUS4(stories([]int{5, 5, 5}, 0))

	m, _ := domain.CalculateSprint(s, 800, fixedNow, testGrace)

	assert.Equal(t, domain.Available(12000), m.EstimatedHours)
	assert.Nil(t, m.EstimatedHours.Finality)
}

// Escenario: sprint planificado sin historias (008/US4-4, FR-015)
func TestUS4_SprintPlanificadoSinHistorias(t *testing.T) {
	m, warnings := domain.CalculateSprint(plannedSprintUS4(nil), factorSix, fixedNow, testGrace)

	assert.Equal(t, domain.Available(0), m.PlannedStoryPoints)
	assert.Equal(t, domain.Available(0), m.EstimatedHours)
	assert.Nil(t, m.PlannedStoryPoints.Finality)
	assert.Equal(t, domain.NotApplicable(), m.StoryCompletionPercent)
	assert.Empty(t, warnings)
}

// Escenario: un compromiso sin puntos en un sprint planificado cuenta como 0 y advierte (008/US4-4, RN-05)
func TestUS4_SprintPlanificadoHistoriaSinPuntos(t *testing.T) {
	st := stories([]int{5, 5}, 0)
	unestimated := domain.SprintStoryData{StoryID: uuid.New(), Title: "sin estimar"}
	s := plannedSprintUS4(append(st, unestimated))

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assert.Equal(t, domain.Available(10), m.PlannedStoryPoints)
	assert.Equal(t, domain.Available(6000), m.EstimatedHours)
	if assert.Len(t, warnings, 1) {
		assert.Equal(t, domain.CodeUnestimatedStoryInSprint, warnings[0].Code)
		assert.Equal(t, domain.WarningDataInconsistency, warnings[0].Kind)
		if assert.NotNil(t, warnings[0].SprintID) {
			assert.Equal(t, s.ID, *warnings[0].SprintID)
		}
		if assert.NotNil(t, warnings[0].StoryID) {
			assert.Equal(t, unestimated.StoryID, *warnings[0].StoryID)
		}
	}
}

// Escenario: el sprint planificado no tiene estado de valor en ninguna métrica (008/US4-4, FR-015)
func TestUS4_SprintFinalityPlanificado(t *testing.T) {
	kinds := []domain.MetricKind{
		domain.MetricPlannedStoryPoints, domain.MetricCompletedStoryPoints, domain.MetricEstimatedHours,
		domain.MetricActualHours, domain.MetricDeviationHours, domain.MetricDeviationPercent,
		domain.MetricStoryCompletionPercent, domain.MetricDefectsDetected, domain.MetricDefectsResolved,
	}
	for _, kind := range kinds {
		for _, graceOpen := range []bool{true, false} {
			t.Run(string(kind)+"/"+map[bool]string{true: "gracia_vigente", false: "gracia_vencida"}[graceOpen], func(t *testing.T) {
				assert.Nil(t, domain.SprintFinality(sprintsdomain.SprintStatusPlanned, kind, graceOpen))
			})
		}
	}
}

// Guarda defensiva (sin escenario en spec.md): un estado de sprint desconocido devuelve solo la
// identificación, sin métricas calculadas ni advertencias.
func TestCalculateSprint_EstadoDesconocidoSoloIdentificacion(t *testing.T) {
	closedAt := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	s := domain.SprintData{
		ID:        uuid.New(),
		Name:      "Sprint X",
		Status:    sprintsdomain.SprintStatus("unknown"),
		StartDate: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
		ClosedAt:  &closedAt,
		Stories:   stories([]int{5, 5}, 1),
	}

	m, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

	assert.Equal(t, domain.SprintMetrics{
		ID:        s.ID,
		Name:      s.Name,
		Status:    s.Status,
		StartDate: s.StartDate,
		EndDate:   s.EndDate,
		ClosedAt:  s.ClosedAt,
	}, m)
	assert.Nil(t, warnings)
}

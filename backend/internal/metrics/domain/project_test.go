package domain_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// day devuelve la medianoche UTC del día dado de octubre de 2026.
func day(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

// closedWith arma un sprint Cerrado que completó los puntos dados, con inicio y cierre indicados.
// Un cierre nulo representa un Cerrado sin ClosedAt.
func closedWith(name string, completedPoints int, start int, closedAt *time.Time) domain.SprintData {
	// Con 0 puntos completados la única historia queda sin completar y vale 5 (valor válido de la escala).
	completedCount := 1
	storyPoints := completedPoints
	if completedPoints == 0 {
		completedCount = 0
		storyPoints = 5
	}
	return domain.SprintData{
		ID:                     uuid.New(),
		Name:                   name,
		Status:                 sprintsdomain.SprintStatusClosed,
		StartDate:              day(start),
		EndDate:                day(start + 1),
		ClosedAt:               closedAt,
		FrozenFactorHundredths: intPtr(factorSix),
		Stories:                stories([]int{storyPoints}, completedCount),
	}
}

// projectOf arma los datos de un proyecto con el factor vigente de prueba.
func projectOf(sprints ...domain.SprintData) domain.ProjectData {
	return domain.ProjectData{ProjectID: uuid.New(), FactorHundredths: 800, Sprints: sprints}
}

// seriesValues extrae los puntos completados de la serie de velocidad.
func seriesValues(v domain.Velocity) []int {
	out := make([]int, len(v.Series))
	for i, p := range v.Series {
		out[i] = p.CompletedStoryPoints
	}
	return out
}

// Escenario: promedio de los 3 cerrados (008/US2)
func TestUS2_VelocidadTresCerrados(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(3))),
		closedWith("S2", 21, 4, timePtr(day(6))),
		closedWith("S3", 8, 7, timePtr(day(9))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	assert.Equal(t, domain.Available(1400), v.Value)
	assert.Equal(t, 3, v.SprintsConsidered)
}

// Escenario: la ventana usa solo los 3 últimos cerrados, con 4 cerrados (008/US2)
func TestUS2_VelocidadCuatroCerrados(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(3))),
		closedWith("S2", 21, 4, timePtr(day(6))),
		closedWith("S3", 8, 7, timePtr(day(9))),
		closedWith("S4", 20, 10, timePtr(day(12))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	assert.Equal(t, domain.Available(1633), v.Value)
	assert.Equal(t, 3, v.SprintsConsidered)
}

// Escenario: velocidad con 1 y 2 cerrados promedia todos los cerrados (008/US2)
func TestUS2_VelocidadUnoYDosCerrados(t *testing.T) {
	t.Run("un cerrado", func(t *testing.T) {
		data := projectOf(closedWith("S1", 13, 1, timePtr(day(3))))

		v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

		assert.Equal(t, domain.Available(1300), v.Value)
		assert.Equal(t, 1, v.SprintsConsidered)
	})
	t.Run("dos cerrados", func(t *testing.T) {
		data := projectOf(
			closedWith("S1", 13, 1, timePtr(day(3))),
			closedWith("S2", 21, 4, timePtr(day(6))),
		)

		v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

		assert.Equal(t, domain.Available(1700), v.Value)
		assert.Equal(t, 2, v.SprintsConsidered)
	})
}

// Escenario: la serie de velocidad trae todos los cerrados en orden cronológico (008/US2)
func TestUS2_VelocidadSerieSinRecortar(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(3))),
		closedWith("S2", 21, 4, timePtr(day(6))),
		closedWith("S3", 8, 7, timePtr(day(9))),
		closedWith("S4", 20, 10, timePtr(day(12))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	assert.Equal(t, []int{13, 21, 8, 20}, seriesValues(v))
	for i, p := range v.Series {
		if i < len(data.Sprints) {
			assert.Equal(t, data.Sprints[i].ID, p.SprintID)
			assert.Equal(t, data.Sprints[i].Name, p.SprintName)
		}
	}
}

// Escenario: la ventana usa el cierre y la serie el inicio (008/US2)
func TestUS2_VelocidadVentanaPorCierreSeriePorInicio(t *testing.T) {
	// El primero en iniciar es el último en cerrar: entra a la ventana y deja afuera al segundo.
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(30))),
		closedWith("S2", 21, 4, timePtr(day(12))),
		closedWith("S3", 8, 7, timePtr(day(13))),
		closedWith("S4", 20, 10, timePtr(day(14))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	// Ventana por cierre: S1 (13), S4 (20), S3 (8) = 41 / 3 = 13,67.
	assert.Equal(t, domain.Available(1367), v.Value)
	assert.Equal(t, 3, v.SprintsConsidered)
	// Serie por inicio: los cuatro, en orden de inicio.
	assert.Equal(t, []int{13, 21, 8, 20}, seriesValues(v))
}

// Escenario: un cerrado sin ClosedAt queda último en la ventana (008/US2)
func TestUS2_VelocidadCerradoSinClosedAtQuedaUltimo(t *testing.T) {
	data := projectOf(
		closedWith("S1", 100, 1, nil),
		closedWith("S2", 3, 4, timePtr(day(6))),
		closedWith("S3", 6, 7, timePtr(day(9))),
		closedWith("S4", 9, 10, timePtr(day(12))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	// El cerrado sin cierre queda fuera de la ventana: (3 + 6 + 9) / 3 = 6,00.
	assert.Equal(t, domain.Available(600), v.Value)
	assert.Equal(t, 3, v.SprintsConsidered)
	assert.Equal(t, []int{100, 3, 6, 9}, seriesValues(v))
}

// Escenario: tres cerrados que no completaron nada dan velocidad 0 (008/US2)
func TestUS2_VelocidadCeroEsPromedioValido(t *testing.T) {
	data := projectOf(
		closedWith("S1", 0, 1, timePtr(day(3))),
		closedWith("S2", 0, 4, timePtr(day(6))),
		closedWith("S3", 0, 7, timePtr(day(9))),
	)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	assert.Equal(t, domain.Available(0), v.Value)
	assert.Equal(t, 3, v.SprintsConsidered)
}

// Escenario: con un Activo y sin Cerrados la velocidad no es calculable (008/US2)
func TestUS2_VelocidadSoloActivoNoCalculable(t *testing.T) {
	active := domain.SprintData{
		ID:        uuid.New(),
		Name:      "Activo",
		Status:    sprintsdomain.SprintStatusActive,
		StartDate: day(1),
		EndDate:   day(14),
		Stories:   stories([]int{5, 3}, 1),
	}

	v := domain.CalculateProject(projectOf(active), fixedNow, testGrace).Velocity

	assert.Equal(t, domain.NotCalculable(), v.Value)
	assert.Equal(t, 0, v.SprintsConsidered)
	assert.Empty(t, v.Series)
}

// Escenario: sin ningún sprint la velocidad no es calculable (008/US2)
func TestUS2_VelocidadSinSprintsNoCalculable(t *testing.T) {
	v := domain.CalculateProject(projectOf(), fixedNow, testGrace).Velocity

	assert.Equal(t, domain.NotCalculable(), v.Value)
	assert.Equal(t, 0, v.SprintsConsidered)
	assert.Empty(t, v.Series)
}

// Escenario: un Planificado no entra en la velocidad (008/US2)
func TestUS2_VelocidadIgnoraPlanificados(t *testing.T) {
	planned := domain.SprintData{
		ID: uuid.New(), Name: "Plan", Status: sprintsdomain.SprintStatusPlanned,
		StartDate: day(20), EndDate: day(25), Stories: stories([]int{40}, 1),
	}
	data := projectOf(closedWith("S1", 13, 1, timePtr(day(3))), planned)

	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity

	assert.Equal(t, domain.Available(1300), v.Value)
	assert.Equal(t, 1, v.SprintsConsidered)
	assert.Equal(t, []int{13}, seriesValues(v))
}

// Escenario: la velocidad nunca es parcial y no lleva Finality (008/US2)
func TestUS2_VelocidadNoTieneFinality(t *testing.T) {
	_, found := reflect.TypeOf(domain.Velocity{}).FieldByName("Finality")
	assert.False(t, found, "Velocity no debe declarar Finality")

	data := projectOf(closedWith("S1", 13, 1, timePtr(day(3))))
	v := domain.CalculateProject(data, fixedNow, testGrace).Velocity
	assert.Equal(t, domain.ValueAvailable, v.Value.Status)
	assert.Nil(t, v.Value.Finality)
}

// plannedSprint arma un sprint Planificado con las historias y el esfuerzo dados.
func plannedSprint(st []domain.SprintStoryData, actual int) domain.SprintData {
	return domain.SprintData{
		ID: uuid.New(), Name: "Plan", Status: sprintsdomain.SprintStatusPlanned,
		StartDate: day(29), EndDate: day(30), Stories: st, ActualHundredths: actual,
	}
}

// spec008Project arma el proyecto de US3-1: dos Cerrados y un Activo, con factor vigente 6.
func spec008Project(extra ...domain.SprintData) domain.ProjectData {
	// Cerrado 1: 21 puntos, 13 completados, 4 historias (2 completas), 142,5 h.
	s1 := closedSprint(stories([]int{8, 5, 5, 3}, 2), 14250, closedLong)
	// Cerrado 2: 24 puntos, 21 completados, 3 historias (2 completas), 130 h.
	s2 := closedSprint(stories([]int{13, 8, 3}, 2), 13000, closedLong)
	s2.StartDate = day(8)
	// Activo: 18 puntos, 5 completados, 4 historias (3 completas), 40 h.
	a := activeSprint(stories([]int{1, 1, 3, 13}, 3), 4000)
	data := projectOf(append([]domain.SprintData{s1, s2, a}, extra...)...)
	data.FactorHundredths = factorSix
	return data
}

// Escenario: acumulados exactos con dos cerrados y un activo (008/US3-1)
func TestUS3_AcumuladoNumerosExactos(t *testing.T) {
	c := domain.CalculateProject(spec008Project(), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.Available(63), c.PlannedStoryPoints)
	assert.Equal(t, domain.Available(39), c.CompletedStoryPoints)
	assert.Equal(t, domain.Available(37800), c.EstimatedHours)
	assert.Equal(t, domain.Available(31250), c.ActualHours)
	assert.Equal(t, domain.Available(-6550), c.DeviationHours)
	assert.Equal(t, domain.Available(-1733), c.DeviationPercent)
}

// Escenario: porcentaje acumulado de historias completadas 7 de 11 (008/US3-2)
func TestUS3_PorcentajeHistoriasCompletadas(t *testing.T) {
	c := domain.CalculateProject(spec008Project(), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.Available(6364), c.StoryCompletionPercent)
}

// Escenario: el sprint Planificado no entra en los acumulados (008/US3-3)
func TestUS3_AcumuladoIgnoraPlanificados(t *testing.T) {
	planned := plannedSprint(stories([]int{40, 13}, 0), 900)

	c := domain.CalculateProject(spec008Project(planned), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.Available(63), c.PlannedStoryPoints)
	assert.Equal(t, domain.Available(39), c.CompletedStoryPoints)
	assert.Equal(t, domain.Available(37800), c.EstimatedHours)
	assert.Equal(t, domain.Available(31250), c.ActualHours)
	assert.Equal(t, domain.Available(6364), c.StoryCompletionPercent)
}

// Escenario: el acumulado con un Activo es parcial (008/US3-4)
func TestUS3_AcumuladoParcial(t *testing.T) {
	c := domain.CalculateProject(spec008Project(), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.FinalityPartial, c.Finality)
	assert.True(t, c.IncludesActiveSprint)
}

// Escenario: el acumulado sin Activo es definitivo (008/US3-5)
func TestUS3_AcumuladoDefinitivo(t *testing.T) {
	data := spec008Project()
	data.Sprints = data.Sprints[:2]

	c := domain.CalculateProject(data, fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.FinalityDefinitive, c.Finality)
	assert.False(t, c.IncludesActiveSprint)
	assert.Equal(t, domain.Available(45), c.PlannedStoryPoints)
	assert.Equal(t, domain.Available(34), c.CompletedStoryPoints)
}

// Escenario: una historia en dos sprints aporta dos planificados y un completado (008/US3-6)
func TestUS3_HistoriaEnDosSprints(t *testing.T) {
	story := domain.SprintStoryData{StoryID: uuid.New(), Title: "h", StoryPoints: intPtr(5)}
	first := closedSprint([]domain.SprintStoryData{story}, 0, closedLong)
	story.Completed = true
	second := closedSprint([]domain.SprintStoryData{story}, 0, closedLong)
	second.StartDate = day(8)

	c := domain.CalculateProject(projectOf(first, second), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.Available(10), c.PlannedStoryPoints)
	assert.Equal(t, domain.Available(5), c.CompletedStoryPoints)
}

// Escenario: el cociente sale de los totales y no del promedio de porcentajes (008/US3-1, RN-11)
func TestUS3_CocienteSalePorTotales(t *testing.T) {
	// Sprint A: 1 de 1 historia completa (100 %), desvío +100 %; sprint B: 1 de 4 (25 %), desvío 0 %.
	a := closedSprint(stories([]int{1}, 1), 1200, closedLong)
	b := closedSprint(stories([]int{1, 1, 1, 1}, 1), 2400, closedLong)
	b.StartDate = day(8)

	c := domain.CalculateProject(projectOf(a, b), fixedNow, testGrace).Cumulative

	// Totales: 2 de 5 historias = 40,00 % (el promedio de 100 y 25 daría 62,50 %).
	assert.Equal(t, domain.Available(4000), c.StoryCompletionPercent)
	// Estimadas 600 + 2400 = 3000; reales 3600; desvío +600 = 20,00 % (el promedio daría 50,00 %).
	assert.Equal(t, domain.Available(600), c.DeviationHours)
	assert.Equal(t, domain.Available(2000), c.DeviationPercent)
}

// Escenario: solo Planificados o sin sprints dan totales en 0 y cocientes no calculables (008/US3-7, US3-8)
func TestUS3_SinSprintsIncluidos(t *testing.T) {
	cases := map[string]domain.ProjectData{
		"solo planificados": projectOf(plannedSprint(stories([]int{8}, 0), 0)),
		"sin sprints":       projectOf(),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			c := domain.CalculateProject(data, fixedNow, testGrace).Cumulative

			assert.Equal(t, domain.Available(0), c.PlannedStoryPoints)
			assert.Equal(t, domain.Available(0), c.CompletedStoryPoints)
			assert.Equal(t, domain.Available(0), c.EstimatedHours)
			assert.Equal(t, domain.Available(0), c.ActualHours)
			assert.Equal(t, domain.Available(0), c.DeviationHours)
			assert.Equal(t, domain.NotCalculable(), c.DeviationPercent)
			assert.Equal(t, domain.NotCalculable(), c.StoryCompletionPercent)
			assert.Equal(t, domain.FinalityDefinitive, c.Finality)
		})
	}
}

// Escenario: estimadas 0 con reales da desvío igual a las reales y porcentual no calculable (008/US3-9)
func TestUS3_EstimadasCeroConReales(t *testing.T) {
	// Un cerrado sin historias: 0 estimadas y 12 h reales.
	s := closedSprint(nil, 1200, closedLong)

	c := domain.CalculateProject(projectOf(s), fixedNow, testGrace).Cumulative

	assert.Equal(t, domain.Available(0), c.EstimatedHours)
	assert.Equal(t, domain.Available(1200), c.ActualHours)
	assert.Equal(t, domain.Available(1200), c.DeviationHours)
	assert.Equal(t, domain.NotCalculable(), c.DeviationPercent)
}

// Escenario: esfuerzo fuera de sprint separado de las horas reales acumuladas (008/US3-10)
func TestUS3_EsfuerzoFueraDeSprint(t *testing.T) {
	data := spec008Project()
	data.OutsideSprintsHundredths = 1800

	m := domain.CalculateProject(data, fixedNow, testGrace)

	assert.Equal(t, 1800, m.EffortOutsideSprintsHundredths)
	assert.Equal(t, domain.Available(31250), m.Cumulative.ActualHours)
}

// Escenario: sin esfuerzo fuera de sprint la cifra viene en 0 (008/US3-11)
func TestUS3_EsfuerzoFueraDeSprintEnCero(t *testing.T) {
	m := domain.CalculateProject(spec008Project(), fixedNow, testGrace)

	assert.Equal(t, 0, m.EffortOutsideSprintsHundredths)
}

// Escenario: esfuerzo en sprints Planificados separado de las horas reales acumuladas (008/US3-12)
func TestUS3_EsfuerzoEnSprintsPlanificados(t *testing.T) {
	t.Run("con esfuerzo", func(t *testing.T) {
		first := plannedSprint(stories([]int{5}, 0), 900)
		second := plannedSprint(stories([]int{3}, 0), 100)

		m := domain.CalculateProject(spec008Project(first, second), fixedNow, testGrace)

		assert.Equal(t, 1000, m.EffortInPlannedSprintsHundredths)
		assert.Equal(t, domain.Available(31250), m.Cumulative.ActualHours)
	})
	t.Run("sin esfuerzo en planificados", func(t *testing.T) {
		m := domain.CalculateProject(spec008Project(), fixedNow, testGrace)

		assert.Equal(t, 0, m.EffortInPlannedSprintsHundredths)
	})
}

// planningWarnings filtra las advertencias de planificación.
func planningWarnings(ws []domain.Warning) []domain.Warning {
	out := []domain.Warning{}
	for _, w := range ws {
		if w.Kind == domain.WarningPlanning {
			out = append(out, w)
		}
	}
	return out
}

// Escenario: historias del backlog sin estimar generan la advertencia con su contador (008/US3-13, US3-14)
func TestUS3_BacklogSinEstimar(t *testing.T) {
	t.Run("con 6 sin estimar", func(t *testing.T) {
		data := spec008Project()
		data.UnestimatedBacklogStories = 6

		m := domain.CalculateProject(data, fixedNow, testGrace)

		assert.Equal(t, 6, m.UnestimatedBacklogStories)
		got := planningWarnings(m.Warnings)
		if assert.Len(t, got, 1) {
			assert.Equal(t, domain.CodeUnestimatedBacklogStories, got[0].Code)
			if assert.NotNil(t, got[0].Count) {
				assert.Equal(t, 6, *got[0].Count)
			}
			assert.Nil(t, got[0].SprintID)
			assert.Nil(t, got[0].StoryID)
		}
	})
	t.Run("con 0 sin estimar no hay advertencia", func(t *testing.T) {
		m := domain.CalculateProject(spec008Project(), fixedNow, testGrace)

		assert.Equal(t, 0, m.UnestimatedBacklogStories)
		assert.Empty(t, planningWarnings(m.Warnings))
	})
}

// Escenario: CalculateSprint nunca produce la advertencia de backlog (008/US3-13, RN-26)
func TestUS3_SprintNuncaAdvierteBacklog(t *testing.T) {
	for _, s := range spec008Project().Sprints {
		_, warnings := domain.CalculateSprint(s, factorSix, fixedNow, testGrace)

		for _, w := range warnings {
			assert.NotEqual(t, domain.CodeUnestimatedBacklogStories, w.Code)
			assert.NotEqual(t, domain.WarningPlanning, w.Kind)
		}
	}
}

// Escenario: las advertencias de inconsistencia viajan junto a las de planificación (008/US3-13, RN-31)
func TestUS3_AdvertenciasConviven(t *testing.T) {
	data := spec008Project()
	data.UnestimatedBacklogStories = 2
	// Una historia sin puntos en el primer Cerrado: se cuenta como 0 y se advierte.
	broken := data.Sprints[0]
	broken.Stories = append(slices.Clone(broken.Stories), domain.SprintStoryData{StoryID: uuid.New(), Title: "rota"})
	data.Sprints[0] = broken

	m := domain.CalculateProject(data, fixedNow, testGrace)

	var inconsistencies []domain.Warning
	for _, w := range m.Warnings {
		if w.Kind == domain.WarningDataInconsistency {
			inconsistencies = append(inconsistencies, w)
		}
	}
	if assert.Len(t, inconsistencies, 1) {
		assert.Equal(t, domain.CodeUnestimatedStoryInSprint, inconsistencies[0].Code)
		if assert.NotNil(t, inconsistencies[0].SprintID) {
			assert.Equal(t, broken.ID, *inconsistencies[0].SprintID)
		}
	}
	assert.Len(t, planningWarnings(m.Warnings), 1)
	// Ninguna métrica se quita: los acumulados siguen disponibles.
	assert.Equal(t, domain.Available(63), m.Cumulative.PlannedStoryPoints)
	assert.Equal(t, domain.Available(-1733), m.Cumulative.DeviationPercent)
	assert.Equal(t, domain.ValueAvailable, m.Velocity.Value.Status)
}

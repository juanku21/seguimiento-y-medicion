package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/metrics/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// allSeries devuelve las seis listas de una serie de proyecto en un orden fijo.
func allSeries(s domain.ProjectSeries) [][]domain.SeriesPoint {
	return [][]domain.SeriesPoint{
		s.PlannedStoryPoints, s.CompletedStoryPoints, s.EstimatedHours,
		s.ActualHours, s.DefectsDetected, s.DefectsResolved,
	}
}

// activeSprint arma un sprint Activo con historias y horas reales.
func activeWithData(start int) domain.SprintData {
	return domain.SprintData{
		ID:               uuid.New(),
		Name:             "Activo",
		Status:           sprintsdomain.SprintStatusActive,
		StartDate:        day(start),
		EndDate:          day(start + 5),
		Stories:          stories([]int{5, 3}, 1),
		ActualHundredths: 1000,
		DefectsDetected:  2,
	}
}

// Escenario: seis series con los mismos sprints en el mismo orden (008/US2)
func TestUS2_SeriesSeisListasMismosSprints(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(30))),
		closedWith("S2", 21, 4, timePtr(day(12))),
		closedWith("S3", 8, 7, timePtr(day(13))),
	)

	series, _ := domain.BuildSeries(data, fixedNow, testGrace)

	for i, list := range allSeries(series) {
		if assert.Len(t, list, 3, "serie %d", i) {
			for j, p := range list {
				assert.Equal(t, data.Sprints[j].ID, p.SprintID, "serie %d punto %d", i, j)
				assert.Equal(t, data.Sprints[j].Name, p.SprintName)
				assert.Equal(t, sprintsdomain.SprintStatusClosed, p.SprintStatus)
			}
		}
	}
}

// Escenario: cada punto es igual a la métrica de su sprint (008/US2)
func TestUS2_SeriesPuntoIgualAMetricaDelSprint(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(closedLong)),
		closedWith("S2", 21, 4, timePtr(closedNear)),
		activeWithData(10),
	)

	series, _ := domain.BuildSeries(data, fixedNow, testGrace)

	if !assert.Len(t, series.PlannedStoryPoints, 3) ||
		!assert.Len(t, series.CompletedStoryPoints, 3) ||
		!assert.Len(t, series.EstimatedHours, 3) ||
		!assert.Len(t, series.ActualHours, 3) ||
		!assert.Len(t, series.DefectsDetected, 3) ||
		!assert.Len(t, series.DefectsResolved, 3) {
		return
	}
	for i, s := range data.Sprints {
		want, _ := domain.CalculateSprint(s, data.FactorHundredths, fixedNow, testGrace)
		assert.Equal(t, want.PlannedStoryPoints, series.PlannedStoryPoints[i].Value)
		assert.Equal(t, want.CompletedStoryPoints, series.CompletedStoryPoints[i].Value)
		assert.Equal(t, want.EstimatedHours, series.EstimatedHours[i].Value)
		assert.Equal(t, want.ActualHours, series.ActualHours[i].Value)
		assert.Equal(t, want.DefectsDetected, series.DefectsDetected[i].Value)
		assert.Equal(t, want.DefectsResolved, series.DefectsResolved[i].Value)
	}
}

// Escenario: con un Activo, las seis incluyen los 4 sprints y el Activo es parcial (008/US2)
func TestUS2_SeriesIncluyenActivoParcial(t *testing.T) {
	data := projectOf(
		closedWith("S1", 13, 1, timePtr(day(3))),
		closedWith("S2", 21, 4, timePtr(day(6))),
		closedWith("S3", 8, 7, timePtr(day(9))),
		activeWithData(10),
	)

	series, _ := domain.BuildSeries(data, fixedNow, testGrace)

	partial := domain.FinalityPartial
	for i, list := range allSeries(series) {
		if assert.Len(t, list, 4, "serie %d", i) {
			last := list[3]
			assert.Equal(t, sprintsdomain.SprintStatusActive, last.SprintStatus)
			assert.Equal(t, &partial, last.Value.Finality, "serie %d: el Activo es parcial", i)
			assert.NotEqual(t, &partial, list[0].Value.Finality, "serie %d: un Cerrado no es parcial", i)
		}
	}
}

// Escenario: un Planificado no aparece en ninguna serie (008/US2)
func TestUS2_SeriesExcluyenPlanificado(t *testing.T) {
	planned := domain.SprintData{
		ID: uuid.New(), Name: "Plan", Status: sprintsdomain.SprintStatusPlanned,
		StartDate: day(20), EndDate: day(25), Stories: stories([]int{40}, 0),
	}
	data := projectOf(closedWith("S1", 13, 1, timePtr(day(3))), planned)

	series, _ := domain.BuildSeries(data, fixedNow, testGrace)

	for i, list := range allSeries(series) {
		if assert.Len(t, list, 1, "serie %d", i) {
			assert.Equal(t, data.Sprints[0].ID, list[0].SprintID)
		}
	}
}

// Escenario: un sprint sin datos aparece con 0 en las seis series (008/US2)
func TestUS2_SeriesSprintSinDatosEsCero(t *testing.T) {
	empty := domain.SprintData{
		ID:                     uuid.New(),
		Name:                   "Vacio",
		Status:                 sprintsdomain.SprintStatusClosed,
		StartDate:              day(1),
		EndDate:                day(5),
		ClosedAt:               timePtr(closedLong),
		FrozenFactorHundredths: intPtr(factorSix),
	}

	series, _ := domain.BuildSeries(projectOf(empty), fixedNow, testGrace)

	for i, list := range allSeries(series) {
		if assert.Len(t, list, 1, "serie %d", i) {
			assert.Equal(t, domain.ValueAvailable, list[0].Value.Status, "serie %d", i)
			assert.Equal(t, 0, list[0].Value.Value, "serie %d", i)
		}
	}
}

// Escenario: sin Cerrados ni Activo las seis series vienen vacías (008/US2)
func TestUS2_SeriesVaciasSinCerradosNiActivo(t *testing.T) {
	t.Run("sin sprints", func(t *testing.T) {
		series, warnings := domain.BuildSeries(projectOf(), fixedNow, testGrace)

		for i, list := range allSeries(series) {
			assert.Len(t, list, 0, "serie %d", i)
		}
		assert.Empty(t, warnings)
	})
	t.Run("solo planificado", func(t *testing.T) {
		planned := domain.SprintData{
			ID: uuid.New(), Name: "Plan", Status: sprintsdomain.SprintStatusPlanned,
			StartDate: day(20), EndDate: day(25),
		}

		series, _ := domain.BuildSeries(projectOf(planned), fixedNow, testGrace)

		for i, list := range allSeries(series) {
			assert.Len(t, list, 0, "serie %d", i)
		}
	})
}

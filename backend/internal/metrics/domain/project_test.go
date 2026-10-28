package domain_test

import (
	"reflect"
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
	// Con 0 puntos completados la única historia queda sin completar.
	completedCount := 1
	if completedPoints == 0 {
		completedCount = 0
	}
	return domain.SprintData{
		ID:                     uuid.New(),
		Name:                   name,
		Status:                 sprintsdomain.SprintStatusClosed,
		StartDate:              day(start),
		EndDate:                day(start + 1),
		ClosedAt:               closedAt,
		FrozenFactorHundredths: intPtr(factorSix),
		Stories:                stories([]int{max(completedPoints, 5)}, completedCount),
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

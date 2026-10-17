package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias del cálculo de horas estimadas (T018, RN-08). Los nombres llevan el prefijo
// "Estimacion" para no chocar con las pruebas de integración de los mismos escenarios. El
// resultado está en centésimas de hora: 10400 equivale a 104,00 horas.

// Escenario: Horas estimadas con factor 6,5 (002/US4)
// Caso límite 5 de US4: con historias de 3, 5 y 8 Story Points (16 en total) y factor 6,5 (650
// centésimas), las horas estimadas son exactamente 104,00, sin pérdida por decimales.
func TestUS4_EstimacionHorasEstimadas(t *testing.T) {
	assert.Equal(t, 10400, EstimatedHoursHundredths(16, 650))
}

// Escenario: Proyecto recién creado sin backlog (002/US4)
// Caso límite 4 de US4: sin Story Points estimados las horas estimadas son 0 para cualquier factor
// válido, incluidos el mínimo y el máximo.
func TestUS4_EstimacionProyectoRecienCreado(t *testing.T) {
	factors := []struct {
		name       string
		hundredths int
	}{
		{name: "factor mínimo 0,01", hundredths: 1},
		{name: "factor 6,5", hundredths: 650},
		{name: "factor máximo 40", hundredths: 4000},
	}

	// Cada factor se prueba por separado para identificar cuál falla.
	for _, tc := range factors {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, 0, EstimatedHoursHundredths(0, tc.hundredths))
		})
	}
}

// Escenario: Límites del factor (002/US1)
// Caso límite 5 de US1: los factores 0,01 (1 centésima) y 40 (4000 centésimas) son el mínimo y el
// máximo válidos, y el producto entero se mantiene exacto en ambos extremos.
func TestUS1_EstimacionLimitesDelFactor(t *testing.T) {
	cases := []struct {
		name        string
		storyPoints int
		hundredths  int
		want        int
	}{
		{name: "factor mínimo con 1 Story Point", storyPoints: 1, hundredths: 1, want: 1},
		{name: "factor mínimo con 16 Story Points", storyPoints: 16, hundredths: 1, want: 16},
		{name: "factor máximo con 1 Story Point", storyPoints: 1, hundredths: 4000, want: 4000},
		{name: "factor máximo con 16 Story Points", storyPoints: 16, hundredths: 4000, want: 64000},
	}

	// Cada combinación se verifica como subtest independiente.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EstimatedHoursHundredths(tc.storyPoints, tc.hundredths))
		})
	}
}

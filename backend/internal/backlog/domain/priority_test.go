package domain

import "testing"

// Pruebas unitarias obligatorias del rango de orden de la prioridad (T029, RN-17; data-model.md
// sección 3). Los nombres llevan el prefijo "Dominio" para no chocar con las pruebas de integración
// de handler_test.go (T030).

// Escenario: Orden por prioridad y fecha (003/US2)
// Cada prioridad tiene su rango fijo: high 0, medium 1 y low 2, de modo que el listado muestre
// primero las Altas, después las Medias y al final las Bajas.
func TestUS2_DominioRangoDePrioridad(t *testing.T) {
	cases := []struct {
		priority StoryPriority
		want     int
	}{
		{StoryPriorityHigh, 0},
		{StoryPriorityMedium, 1},
		{StoryPriorityLow, 2},
	}
	for _, tc := range cases {
		t.Run(string(tc.priority), func(t *testing.T) {
			if got := tc.priority.Rank(); got != tc.want {
				t.Errorf("%q.Rank() = %d; se esperaba %d", tc.priority, got, tc.want)
			}
		})
	}
}

// Escenario: Orden por prioridad y fecha (003/US2)
// El rango ordena estrictamente de Alta a Baja: ninguna prioridad comparte rango con otra.
func TestUS2_DominioRangoOrdenaDeAltaABaja(t *testing.T) {
	high, medium, low := StoryPriorityHigh.Rank(), StoryPriorityMedium.Rank(), StoryPriorityLow.Rank()
	if high >= medium || medium >= low {
		t.Errorf("se esperaba high < medium < low; se obtuvo high=%d, medium=%d, low=%d", high, medium, low)
	}
}

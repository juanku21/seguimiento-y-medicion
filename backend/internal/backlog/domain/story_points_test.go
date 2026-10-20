package domain

import "testing"

// Pruebas unitarias obligatorias de la escala de Story Points (RN-06, RN-07, SC-007). Los nombres
// llevan el prefijo "Dominio" para no chocar con las pruebas de integración de handler_test.go
// (T015), que cubren los mismos escenarios de punta a punta con TestUS1_CeroStoryPoints y
// TestUS1_StoryPointsFueraDeEscala.

// Escenario: Cero Story Points (003/US1)
// Los diez valores de la escala se aceptan, incluido 0, que es una estimación y no la ausencia de
// ella (RN-07).
func TestUS1_DominioEscalaDeStoryPoints(t *testing.T) {
	for _, value := range []int{0, 1, 2, 3, 5, 8, 13, 20, 40, 100} {
		if !IsValidStoryPoints(value) {
			t.Errorf("IsValidStoryPoints(%d) = false; el valor pertenece a la escala", value)
		}
	}
}

// Escenario: Story Points fuera de la escala (003/US1)
// Los enteros intermedios, el posterior al máximo y los negativos se rechazan (SC-007).
func TestUS1_DominioStoryPointsFueraDeEscala(t *testing.T) {
	for _, value := range []int{4, 6, 7, 50, 101, -1} {
		if IsValidStoryPoints(value) {
			t.Errorf("IsValidStoryPoints(%d) = true; el valor está fuera de la escala", value)
		}
	}
}

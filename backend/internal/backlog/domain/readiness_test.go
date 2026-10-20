package domain

import (
	"fmt"
	"slices"
	"testing"
)

// Pruebas unitarias obligatorias de la regla "lista para planificar" (RN-12, RN-13, SC-009) con las
// cinco combinaciones de data-model.md, sección 5. Los nombres llevan el prefijo "Dominio" para no
// chocar con las pruebas de integración de handler_test.go (T015).

// intPtr devuelve un puntero al valor, para expresar Story Points estimados en las tablas de casos.
func intPtr(value int) *int {
	return &value
}

// assertReadiness compara el resultado de Readiness con lo esperado: ready y la lista missing con
// sus motivos en orden (story_points antes que acceptance_criteria).
func assertReadiness(t *testing.T, storyPoints *int, criteriaCount int, wantReady bool, wantMissing []string) {
	t.Helper()
	ready, missing := Readiness(storyPoints, criteriaCount)
	if ready != wantReady {
		t.Errorf("Readiness(%v, %d) ready = %v; se esperaba %v", describePoints(storyPoints), criteriaCount, ready, wantReady)
	}
	if len(wantMissing) == 0 && len(missing) == 0 {
		return
	}
	if !slices.Equal(missing, wantMissing) {
		t.Errorf("Readiness(%v, %d) missing = %v; se esperaba %v", describePoints(storyPoints), criteriaCount, missing, wantMissing)
	}
}

// describePoints muestra los Story Points en los mensajes de fallo, distinguiendo "sin estimar".
func describePoints(storyPoints *int) string {
	if storyPoints == nil {
		return "nil"
	}
	return fmt.Sprint(*storyPoints)
}

// Escenario: Creación completa (003/US1)
// Con Story Points estimados y criterios de aceptación, la historia está lista y no falta nada.
func TestUS1_DominioCreacionCompleta(t *testing.T) {
	assertReadiness(t, intPtr(5), 2, true, nil)
}

// Escenario: Cero Story Points (003/US1)
// El 0 es una estimación válida: con un criterio, la historia queda lista para planificar (RN-07).
func TestUS1_DominioCeroStoryPointsListaParaPlanificar(t *testing.T) {
	assertReadiness(t, intPtr(0), 1, true, nil)
}

// Escenario: Solo título y prioridad (003/US1)
// Si falta alguna condición, la historia no está lista y se informa qué falta, siempre en el orden
// story_points, acceptance_criteria (SC-009).
func TestUS1_DominioSoloTituloYPrioridad(t *testing.T) {
	t.Run("sin estimar con criterios", func(t *testing.T) {
		assertReadiness(t, nil, 2, false, []string{"story_points"})
	})
	t.Run("estimada sin criterios", func(t *testing.T) {
		assertReadiness(t, intPtr(3), 0, false, []string{"acceptance_criteria"})
	})
	t.Run("sin estimar y sin criterios", func(t *testing.T) {
		assertReadiness(t, nil, 0, false, []string{"story_points", "acceptance_criteria"})
	})
}

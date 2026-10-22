package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de la máquina de estados del sprint (T039; RN-02, RN-18, SC-006;
// data-model.md sección 4). Recorren la matriz completa de los nueve pares origen-destino: solo
// planned -> active y active -> closed se aceptan; los otros siete, incluida la permanencia en el
// mismo estado, se rechazan con ErrInvalidSprintTransition. No llevan la etiqueta de un escenario de
// la spec: los escenarios de inicio y cierre se cubren con las pruebas de integración de US3 y US5.
func TestSprintStatusValidateTransition(t *testing.T) {
	cases := []struct {
		name    string
		from    SprintStatus
		to      SprintStatus
		allowed bool
	}{
		{name: "planned a planned", from: SprintStatusPlanned, to: SprintStatusPlanned, allowed: false},
		{name: "planned a active", from: SprintStatusPlanned, to: SprintStatusActive, allowed: true},
		{name: "planned a closed", from: SprintStatusPlanned, to: SprintStatusClosed, allowed: false},
		{name: "active a planned", from: SprintStatusActive, to: SprintStatusPlanned, allowed: false},
		{name: "active a active", from: SprintStatusActive, to: SprintStatusActive, allowed: false},
		{name: "active a closed", from: SprintStatusActive, to: SprintStatusClosed, allowed: true},
		{name: "closed a planned", from: SprintStatusClosed, to: SprintStatusPlanned, allowed: false},
		{name: "closed a active", from: SprintStatusClosed, to: SprintStatusActive, allowed: false},
		{name: "closed a closed", from: SprintStatusClosed, to: SprintStatusClosed, allowed: false},
	}

	// Cada par se prueba en un subtest propio: los permitidos no devuelven error y los demás devuelven
	// el centinela de transición inválida, que delivery traduce a la respuesta HTTP.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.ValidateTransition(tc.to)
			if tc.allowed {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrInvalidSprintTransition)
		})
	}
}

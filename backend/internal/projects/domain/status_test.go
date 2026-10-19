package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de la máquina de estados del proyecto (T070, RN-09, SC-009): solo
// se aceptan planned -> in_progress e in_progress -> finished, y se rechazan las seis combinaciones
// restantes que enumera la tarea. Los nombres llevan el prefijo "Transicion" para no chocar con las
// pruebas de integración de los mismos escenarios (T072).

// statusTransition es un par de estado origen y destino con el nombre del subtest que lo identifica.
type statusTransition struct {
	name string
	from ProjectStatus
	to   ProjectStatus
}

// assertRejected verifica que cada transición se rechace con el error de transición inválida, en un
// subtest propio para identificar cuál falla.
func assertRejected(t *testing.T, transitions []statusTransition) {
	t.Helper()
	for _, tc := range transitions {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.ValidateTransition(tc.to)
			assert.ErrorIs(t, err, ErrInvalidStatusTransition)
		})
	}
}

// Escenario: Recorrido completo (002/US6)
// Caso normal 1 de US6: las dos únicas transiciones válidas, planned -> in_progress e
// in_progress -> finished, se aceptan sin error (FR-037).
func TestUS6_TransicionRecorridoCompleto(t *testing.T) {
	transitions := []statusTransition{
		{name: "planned a in_progress", from: ProjectStatusPlanned, to: ProjectStatusInProgress},
		{name: "in_progress a finished", from: ProjectStatusInProgress, to: ProjectStatusFinished},
	}

	// Cada transición válida se prueba por separado para identificar cuál falla.
	for _, tc := range transitions {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, tc.from.ValidateTransition(tc.to))
		})
	}
}

// Escenario: Retroceso rechazado (002/US6)
// Caso de error 6 de US6: los estados solo avanzan, así que todo retroceso a un estado anterior se
// rechaza como transición inválida (FR-038, RN-09).
func TestUS6_TransicionRetrocesoRechazado(t *testing.T) {
	assertRejected(t, []statusTransition{
		{name: "in_progress a planned", from: ProjectStatusInProgress, to: ProjectStatusPlanned},
		{name: "finished a planned", from: ProjectStatusFinished, to: ProjectStatusPlanned},
		{name: "finished a in_progress", from: ProjectStatusFinished, to: ProjectStatusInProgress},
	})
}

// Escenario: Salto de estado rechazado (002/US6)
// Caso de error 7 de US6: pasar de planned directamente a finished se rechaza porque hay que pasar
// primero por in_progress (FR-038).
func TestUS6_TransicionSaltoDeEstadoRechazado(t *testing.T) {
	assertRejected(t, []statusTransition{
		{name: "planned a finished", from: ProjectStatusPlanned, to: ProjectStatusFinished},
	})
}

// Escenario: Mismo estado rechazado (002/US6)
// Caso de error 8 de US6: la transición al estado actual se rechaza como inválida (FR-038).
func TestUS6_TransicionMismoEstadoRechazado(t *testing.T) {
	assertRejected(t, []statusTransition{
		{name: "planned a planned", from: ProjectStatusPlanned, to: ProjectStatusPlanned},
		{name: "in_progress a in_progress", from: ProjectStatusInProgress, to: ProjectStatusInProgress},
	})
}

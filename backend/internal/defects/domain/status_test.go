package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Fechas de inicio de referencia: el sprint de detección empieza en detectionStart; los demás
// sprints se ubican antes o después para probar RN-14.
var (
	sprintBeforeStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	detectionStart    = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	sprintAfterStart  = time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	sprintLaterStart  = time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
)

// newAnchor arma la vista de un sprint con un identificador nuevo, el estado y la fecha de inicio.
func newAnchor(status sprintsdomain.SprintStatus, start time.Time) SprintAnchor {
	return SprintAnchor{ID: uuid.New(), Status: status, StartDate: start}
}

// assertResolvedSprint verifica que PlanTransition aceptó la resolución y devolvió el ID del sprint
// de resolución indicado.
func assertResolvedSprint(t *testing.T, got *uuid.UUID, err error, want uuid.UUID) {
	t.Helper()
	if err != nil {
		t.Fatalf("PlanTransition devolvió el error %v, se esperaba aceptar la resolución", err)
	}
	if got == nil {
		t.Fatalf("PlanTransition devolvió nil, se esperaba el sprint de resolución %s", want)
	}
	if *got != want {
		t.Errorf("PlanTransition devolvió el sprint %s, se esperaba %s", *got, want)
	}
}

// assertRejected verifica que PlanTransition rechazó la transición con el error esperado y sin
// sprint de resolución.
func assertRejected(t *testing.T, got *uuid.UUID, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("PlanTransition devolvió el error %v, se esperaba %v", err, want)
	}
	if got != nil {
		t.Errorf("PlanTransition devolvió el sprint %s junto con el error, se esperaba nil", *got)
	}
}

// Escenario: Los estados del defecto son Abierto, En progreso, Resuelto y Descartado, RN-11 (007/US2)
func TestUS2_EstadoValido(t *testing.T) {
	cases := []struct {
		name  string
		value DefectStatus
		valid bool
	}{
		{"open", DefectStatusOpen, true},
		{"in_progress", DefectStatusInProgress, true},
		{"resolved", DefectStatusResolved, true},
		{"discarded", DefectStatusDiscarded, true},
		{"closed no existe", "closed", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.value.Valid(); got != c.valid {
				t.Errorf("Valid(%q) = %v, se esperaba %v", c.value, got, c.valid)
			}
		})
	}
}

// Escenario: Transiciones admitidas de Abierto a En progreso y de En progreso a Resuelto, RN-12 (007/US2)
func TestUS2_TransicionesAdmitidas(t *testing.T) {
	cases := []struct {
		current DefectStatus
		want    []DefectStatus
	}{
		{DefectStatusOpen, []DefectStatus{DefectStatusInProgress}},
		{DefectStatusInProgress, []DefectStatus{DefectStatusResolved}},
	}
	for _, c := range cases {
		t.Run(string(c.current), func(t *testing.T) {
			if got := AllowedTransitions(c.current); !slices.Equal(got, c.want) {
				t.Errorf("AllowedTransitions(%q) = %v, se esperaba %v", c.current, got, c.want)
			}
		})
	}
}

// Escenario: Abierto pasa a En progreso y más tarde a Resuelto con el sprint Activo (007/US2)
func TestUS2_CicloCompleto(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)

	// Pasar a En progreso no registra sprint de resolución.
	t.Run("open a in_progress", func(t *testing.T) {
		got, err := PlanTransition(DefectStatusOpen, DefectStatusInProgress, detected, nil)
		if err != nil {
			t.Fatalf("PlanTransition devolvió el error %v, se esperaba nil", err)
		}
		if got != nil {
			t.Errorf("PlanTransition devolvió el sprint %s, se esperaba nil", *got)
		}
	})

	// Resolver con el sprint Activo posterior registra ese sprint.
	t.Run("in_progress a resolved con sprint activo", func(t *testing.T) {
		resolution := newAnchor(sprintsdomain.SprintStatusActive, sprintAfterStart)
		got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &resolution)
		assertResolvedSprint(t, got, err, resolution.ID)
	})
}

// Escenario: Defecto detectado en un sprint Cerrado se resuelve en un sprint posterior, Activo o Cerrado (007/US2)
func TestUS2_ResolucionEnSprintPosterior(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)
	cases := []struct {
		name       string
		resolution SprintAnchor
	}{
		{"sprint activo posterior", newAnchor(sprintsdomain.SprintStatusActive, sprintLaterStart)},
		{"otro sprint cerrado posterior", newAnchor(sprintsdomain.SprintStatusClosed, sprintAfterStart)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resolution := c.resolution
			got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &resolution)
			assertResolvedSprint(t, got, err, resolution.ID)
		})
	}
}

// Escenario: Defecto detectado en un sprint se resuelve en ese mismo sprint, FR-025 y RN-14 (007/US2)
func TestUS2_ResolucionEnElMismoSprint(t *testing.T) {
	cases := []struct {
		name   string
		status sprintsdomain.SprintStatus
	}{
		{"mismo sprint activo", sprintsdomain.SprintStatusActive},
		{"unico sprint del proyecto, cerrado", sprintsdomain.SprintStatusClosed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			detected := newAnchor(c.status, detectionStart)
			resolution := detected
			got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &resolution)
			assertResolvedSprint(t, got, err, detected.ID)
		})
	}
}

// Escenario: Resolver sin indicar sprint de resolución se rechaza, RN-13 (007/US2)
func TestUS2_SinSprintDeResolucion(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusActive, detectionStart)
	got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, nil)
	assertRejected(t, got, err, ErrResolutionSprintRequired)
}

// Escenario: Un sprint Planificado como sprint de resolución se rechaza, RN-13 (007/US2)
func TestUS2_SprintDeResolucionPlanificado(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusActive, detectionStart)
	resolution := newAnchor(sprintsdomain.SprintStatusPlanned, sprintAfterStart)
	got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &resolution)
	assertRejected(t, got, err, ErrResolutionSprintPlanned)
}

// Escenario: Sprint de resolución que empieza antes que el de detección se rechaza, RN-14 (007/US2)
func TestUS2_ResolucionAntesDeLaDeteccion(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusActive, detectionStart)
	resolution := newAnchor(sprintsdomain.SprintStatusClosed, sprintBeforeStart)
	got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &resolution)
	assertRejected(t, got, err, ErrResolutionBeforeDetection)
}

// Escenario: Transiciones no admitidas, como Abierto a Resuelto o En progreso a Abierto, se rechazan con el estado vigente y las permitidas, RN-12 (007/US2)
func TestUS2_TransicionNoAdmitida(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)
	resolution := newAnchor(sprintsdomain.SprintStatusActive, sprintAfterStart)
	cases := []struct {
		name    string
		current DefectStatus
		to      DefectStatus
	}{
		{"open a resolved", DefectStatusOpen, DefectStatusResolved},
		{"in_progress a open", DefectStatusInProgress, DefectStatusOpen},
		{"open a open", DefectStatusOpen, DefectStatusOpen},
		{"in_progress a in_progress", DefectStatusInProgress, DefectStatusInProgress},
		{"resolved a in_progress", DefectStatusResolved, DefectStatusInProgress},
		{"resolved a resolved", DefectStatusResolved, DefectStatusResolved},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Se envía un sprint de resolución válido para que el rechazo dependa solo de la
			// transición y no de la falta de sprint.
			res := resolution
			got, err := PlanTransition(c.current, c.to, detected, &res)
			var transitionErr *InvalidTransitionError
			if !errors.As(err, &transitionErr) {
				t.Fatalf("PlanTransition(%q, %q) devolvió el error %v, se esperaba *InvalidTransitionError", c.current, c.to, err)
			}
			if got != nil {
				t.Errorf("PlanTransition devolvió el sprint %s junto con el error, se esperaba nil", *got)
			}
			if transitionErr.Current != c.current {
				t.Errorf("Current = %q, se esperaba %q", transitionErr.Current, c.current)
			}
			if transitionErr.Requested != c.to {
				t.Errorf("Requested = %q, se esperaba %q", transitionErr.Requested, c.to)
			}
			// Allowed son los destinos admitidos desde el estado vigente; sus valores concretos se
			// verifican en TestUS2_TransicionesAdmitidas.
			if want := AllowedTransitions(c.current); !slices.Equal(transitionErr.Allowed, want) {
				t.Errorf("Allowed = %v, se esperaba %v", transitionErr.Allowed, want)
			}
		})
	}
}

// Escenario: Un defecto Resuelto solo admite reabrirse a Abierto, RN-15 (007/US5)
func TestUS5_TransicionAdmitidaDesdeResuelto(t *testing.T) {
	want := []DefectStatus{DefectStatusOpen}
	if got := AllowedTransitions(DefectStatusResolved); !slices.Equal(got, want) {
		t.Errorf("AllowedTransitions(%q) = %v, se esperaba %v", DefectStatusResolved, got, want)
	}
}

// Escenario: Reabrir un defecto Resuelto lo devuelve a Abierto y borra el sprint de resolución, aunque no haya sprint Activo, RN-15 (007/US5)
func TestUS5_ReabrirDefectoResuelto(t *testing.T) {
	cases := []struct {
		name     string
		detected SprintAnchor
	}{
		{"sprint de detección activo", newAnchor(sprintsdomain.SprintStatusActive, detectionStart)},
		{"único sprint del proyecto, cerrado", newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Reabrir no recibe sprint de resolución y el resultado nil indica que se borra.
			got, err := PlanTransition(DefectStatusResolved, DefectStatusOpen, c.detected, nil)
			if err != nil {
				t.Fatalf("PlanTransition devolvió el error %v, se esperaba nil", err)
			}
			if got != nil {
				t.Errorf("PlanTransition devolvió el sprint %s, se esperaba nil", *got)
			}
		})
	}
}

// Escenario: Solo se reabre un defecto Resuelto; Abierto y En progreso se rechazan con el estado vigente y las permitidas, RN-15 (007/US5)
func TestUS5_ReabrirDefectoNoResuelto(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)
	cases := []struct {
		name    string
		current DefectStatus
	}{
		{"open a open", DefectStatusOpen},
		{"in_progress a open", DefectStatusInProgress},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := PlanTransition(c.current, DefectStatusOpen, detected, nil)
			var transitionErr *InvalidTransitionError
			if !errors.As(err, &transitionErr) {
				t.Fatalf("PlanTransition(%q, open) devolvió el error %v, se esperaba *InvalidTransitionError", c.current, err)
			}
			if got != nil {
				t.Errorf("PlanTransition devolvió el sprint %s junto con el error, se esperaba nil", *got)
			}
			if transitionErr.Current != c.current || transitionErr.Requested != DefectStatusOpen {
				t.Errorf("error con Current=%q y Requested=%q, se esperaba %q y open", transitionErr.Current, transitionErr.Requested, c.current)
			}
			if want := AllowedTransitions(c.current); !slices.Equal(transitionErr.Allowed, want) {
				t.Errorf("Allowed = %v, se esperaba %v", transitionErr.Allowed, want)
			}
		})
	}
}

// Escenario: Un defecto reabierto se vuelve a resolver cuantas veces haga falta y vale la última resolución, RN-17 (007/US5)
func TestUS5_ResolverDeNuevoTrasReabrir(t *testing.T) {
	detected := newAnchor(sprintsdomain.SprintStatusClosed, detectionStart)

	// Un defecto reabierto vuelve a pasar por En progreso sin sprint de resolución.
	t.Run("open a in_progress tras reabrir", func(t *testing.T) {
		got, err := PlanTransition(DefectStatusOpen, DefectStatusInProgress, detected, nil)
		if err != nil || got != nil {
			t.Fatalf("PlanTransition devolvió (%v, %v), se esperaba (nil, nil)", got, err)
		}
	})

	// La segunda resolución en un sprint posterior se acepta y registra el sprint nuevo.
	t.Run("segunda resolución en sprint posterior", func(t *testing.T) {
		second := newAnchor(sprintsdomain.SprintStatusActive, sprintLaterStart)
		got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &second)
		assertResolvedSprint(t, got, err, second.ID)
	})

	// La regla compara contra el sprint de detección, que no cambió: resolver de nuevo en el
	// mismo sprint de la primera resolución se acepta.
	t.Run("segunda resolución en el mismo sprint que la primera", func(t *testing.T) {
		first := newAnchor(sprintsdomain.SprintStatusClosed, sprintAfterStart)
		again := first
		got, err := PlanTransition(DefectStatusInProgress, DefectStatusResolved, detected, &again)
		assertResolvedSprint(t, got, err, first.ID)
	})
}

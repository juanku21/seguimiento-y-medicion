package domain

import (
	"errors"
	"testing"

	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Escenario: Sprint en estado Planificado usado como sprint de detección se rechaza; Activo y Cerrado se aceptan, RN-08 (007/US1)
func TestUS1_SprintDeteccionUsable(t *testing.T) {
	cases := []struct {
		name   string
		status sprintsdomain.SprintStatus
		want   error
	}{
		{"planned se rechaza", sprintsdomain.SprintStatusPlanned, ErrDetectionSprintPlanned},
		{"active se acepta", sprintsdomain.SprintStatusActive, nil},
		{"closed se acepta", sprintsdomain.SprintStatusClosed, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnsureSprintUsableForDetection(c.status)
			if !errors.Is(got, c.want) {
				t.Errorf("EnsureSprintUsableForDetection(%q) = %v, se esperaba %v", c.status, got, c.want)
			}
		})
	}
}

// Escenario: Defecto Descartado no admite la modificación de ninguno de sus datos; Abierto, En progreso y Resuelto sí, RN-18 y RN-22 (007/US4)
func TestUS4_DefectoEditable(t *testing.T) {
	cases := []struct {
		name   string
		status DefectStatus
		want   error
	}{
		{"open se acepta", DefectStatusOpen, nil},
		{"in_progress se acepta", DefectStatusInProgress, nil},
		{"resolved se acepta", DefectStatusResolved, nil},
		{"discarded se rechaza", DefectStatusDiscarded, ErrDefectDiscarded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnsureDefectEditable(c.status)
			if !errors.Is(got, c.want) {
				t.Errorf("EnsureDefectEditable(%q) = %v, se esperaba %v", c.status, got, c.want)
			}
		})
	}
}

// Escenario: Historia relacionada y sprint de detección solo se corrigen con el defecto Abierto; En progreso, Resuelto y Descartado los congelan, RN-16 (007/US4)
func TestUS4_VinculosCorregibles(t *testing.T) {
	cases := []struct {
		name   string
		status DefectStatus
		want   error
	}{
		{"open se acepta", DefectStatusOpen, nil},
		{"in_progress se rechaza", DefectStatusInProgress, ErrLinksFrozen},
		{"resolved se rechaza", DefectStatusResolved, ErrLinksFrozen},
		{"discarded se rechaza", DefectStatusDiscarded, ErrLinksFrozen},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnsureLinksCorrectable(c.status)
			if !errors.Is(got, c.want) {
				t.Errorf("EnsureLinksCorrectable(%q) = %v, se esperaba %v", c.status, got, c.want)
			}
		})
	}
}

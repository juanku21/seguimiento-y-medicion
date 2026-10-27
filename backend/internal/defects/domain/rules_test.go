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

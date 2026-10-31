package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Escenario: Un sprint planificado no se puede reportar (010/US1)
func TestUS1_EnsureReportable(t *testing.T) {
	tests := []struct {
		name    string
		status  sprintsdomain.SprintStatus
		wantErr error
	}{
		{"planificado se rechaza", sprintsdomain.SprintStatusPlanned, ErrSprintNotStarted},
		{"activo se acepta", sprintsdomain.SprintStatusActive, nil},
		{"cerrado se acepta", sprintsdomain.SprintStatusClosed, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureReportable(tt.status)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// Escenario: El reporte de un sprint activo es parcial (010/US1)
func TestUS1_IsPartial(t *testing.T) {
	tests := []struct {
		name   string
		status sprintsdomain.SprintStatus
		want   bool
	}{
		{"sprint activo es parcial", sprintsdomain.SprintStatusActive, true},
		{"sprint cerrado no es parcial", sprintsdomain.SprintStatusClosed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPartial(ScopeSprint, []SprintRef{{ID: uuid.New(), Status: tt.status}})
			assert.Equal(t, tt.want, got)
		})
	}
}

// storyLine arma una historia de prueba con identificador, título y prioridad.
func storyLine(id string, title string, priority backlogdomain.StoryPriority) StoryLine {
	return StoryLine{StoryID: uuid.MustParse(id), Title: title, Priority: priority}
}

// Escenario: Las historias del reporte salen en un orden estable (010/US1)
func TestUS1_SortStories(t *testing.T) {
	const (
		id1 = "00000000-0000-0000-0000-000000000001"
		id2 = "00000000-0000-0000-0000-000000000002"
		id3 = "00000000-0000-0000-0000-000000000003"
		id4 = "00000000-0000-0000-0000-000000000004"
	)
	high := backlogdomain.StoryPriorityHigh
	medium := backlogdomain.StoryPriorityMedium
	low := backlogdomain.StoryPriorityLow

	// Orden esperado: prioridad, después título y después story_id (id1 e id4 empatan en título).
	want := []StoryLine{
		storyLine(id3, "Zeta", high),
		storyLine(id2, "Alfa", medium),
		storyLine(id1, "Beta", medium),
		storyLine(id4, "Beta", medium),
		storyLine(id1, "Alfa", low),
	}

	tests := []struct {
		name  string
		input []StoryLine
	}{
		{"prioridades mezcladas y empate de título", []StoryLine{want[4], want[3], want[0], want[2], want[1]}},
		{"entrada en otro orden", []StoryLine{want[2], want[4], want[1], want[3], want[0]}},
		{"lista ya ordenada", append([]StoryLine{}, want...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortStories(tt.input)
			assert.Equal(t, want, tt.input)
		})
	}
}

// Escenario: Un sprint cerrado muestra el título congelado de la historia (010/US1)
func TestUS1_StoryTitle(t *testing.T) {
	frozen := "Título congelado"
	tests := []struct {
		name     string
		snapshot *string
		current  string
		want     string
	}{
		{"en la instantánea devuelve el congelado", &frozen, "Título vigente", "Título congelado"},
		{"fuera de la instantánea devuelve el vigente", nil, "Título vigente", "Título vigente"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StoryTitle(tt.snapshot, tt.current))
		})
	}
}

// Escenario: Una sección sin historias se muestra vacía (010/US1)
func TestUS1_StoriesState(t *testing.T) {
	sprint := &SprintRef{ID: uuid.New()}
	tests := []struct {
		name   string
		groups []StoryGroup
		want   SectionState
	}{
		{"todos los grupos vacíos", []StoryGroup{{Sprint: sprint}, {Sprint: nil}}, SectionEmpty},
		{"un grupo con historias", []StoryGroup{
			{Sprint: sprint, Stories: []StoryLine{{StoryID: uuid.New()}}},
			{Sprint: nil},
		}, SectionComplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StoriesState(tt.groups))
		})
	}
}

// sprintRef arma un sprint de prueba con identificador, estado y fechas a medianoche UTC.
func sprintRef(id string, status sprintsdomain.SprintStatus, start, end string) SprintRef {
	s, _ := time.Parse(time.DateOnly, start)
	e, _ := time.Parse(time.DateOnly, end)
	return SprintRef{ID: uuid.MustParse(id), Status: status, StartDate: s, EndDate: e}
}

// Escenario: El reporte de proyecto abarca solo sprints activos y cerrados, en orden cronológico (010/US3)
func TestUS3_ReportableSprints(t *testing.T) {
	const (
		id1 = "00000000-0000-0000-0000-000000000001"
		id2 = "00000000-0000-0000-0000-000000000002"
		id3 = "00000000-0000-0000-0000-000000000003"
		id4 = "00000000-0000-0000-0000-000000000004"
		id5 = "00000000-0000-0000-0000-000000000005"
	)
	closed1 := sprintRef(id1, sprintsdomain.SprintStatusClosed, "2026-01-01", "2026-01-14")
	planned := sprintRef(id2, sprintsdomain.SprintStatusPlanned, "2026-01-15", "2026-01-28")
	closed2 := sprintRef(id3, sprintsdomain.SprintStatusClosed, "2026-02-01", "2026-02-14")
	plannedLate := sprintRef(id4, sprintsdomain.SprintStatusPlanned, "2026-02-15", "2026-02-28")
	active := sprintRef(id5, sprintsdomain.SprintStatusActive, "2026-03-01", "2026-03-14")

	tests := []struct {
		name  string
		input []SprintRef
		want  []SprintRef
	}{
		{"planificados intercalados se excluyen y el orden se conserva",
			[]SprintRef{closed1, planned, closed2, plannedLate, active},
			[]SprintRef{closed1, closed2, active}},
		{"solo planificados no deja ninguno", []SprintRef{planned, plannedLate}, []SprintRef{}},
		{"el orden recibido no se reordena", []SprintRef{active, closed1}, []SprintRef{active, closed1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReportableSprints(tt.input)
			assert.Equal(t, len(tt.want), len(got))
			if len(tt.want) > 0 {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// Escenario: El período del reporte de proyecto cubre desde el primer inicio hasta el último fin (010/US3)
func TestUS3_ReportPeriod(t *testing.T) {
	a := sprintRef("00000000-0000-0000-0000-000000000001", sprintsdomain.SprintStatusClosed, "2026-01-01", "2026-01-14")
	b := sprintRef("00000000-0000-0000-0000-000000000002", sprintsdomain.SprintStatusClosed, "2026-02-01", "2026-02-14")
	c := sprintRef("00000000-0000-0000-0000-000000000003", sprintsdomain.SprintStatusActive, "2026-03-01", "2026-03-14")

	t.Run("tres sprints devuelven menor inicio y mayor fin", func(t *testing.T) {
		got := ReportPeriod([]SprintRef{b, c, a})
		if assert.NotNil(t, got) {
			assert.Equal(t, a.StartDate, got.StartDate)
			assert.Equal(t, c.EndDate, got.EndDate)
		}
	})
	t.Run("un sprint devuelve sus fechas", func(t *testing.T) {
		got := ReportPeriod([]SprintRef{b})
		if assert.NotNil(t, got) {
			assert.Equal(t, b.StartDate, got.StartDate)
			assert.Equal(t, b.EndDate, got.EndDate)
		}
	})
	t.Run("sin sprints devuelve nil", func(t *testing.T) {
		assert.Nil(t, ReportPeriod(nil))
	})
}

// Escenario: El reporte de proyecto es parcial solo si incluye al sprint activo (010/US3)
func TestUS3_IsPartialProject(t *testing.T) {
	closed := sprintRef("00000000-0000-0000-0000-000000000001", sprintsdomain.SprintStatusClosed, "2026-01-01", "2026-01-14")
	active := sprintRef("00000000-0000-0000-0000-000000000002", sprintsdomain.SprintStatusActive, "2026-02-01", "2026-02-14")

	t.Run("con un sprint activo es parcial", func(t *testing.T) {
		assert.True(t, IsPartial(ScopeProject, []SprintRef{closed, active}))
	})
	t.Run("solo con sprints cerrados no es parcial", func(t *testing.T) {
		assert.False(t, IsPartial(ScopeProject, []SprintRef{closed}))
	})
}

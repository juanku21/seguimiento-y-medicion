package domain

import (
	"testing"

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

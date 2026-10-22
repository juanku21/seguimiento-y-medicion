package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// Pruebas unitarias obligatorias de Admission (T025; RN-08, SC-007; data-model.md sección 6 y
// research.md sección 8). No llevan la etiqueta de un escenario de la spec: los escenarios US2-7,
// US2-8, US2-9 y US2-11 se cubren con las pruebas de integración de la historia (T028).

// points devuelve un puntero a los Story Points, para expresar una historia estimada.
func points(value int) *int {
	return &value
}

// readyCandidate construye una historia lista para planificar y Pendiente, que cada caso modifica
// para provocar el motivo que quiere verificar.
func readyCandidate() AdmissionCandidate {
	return AdmissionCandidate{
		ID:            uuid.New(),
		Status:        backlogdomain.StoryStatusPending,
		StoryPoints:   points(5),
		CriteriaCount: 2,
	}
}

// reasonsOf extrae los motivos de los problemas, para comparar qué condiciones se informaron.
func reasonsOf(problems []AdmissionProblem) []AdmissionReason {
	reasons := make([]AdmissionReason, 0, len(problems))
	for _, problem := range problems {
		reasons = append(reasons, problem.Reason)
	}
	return reasons
}

// findProblem devuelve el problema con el motivo pedido o falla la prueba si no está.
func findProblem(t *testing.T, problems []AdmissionProblem, reason AdmissionReason) AdmissionProblem {
	t.Helper()
	for _, problem := range problems {
		if problem.Reason == reason {
			return problem
		}
	}
	require.Failf(t, "falta el motivo", "se esperaba el motivo %q en %v", reason, reasonsOf(problems))
	return AdmissionProblem{}
}

// Una historia lista para planificar (incluida la estimada en 0), Pendiente o En progreso y sin
// compromiso abierto se admite: Admission no informa ningún problema.
func TestAdmission_HistoriaAdmisible(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*AdmissionCandidate)
	}{
		{"lista y pendiente", func(*AdmissionCandidate) {}},
		{"estimada en cero", func(c *AdmissionCandidate) { c.StoryPoints = points(0) }},
		{"en progreso", func(c *AdmissionCandidate) { c.Status = backlogdomain.StoryStatusInProgress }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			story := readyCandidate()
			tc.modify(&story)

			problems := Admission(story, nil)

			assert.Empty(t, problems)
		})
	}
}

// Una historia sin Story Points, sin criterios o sin ninguno de los dos se rechaza con un único
// problema not_ready cuyo Missing es exactamente el que informa Readiness de la 003, sin sprint.
func TestAdmission_NoLista(t *testing.T) {
	cases := []struct {
		name        string
		storyPoints *int
		criteria    int
		wantMissing []string
	}{
		{"sin story points", nil, 2, []string{"story_points"}},
		{"sin criterios", points(3), 0, []string{"acceptance_criteria"}},
		{"sin ninguno de los dos", nil, 0, []string{"story_points", "acceptance_criteria"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			story := readyCandidate()
			story.StoryPoints = tc.storyPoints
			story.CriteriaCount = tc.criteria

			problems := Admission(story, nil)

			require.Len(t, problems, 1)
			problem := problems[0]
			assert.Equal(t, story.ID, problem.StoryID)
			assert.Equal(t, AdmissionReasonNotReady, problem.Reason)
			assert.Equal(t, tc.wantMissing, problem.Missing)
			_, readinessMissing := backlogdomain.Readiness(tc.storyPoints, tc.criteria)
			assert.Equal(t, readinessMissing, problem.Missing, "Missing debe coincidir con Readiness de la 003")
			assert.Nil(t, problem.Sprint)
		})
	}
}

// Una historia Completada, lista y sin compromiso abierto se rechaza con un único problema
// completed, sin Missing ni sprint.
func TestAdmission_Completada(t *testing.T) {
	story := readyCandidate()
	story.Status = backlogdomain.StoryStatusCompleted

	problems := Admission(story, nil)

	require.Len(t, problems, 1)
	assert.Equal(t, story.ID, problems[0].StoryID)
	assert.Equal(t, AdmissionReasonCompleted, problems[0].Reason)
	assert.Empty(t, problems[0].Missing)
	assert.Nil(t, problems[0].Sprint)
}

// Una historia lista con un compromiso abierto se rechaza con un único problema committed_elsewhere
// que indica ese sprint, tanto si es otro sprint como si es el mismo al que se intenta asignar
// (research.md sección 8): Admission no distingue el caso, solo informa el sprint recibido.
func TestAdmission_ComprometidaEnOtroSprint(t *testing.T) {
	cases := []struct {
		name   string
		sprint ConflictingSprint
	}{
		{"en otro sprint abierto", ConflictingSprint{ID: uuid.New(), Name: "Sprint 1"}},
		{"en este mismo sprint", ConflictingSprint{ID: uuid.New(), Name: "Sprint 2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			story := readyCandidate()
			open := tc.sprint

			problems := Admission(story, &open)

			require.Len(t, problems, 1)
			assert.Equal(t, story.ID, problems[0].StoryID)
			assert.Equal(t, AdmissionReasonCommittedElsewhere, problems[0].Reason)
			assert.Empty(t, problems[0].Missing)
			require.NotNil(t, problems[0].Sprint)
			assert.Equal(t, tc.sprint, *problems[0].Sprint)
		})
	}
}

// Una historia que incumple varias condiciones informa todos sus motivos, cada uno con sus datos:
// Missing solo en not_ready y Sprint solo en committed_elsewhere. El orden no se fija porque la spec
// no lo define.
func TestAdmission_VariosMotivos(t *testing.T) {
	conflict := ConflictingSprint{ID: uuid.New(), Name: "Sprint 1"}
	cases := []struct {
		name        string
		storyPoints *int
		criteria    int
		status      backlogdomain.StoryStatus
		open        *ConflictingSprint
		wantReasons []AdmissionReason
		wantMissing []string
	}{
		{
			name: "no lista y completada", storyPoints: nil, criteria: 2,
			status:      backlogdomain.StoryStatusCompleted,
			wantReasons: []AdmissionReason{AdmissionReasonNotReady, AdmissionReasonCompleted},
			wantMissing: []string{"story_points"},
		},
		{
			name: "no lista y comprometida", storyPoints: points(3), criteria: 0,
			status: backlogdomain.StoryStatusPending, open: &conflict,
			wantReasons: []AdmissionReason{AdmissionReasonNotReady, AdmissionReasonCommittedElsewhere},
			wantMissing: []string{"acceptance_criteria"},
		},
		{
			name: "completada y comprometida", storyPoints: points(3), criteria: 1,
			status: backlogdomain.StoryStatusCompleted, open: &conflict,
			wantReasons: []AdmissionReason{AdmissionReasonCompleted, AdmissionReasonCommittedElsewhere},
		},
		{
			name: "las tres condiciones", storyPoints: nil, criteria: 0,
			status: backlogdomain.StoryStatusCompleted, open: &conflict,
			wantReasons: []AdmissionReason{AdmissionReasonNotReady, AdmissionReasonCompleted, AdmissionReasonCommittedElsewhere},
			wantMissing: []string{"story_points", "acceptance_criteria"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			story := readyCandidate()
			story.StoryPoints = tc.storyPoints
			story.CriteriaCount = tc.criteria
			story.Status = tc.status

			problems := Admission(story, tc.open)

			assert.ElementsMatch(t, tc.wantReasons, reasonsOf(problems))
			for _, problem := range problems {
				assert.Equal(t, story.ID, problem.StoryID)
			}
			for _, reason := range tc.wantReasons {
				problem := findProblem(t, problems, reason)
				switch reason {
				case AdmissionReasonNotReady:
					assert.Equal(t, tc.wantMissing, problem.Missing)
					assert.Nil(t, problem.Sprint)
				case AdmissionReasonCompleted:
					assert.Empty(t, problem.Missing)
					assert.Nil(t, problem.Sprint)
				case AdmissionReasonCommittedElsewhere:
					assert.Empty(t, problem.Missing)
					require.NotNil(t, problem.Sprint)
					assert.Equal(t, conflict, *problem.Sprint)
				}
			}
		})
	}
}

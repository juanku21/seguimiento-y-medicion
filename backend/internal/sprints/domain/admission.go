package domain

import (
	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// AdmissionCandidate es la historia que se evalúa para comprometerla en un sprint, con los datos
// que AssignStories relee bajo bloqueo: el estado, los Story Points (nil significa "sin estimar") y
// la cantidad de criterios de aceptación. No es un modelo persistido.
type AdmissionCandidate struct {
	ID            uuid.UUID
	Status        backlogdomain.StoryStatus
	StoryPoints   *int
	CriteriaCount int
}

// Admission evalúa las tres condiciones de RN-08 sobre una historia y devuelve un problema por cada
// condición incumplida: not_ready (con lo que informa Readiness de la 003), completed y
// committed_elsewhere (con el sprint del compromiso abierto, que es nil si no lo hay). Una lista
// vacía significa que la historia se admite (research.md sección 8).
func Admission(story AdmissionCandidate, openCommitment *ConflictingSprint) []AdmissionProblem {
	// Cada condición se evalúa por separado y acumula su propio problema; la de "lista para
	// planificar" delega en Readiness de la 003 para no duplicar la regla.
	var problems []AdmissionProblem
	if ready, missing := backlogdomain.Readiness(story.StoryPoints, story.CriteriaCount); !ready {
		problems = append(problems, AdmissionProblem{StoryID: story.ID, Reason: AdmissionReasonNotReady, Missing: missing})
	}
	if story.Status == backlogdomain.StoryStatusCompleted {
		problems = append(problems, AdmissionProblem{StoryID: story.ID, Reason: AdmissionReasonCompleted})
	}
	if openCommitment != nil {
		sprint := *openCommitment
		problems = append(problems, AdmissionProblem{StoryID: story.ID, Reason: AdmissionReasonCommittedElsewhere, Sprint: &sprint})
	}
	return problems
}

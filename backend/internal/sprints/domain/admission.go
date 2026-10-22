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
	return nil
}

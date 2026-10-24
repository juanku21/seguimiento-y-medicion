package domain

import (
	"time"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// InactivityTimeout es el tiempo sin hechos a partir del cual el sistema cancela una sesión abierta;
// el límite es inclusivo (RN-31, SC-021).
const InactivityTimeout = 60 * time.Minute

// AutoCancelReason decide si una sesión abierta debe cancelarla el sistema y con qué motivo, según
// el estado de su historia, si su proyecto está finalizado y el tiempo desde su último hecho (FR-061,
// FR-063, RN-30, RN-31). Devuelve el motivo y verdadero si corresponde cancelar. Las condiciones se
// evalúan en el orden de precedencia de data-model.md sección 6: historia completada, proyecto
// finalizado e inactividad.
func AutoCancelReason(storyStatus backlogdomain.StoryStatus, projectFinished bool, lastActivityAt, now time.Time) (CancelReason, bool) {
	switch {
	case storyStatus == backlogdomain.StoryStatusCompleted:
		return CancelReasonStoryCompleted, true
	case projectFinished:
		return CancelReasonProjectFinished, true
	case now.Sub(lastActivityAt) >= InactivityTimeout:
		return CancelReasonInactivity, true
	default:
		return "", false
	}
}

package domain

import (
	"time"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// AutoCancelReason decide si una sesión abierta debe cancelarla el sistema y con qué motivo, según
// el estado de su historia, si su proyecto está finalizado y el tiempo desde su último hecho (FR-061,
// FR-063, RN-30, RN-31). Devuelve el motivo y verdadero si corresponde cancelar. Firma mínima de
// T078: la regla la implementa T081.
func AutoCancelReason(storyStatus backlogdomain.StoryStatus, projectFinished bool, lastActivityAt, now time.Time) (CancelReason, bool) {
	return "", false
}

package domain

import (
	"time"

	"github.com/google/uuid"
)

// FacilitatorTakeoverDelay es cuánto tiempo debe llevar desconectado el facilitador vigente para que
// otro participante pueda tomar su rol (FR-068, RN-25).
const FacilitatorTakeoverDelay = 5 * time.Minute

// FacilitatorTakeover decide si requesterID puede tomar el rol de facilitador en el instante now,
// según la presencia del facilitador vigente (FR-068, FR-069, RN-25). Quien ya es el facilitador, o
// cualquiera mientras el facilitador está conectado, recibe FacilitatorUnavailableError sin espera
// (RN-26, RN-35); si el facilitador lleva desconectado menos de FacilitatorTakeoverDelay, el error
// indica en RetryAfter cuánto falta. La desconexión se mide desde DisconnectedAt; un facilitador
// desconectado sin ese momento se trata como presente, porque no se puede medir la espera.
func FacilitatorTakeover(now time.Time, facilitator SnapshotParticipant, requesterID uuid.UUID) error {
	present := facilitator.Connected || facilitator.DisconnectedAt == nil
	if requesterID == facilitator.User.ID || present {
		return &FacilitatorUnavailableError{Facilitator: facilitator.User}
	}

	// El rol queda disponible al cumplirse el plazo exacto; antes se informa la espera restante.
	availableAt := facilitator.DisconnectedAt.Add(FacilitatorTakeoverDelay)
	if now.Before(availableAt) {
		return &FacilitatorUnavailableError{Facilitator: facilitator.User, RetryAfter: availableAt.Sub(now)}
	}
	return nil
}

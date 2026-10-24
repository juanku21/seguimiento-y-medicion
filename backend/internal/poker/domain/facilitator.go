package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// FacilitatorTakeover decide si requesterID puede tomar el rol de facilitador en el instante now,
// según la presencia del facilitador vigente (FR-068, FR-069, RN-25). Firma mínima de T070: la regla
// se implementa en T074.
func FacilitatorTakeover(now time.Time, facilitator SnapshotParticipant, requesterID uuid.UUID) error {
	return errors.New("not implemented")
}

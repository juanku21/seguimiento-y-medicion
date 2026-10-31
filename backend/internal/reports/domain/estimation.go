package domain

import (
	"time"

	pokerdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// SelectEstimation clasifica el estado de Planning Poker de una historia y elige la sesión
// finalizada vigente al corte (RN-09, RN-10, FR-013 a FR-015). Con cutoff nulo todas las sesiones
// cuentan; con corte, cuenta la creada hasta ese instante y solo aporta valor la finalizada hasta él.
func SelectEstimation(sessions []SessionRecord, cutoff *time.Time) (PokerState, *PokerSummary) {
	hasSession := false
	var chosen *SessionRecord

	for i := range sessions {
		s := &sessions[i]

		// Una sesión creada después del corte no cuenta para el reporte.
		if cutoff != nil && s.CreatedAt.After(*cutoff) {
			continue
		}
		hasSession = true

		// Solo aporta valor la finalizada con cierre y valor acordado, dentro del corte.
		if s.Status != pokerdomain.SessionStatusFinished || s.ClosedAt == nil || s.AgreedStoryPoints == nil {
			continue
		}
		if cutoff != nil && s.ClosedAt.After(*cutoff) {
			continue
		}

		// Gana la de mayor closed_at y, ante empate, la de mayor id (RN-09).
		if chosen == nil || s.ClosedAt.After(*chosen.ClosedAt) ||
			(s.ClosedAt.Equal(*chosen.ClosedAt) && s.ID.String() > chosen.ID.String()) {
			chosen = s
		}
	}

	if chosen != nil {
		return PokerAgreed, &PokerSummary{
			Rounds:            chosen.Rounds,
			AgreedStoryPoints: *chosen.AgreedStoryPoints,
			FinishedAt:        *chosen.ClosedAt,
		}
	}
	if hasSession {
		return PokerNoAgreement, nil
	}
	return PokerNone, nil
}

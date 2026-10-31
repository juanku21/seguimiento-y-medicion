package domain

import "time"

// SelectEstimation clasifica el estado de Planning Poker de una historia y elige la sesión
// finalizada vigente al corte (RN-09, RN-10, FR-013 a FR-015). Firma mínima: la lógica llega en T016.
func SelectEstimation(sessions []SessionRecord, cutoff *time.Time) (PokerState, *PokerSummary) {
	return "", nil
}

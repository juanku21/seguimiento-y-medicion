package domain

import "github.com/google/uuid"

// ShouldAutoReveal indica si la ronda vigente debe revelarse sola: hay al menos un participante
// conectado y todos los conectados votaron (FR-030, RN-09, RN-12, RN-23, RN-34). Los desconectados
// no se cuentan y cualquier carta, incluida "?", cuenta como voto emitido.
func ShouldAutoReveal(participants []EstimationParticipant, votes []EstimationVote) bool {
	// Se indexan los autores de los votos para consultar en tiempo constante quién votó.
	voted := make(map[uuid.UUID]struct{}, len(votes))
	for _, vote := range votes {
		voted[vote.ParticipantID] = struct{}{}
	}

	// Basta un conectado sin voto para esperar; sin ningún conectado tampoco se revela.
	anyConnected := false
	for _, participant := range participants {
		if !participant.Connected {
			continue
		}
		anyConnected = true
		if _, ok := voted[participant.ID]; !ok {
			return false
		}
	}
	return anyConnected
}

// CanRevealManually verifica que la ronda vigente tenga al menos un voto para revelarla a mano o
// devuelve ErrNoVotesToReveal (FR-032, FR-033, RN-10). No mira la presencia: sirve el voto de un
// participante ya desconectado.
func CanRevealManually(votes []EstimationVote) error {
	if len(votes) == 0 {
		return ErrNoVotesToReveal
	}
	return nil
}

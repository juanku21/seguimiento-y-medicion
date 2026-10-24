package domain

import "errors"

// ShouldAutoReveal indica si la ronda vigente debe revelarse sola: hay al menos un participante
// conectado y todos los conectados votaron (FR-030, RN-09, RN-12, RN-23, RN-34). Firma mínima de
// T045; la regla se implementa en T050.
func ShouldAutoReveal(participants []EstimationParticipant, votes []EstimationVote) bool {
	return false
}

// CanRevealManually verifica que la ronda vigente tenga al menos un voto para revelarla a mano o
// devuelve ErrNoVotesToReveal (FR-032, FR-033, RN-10). Firma mínima de T045; la regla se implementa
// en T050.
func CanRevealManually(votes []EstimationVote) error {
	return errors.New("not implemented")
}

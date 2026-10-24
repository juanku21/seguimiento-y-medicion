package domain

// Summarize calcula el resultado de una ronda revelada a partir de sus votos con sus autores:
// consensus con su valor, divergence con el mínimo y el máximo y todos sus autores, o
// no_numeric_votes; los "?" no participan y no hay promedio ni mediana (FR-036 a FR-042, RN-12 a
// RN-14). Firma mínima de T046; la regla se implementa en T051.
func Summarize(votes []SessionVote) RoundOutcome {
	return RoundOutcome{}
}

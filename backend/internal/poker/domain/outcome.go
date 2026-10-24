package domain

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
)

// OutcomeExtreme es el mínimo o el máximo de una ronda divergente, con todos los autores de ese
// valor (FR-039).
type OutcomeExtreme struct {
	Value   int
	Authors []UserRef
}

// RoundOutcome es el resultado de una ronda revelada, derivado de sus votos y nunca persistido
// (data-model.md sección 8; research.md sección 5). Value solo se completa en consensus; Min y Max
// solo en divergence; no_numeric_votes no lleva ninguno. No hay promedio ni mediana (FR-042).
type RoundOutcome struct {
	Kind  OutcomeKind
	Value *int
	Min   *OutcomeExtreme
	Max   *OutcomeExtreme
}

// Summarize calcula el resultado de una ronda revelada a partir de sus votos con sus autores:
// consensus con su valor, divergence con el mínimo y el máximo y todos sus autores, o
// no_numeric_votes; los "?" no participan y no hay promedio ni mediana (FR-036 a FR-042, RN-12 a
// RN-14). Primero agrupa los autores por valor numérico, descartando las cartas que no son número
// (entre las cartas admitidas, solo "?"); luego decide el resultado según cuántos valores distintos
// quedaron.
func Summarize(votes []SessionVote) RoundOutcome {
	authorsByValue := map[int][]UserRef{}
	for _, v := range votes {
		points, err := strconv.Atoi(string(v.Card))
		if err != nil {
			continue
		}
		authorsByValue[points] = append(authorsByValue[points], v.User)
	}

	// Sin valores numéricos no hay resultado; con uno solo hay consenso; con más, divergencia entre
	// el menor y el mayor.
	values := slices.Sorted(maps.Keys(authorsByValue))
	switch len(values) {
	case 0:
		return RoundOutcome{Kind: OutcomeKindNoNumericVotes}
	case 1:
		return RoundOutcome{Kind: OutcomeKindConsensus, Value: &values[0]}
	}
	minValue, maxValue := values[0], values[len(values)-1]
	return RoundOutcome{
		Kind: OutcomeKindDivergence,
		Min:  extreme(minValue, authorsByValue[minValue]),
		Max:  extreme(maxValue, authorsByValue[maxValue]),
	}
}

// extreme arma un extremo de la divergencia con sus autores ordenados por nombre de forma
// ascendente, para que todos los participantes vean la misma lista sin importar el orden de los
// votos (SC-007).
func extreme(value int, authors []UserRef) *OutcomeExtreme {
	sorted := slices.SortedFunc(slices.Values(authors), func(a, b UserRef) int {
		return cmp.Compare(a.FullName, b.FullName)
	})
	return &OutcomeExtreme{Value: value, Authors: sorted}
}

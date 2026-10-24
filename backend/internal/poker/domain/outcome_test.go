package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de Summarize (T046, RN-12, RN-13, RN-14, SC-008, SC-009) con la
// tabla completa de data-model.md sección 6: consenso cuando todos los votos numéricos coinciden,
// divergencia con el mínimo y el máximo y todos sus autores ordenados por nombre, y sin votos
// numéricos cuando todos votaron "?". Los nombres llevan el prefijo "Dominio", convención de las
// pruebas de dominio del proyecto, para no chocar con las pruebas de integración de US3.

// author arma una persona con un ID nuevo y el nombre indicado, y castVote el voto revelado de esa
// persona con la carta indicada.
func author(name string) UserRef {
	return UserRef{ID: uuid.New(), FullName: name}
}

func castVote(user UserRef, card Card) SessionVote {
	return SessionVote{User: user, Card: card}
}

// requireConsensus verifica que el resultado sea consenso en el valor indicado, sin mínimo ni
// máximo (FR-036, FR-040).
func requireConsensus(t *testing.T, outcome RoundOutcome, want int) {
	t.Helper()
	assert.Equal(t, OutcomeKindConsensus, outcome.Kind)
	require.NotNil(t, outcome.Value, "el consenso debe informar su valor")
	assert.Equal(t, want, *outcome.Value)
	assert.Nil(t, outcome.Min, "el consenso no informa mínimo")
	assert.Nil(t, outcome.Max, "el consenso no informa máximo")
}

// requireDivergence verifica que el resultado sea divergencia, sin valor de consenso, con el mínimo
// y el máximo indicados y sus autores en el orden esperado (FR-037, FR-039).
func requireDivergence(t *testing.T, outcome RoundOutcome, wantMin OutcomeExtreme, wantMax OutcomeExtreme) {
	t.Helper()
	assert.Equal(t, OutcomeKindDivergence, outcome.Kind)
	assert.Nil(t, outcome.Value, "la divergencia no informa valor de consenso")
	require.NotNil(t, outcome.Min, "la divergencia debe informar el mínimo")
	require.NotNil(t, outcome.Max, "la divergencia debe informar el máximo")
	assert.Equal(t, wantMin, *outcome.Min)
	assert.Equal(t, wantMax, *outcome.Max)
}

// Escenario: Revelar los votos y ver las diferencias - Caso normal: revelación automática (005/US3)
// Con votos 3, 5 y 13 la ronda diverge y señala el mínimo 3 y el máximo 13 con sus autores; el voto
// intermedio no figura en ningún extremo (FR-037, FR-039, RN-14, SC-008).
func TestUS3_DominioResultadoDivergencia(t *testing.T) {
	ana, bruno, carla := author("Ana"), author("Bruno"), author("Carla")
	outcome := Summarize([]SessionVote{castVote(bruno, Card("5")), castVote(carla, Card("13")), castVote(ana, Card("3"))})
	requireDivergence(t, outcome,
		OutcomeExtreme{Value: 3, Authors: []UserRef{ana}},
		OutcomeExtreme{Value: 13, Authors: []UserRef{carla}},
	)
}

// Escenario: Revelar los votos y ver las diferencias - Caso límite: varios autores en los extremos (005/US3)
// Con votos 3, 3, 40 y 40 se señalan los dos autores del mínimo y los dos del máximo (RN-14).
func TestUS3_DominioResultadoVariosAutoresEnExtremos(t *testing.T) {
	ana, bruno, carla, diego := author("Ana"), author("Bruno"), author("Carla"), author("Diego")
	outcome := Summarize([]SessionVote{
		castVote(ana, Card("3")), castVote(carla, Card("40")), castVote(bruno, Card("3")), castVote(diego, Card("40")),
	})
	requireDivergence(t, outcome,
		OutcomeExtreme{Value: 3, Authors: []UserRef{ana, bruno}},
		OutcomeExtreme{Value: 40, Authors: []UserRef{carla, diego}},
	)
}

// Requisito de T046, no un escenario de la spec: los autores de cada extremo salen ordenados por
// nombre, sin importar el orden en que llegan los votos, para que todos los participantes vean la
// misma lista (SC-007).
func TestUS3_DominioResultadoAutoresOrdenadosPorNombre(t *testing.T) {
	ana, bruno, carla, diego := author("Ana"), author("Bruno"), author("Carla"), author("Diego")
	outcome := Summarize([]SessionVote{
		castVote(diego, Card("1")), castVote(bruno, Card("20")), castVote(carla, Card("1")),
		castVote(ana, Card("20")), castVote(author("Elena"), Card("8")),
	})
	requireDivergence(t, outcome,
		OutcomeExtreme{Value: 1, Authors: []UserRef{carla, diego}},
		OutcomeExtreme{Value: 20, Authors: []UserRef{ana, bruno}},
	)
}

// Escenario: Revelar los votos y ver las diferencias - Caso alternativo: votos iguales (005/US3)
// Con todos los votos numéricos iguales la ronda tiene consenso en ese valor y no señala mínimo ni
// máximo (FR-036, FR-040, RN-13, SC-008). Se verifica con la fila 8, 8, 8 de data-model.md y con el
// 20, 20, 20 del escenario de la spec.
func TestUS3_DominioResultadoVotosIguales(t *testing.T) {
	// Cada variante tiene tres votos con la misma carta.
	for _, tc := range []struct {
		name string
		card Card
		want int
	}{
		{name: "8, 8 y 8", card: Card("8"), want: 8},
		{name: "20, 20 y 20", card: Card("20"), want: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := Summarize([]SessionVote{
				castVote(author("Ana"), tc.card), castVote(author("Bruno"), tc.card), castVote(author("Carla"), tc.card),
			})
			requireConsensus(t, outcome, tc.want)
		})
	}
}

// Escenario: Revelar los votos y ver las diferencias - Caso alternativo: el "?" no participa del cálculo (005/US3)
// El "?" no participa del consenso (FR-038, RN-12): con 5, 5 y "?" hay consenso en 5, y con 13, "?"
// y "?" basta el único voto numérico para el consenso en 13.
func TestUS3_DominioResultadoInterrogacionNoParticipa(t *testing.T) {
	// Dos votos numéricos iguales y un "?".
	t.Run("5, 5 y ?", func(t *testing.T) {
		outcome := Summarize([]SessionVote{
			castVote(author("Ana"), Card("5")), castVote(author("Bruno"), CardUnknown), castVote(author("Carla"), Card("5")),
		})
		requireConsensus(t, outcome, 5)
	})

	// Un único voto numérico y dos "?".
	t.Run("13, ? y ?", func(t *testing.T) {
		outcome := Summarize([]SessionVote{
			castVote(author("Ana"), CardUnknown), castVote(author("Bruno"), Card("13")), castVote(author("Carla"), CardUnknown),
		})
		requireConsensus(t, outcome, 13)
	})
}

// Escenario: Revelar los votos y ver las diferencias - Caso límite: único participante (005/US3)
// Un único voto es consenso en su valor, incluso cuando es 0, que no debe confundirse con la
// ausencia de valor (FR-036, RN-13).
func TestUS3_DominioResultadoUnicoVoto(t *testing.T) {
	outcome := Summarize([]SessionVote{castVote(author("Ana"), Card("0"))})
	requireConsensus(t, outcome, 0)
}

// Escenario: Revelar los votos y ver las diferencias - Caso límite: todos votan "?" (005/US3)
// Si todos votaron "?", la ronda no tiene votos numéricos: no hay consenso ni mínimo ni máximo
// (FR-040, FR-041, RN-13, SC-009).
func TestUS3_DominioResultadoSinVotosNumericos(t *testing.T) {
	outcome := Summarize([]SessionVote{
		castVote(author("Ana"), CardUnknown), castVote(author("Bruno"), CardUnknown), castVote(author("Carla"), CardUnknown),
	})
	assert.Equal(t, OutcomeKindNoNumericVotes, outcome.Kind)
	assert.Nil(t, outcome.Value, "sin votos numéricos no hay consenso")
	assert.Nil(t, outcome.Min, "sin votos numéricos no hay mínimo")
	assert.Nil(t, outcome.Max, "sin votos numéricos no hay máximo")
}

// Requisito FR-042, no un escenario de la spec: el resultado y sus extremos solo tienen los campos
// del consenso, el mínimo y el máximo, así que no puede llevar promedio, mediana ni otra agregación.
func TestUS3_DominioResultadoSinPromedioNiMediana(t *testing.T) {
	// fieldNames lista los campos de un struct en el orden en que se declaran.
	fieldNames := func(value any) []string {
		typ := reflect.TypeOf(value)
		names := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			names = append(names, typ.Field(i).Name)
		}
		return names
	}
	assert.Equal(t, []string{"Kind", "Value", "Min", "Max"}, fieldNames(RoundOutcome{}))
	assert.Equal(t, []string{"Value", "Authors"}, fieldNames(OutcomeExtreme{}))
}

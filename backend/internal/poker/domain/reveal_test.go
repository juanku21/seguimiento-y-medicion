package domain

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Pruebas unitarias obligatorias de ShouldAutoReveal y CanRevealManually (T045, RN-09, RN-10, RN-12,
// RN-16, RN-23, SC-006): la ronda se revela sola cuando hay al menos un conectado y todos los
// conectados votaron, sin importar los desconectados; la revelación manual exige al menos un voto,
// sea de quien sea. Los nombres llevan el prefijo "Dominio", convención de las pruebas de dominio del
// proyecto, para no chocar con las pruebas de integración de US3.

// connectedParticipant y disconnectedParticipant arman la presencia de un integrante con un ID nuevo;
// a ShouldAutoReveal solo le importan el ID y Connected.
func connectedParticipant() EstimationParticipant {
	return EstimationParticipant{ID: uuid.New(), Connected: true}
}

func disconnectedParticipant() EstimationParticipant {
	return EstimationParticipant{ID: uuid.New(), Connected: false}
}

// voteOf arma el voto de un participante en la ronda vigente con la carta indicada.
func voteOf(participant EstimationParticipant, card Card) EstimationVote {
	return EstimationVote{ID: uuid.New(), ParticipantID: participant.ID, Card: card}
}

// Escenario: Revelar los votos y ver las diferencias - Caso normal: revelación automática (005/US3)
// Cuando todos los conectados votaron, la ronda se revela sola (FR-030, RN-09). Se verifica con 1, 2
// y 5 participantes, como exige SC-006, cada cantidad en su subtest.
func TestUS3_DominioRevelacionAutomatica(t *testing.T) {
	// Cada cantidad de conectados vota completa y la regla debe disparar la revelación.
	for _, count := range []int{1, 2, 5} {
		t.Run(fmt.Sprintf("%d conectados con todos sus votos", count), func(t *testing.T) {
			participants := make([]EstimationParticipant, 0, count)
			votes := make([]EstimationVote, 0, count)
			for i := 0; i < count; i++ {
				participant := connectedParticipant()
				participants = append(participants, participant)
				votes = append(votes, voteOf(participant, Card("5")))
			}
			assert.True(t, ShouldAutoReveal(participants, votes))
		})
	}
}

// Escenario: Revelar los votos y ver las diferencias - Caso límite: único participante (005/US3)
// Un único conectado que vota dispara la revelación automática, porque es todo el equipo conectado.
func TestUS3_DominioUnicoParticipante(t *testing.T) {
	alone := connectedParticipant()
	assert.True(t, ShouldAutoReveal([]EstimationParticipant{alone}, []EstimationVote{voteOf(alone, Card("8"))}))
}

// Contraparte de la regla RN-09, no un escenario de la spec: mientras un conectado no votó, la ronda
// no se revela sola, aunque los demás ya hayan votado.
func TestUS3_DominioConectadoSinVoto(t *testing.T) {
	first, second, pending := connectedParticipant(), connectedParticipant(), connectedParticipant()
	participants := []EstimationParticipant{first, second, pending}
	votes := []EstimationVote{voteOf(first, Card("3")), voteOf(second, Card("5"))}
	assert.False(t, ShouldAutoReveal(participants, votes))
}

// Escenario: Revelar los votos y ver las diferencias - Caso límite: desconexión dispara la revelación (005/US3)
// Un desconectado sin voto deja de contarse (RN-23): con los tres conectados restantes ya votados, la
// ronda se revela sola.
func TestUS3_DominioDesconexionDisparaRevelacion(t *testing.T) {
	a, b, c := connectedParticipant(), connectedParticipant(), connectedParticipant()
	gone := disconnectedParticipant()
	participants := []EstimationParticipant{a, b, c, gone}
	votes := []EstimationVote{voteOf(a, Card("3")), voteOf(b, Card("5")), voteOf(c, Card("13"))}
	assert.True(t, ShouldAutoReveal(participants, votes))
}

// Contraparte de las reglas RN-09 y RN-23, no un escenario de la spec: sin ningún conectado no hay
// revelación automática, aunque un desconectado haya dejado su voto.
func TestUS3_DominioNadieConectado(t *testing.T) {
	gone := disconnectedParticipant()
	participants := []EstimationParticipant{gone}
	votes := []EstimationVote{voteOf(gone, Card("5"))}
	assert.False(t, ShouldAutoReveal(participants, votes))
}

// Regla RN-12, no un escenario propio de la spec: la carta "?" cuenta como voto emitido para la
// revelación automática, tanto mezclada con números como cuando todos la eligen.
func TestUS3_DominioInterrogacionCuentaComoVoto(t *testing.T) {
	// Un "?" completa la ronda junto con votos numéricos.
	t.Run("mezclada con votos numéricos", func(t *testing.T) {
		a, b, unsure := connectedParticipant(), connectedParticipant(), connectedParticipant()
		participants := []EstimationParticipant{a, b, unsure}
		votes := []EstimationVote{voteOf(a, Card("5")), voteOf(b, Card("5")), voteOf(unsure, CardUnknown)}
		assert.True(t, ShouldAutoReveal(participants, votes))
	})

	// Tres "?" completan la ronda aunque no haya ningún voto numérico.
	t.Run("todos votaron ?", func(t *testing.T) {
		a, b, c := connectedParticipant(), connectedParticipant(), connectedParticipant()
		participants := []EstimationParticipant{a, b, c}
		votes := []EstimationVote{voteOf(a, CardUnknown), voteOf(b, CardUnknown), voteOf(c, CardUnknown)}
		assert.True(t, ShouldAutoReveal(participants, votes))
	})
}

// Regla RN-16, no un escenario propio de la spec: el facilitador vota como cualquier otro, así que
// su voto completa la ronda y, mientras no vota, la ronda espera por él. El facilitador no se marca
// en EstimationParticipant (es estimation_sessions.facilitator_id), por eso la regla no puede tratarlo
// distinto.
func TestUS3_DominioVotoDelFacilitador(t *testing.T) {
	facilitator, member := connectedParticipant(), connectedParticipant()
	participants := []EstimationParticipant{facilitator, member}

	// El voto del facilitador es el que completa la ronda.
	t.Run("su voto completa la ronda", func(t *testing.T) {
		votes := []EstimationVote{voteOf(member, Card("8")), voteOf(facilitator, Card("8"))}
		assert.True(t, ShouldAutoReveal(participants, votes))
	})

	// Sin el voto del facilitador conectado, la ronda no se revela sola.
	t.Run("sin su voto la ronda espera", func(t *testing.T) {
		votes := []EstimationVote{voteOf(member, Card("8"))}
		assert.False(t, ShouldAutoReveal(participants, votes))
	})
}

// Escenario: Revelar los votos y ver las diferencias - Caso de error: revelar sin votos (005/US3)
// Una ronda sin ningún voto emitido no se puede revelar a mano (FR-033, RN-10).
func TestUS3_DominioRevelarSinVotos(t *testing.T) {
	// La lista nil y la lista vacía representan igual una ronda sin votos.
	t.Run("lista nil", func(t *testing.T) {
		assert.ErrorIs(t, CanRevealManually(nil), ErrNoVotesToReveal)
	})
	t.Run("lista vacía", func(t *testing.T) {
		assert.ErrorIs(t, CanRevealManually([]EstimationVote{}), ErrNoVotesToReveal)
	})
}

// Contraparte de la regla RN-10, no un escenario de la spec: basta un voto emitido para revelar a
// mano, aunque su autor ya esté desconectado, porque la revelación manual mira los votos y no la
// presencia.
func TestUS3_DominioRevelarConVotoDeDesconectado(t *testing.T) {
	gone := disconnectedParticipant()
	assert.NoError(t, CanRevealManually([]EstimationVote{voteOf(gone, Card("5"))}))
}

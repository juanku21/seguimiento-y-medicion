package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pruebas unitarias obligatorias de ViewFor (T013) en la parte que cubre US1: la historia con su
// texto y sus criterios en orden (FR-012), las once cartas, los participantes con presencia y rol,
// las rondas con número, estado y apertura, y los datos de apertura y cierre copiados de la
// instantánea. Los votos se prueban en US2. Los nombres llevan el prefijo "Dominio", convención de
// las pruebas de dominio del proyecto, para no chocar con las pruebas de integración de US1.

// viewFixture reúne una instantánea de prueba con las personas que participan en ella, para que
// cada prueba compare la vista contra los datos de entrada sin repetir literales.
type viewFixture struct {
	snapshot SessionSnapshot
	alice    UserRef
	bob      UserRef
	carol    UserRef
	openedAt time.Time
	round1At time.Time
	round2At time.Time
}

// newOpenViewFixture arma una sesión abierta que abrió Alice y cuyo facilitador actual es Bob, con
// tres participantes (Carol desconectada), una historia con tres criterios y dos rondas: la 1
// revelada y la 2 en votación, ambas sin votos.
func newOpenViewFixture() viewFixture {
	alice := UserRef{ID: uuid.New(), FullName: "Alice Pérez"}
	bob := UserRef{ID: uuid.New(), FullName: "Bob Gómez"}
	carol := UserRef{ID: uuid.New(), FullName: "Carol Díaz"}
	openedAt := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	round1At := openedAt
	round2At := openedAt.Add(5 * time.Minute)
	revealedAt := openedAt.Add(3 * time.Minute)
	disconnectedAt := openedAt.Add(4 * time.Minute)
	description := "Como integrante quiero iniciar sesión para ver mis proyectos."
	storyID := uuid.New()

	snapshot := SessionSnapshot{
		Session: EstimationSession{
			ID:             uuid.New(),
			StoryID:        storyID,
			FacilitatorID:  bob.ID,
			Status:         SessionStatusOpen,
			OpenedByID:     alice.ID,
			LastActivityAt: round2At,
			Version:        7,
			CreatedAt:      openedAt,
			UpdatedAt:      round2At,
		},
		Story: SessionStory{
			ID:          storyID,
			ProjectID:   uuid.New(),
			Title:       "Inicio de sesión",
			Description: &description,
			AcceptanceCriteria: []string{
				"Zeta: el formulario pide correo y contraseña",
				"Alfa: las credenciales inválidas se rechazan",
				"Media: la sesión dura ocho horas",
			},
		},
		Facilitator: bob,
		OpenedBy:    alice,
		Participants: []SnapshotParticipant{
			{User: alice, Connected: true},
			{User: bob, Connected: true},
			{User: carol, Connected: false, DisconnectedAt: &disconnectedAt},
		},
		Rounds: []SnapshotRound{
			{Number: 1, Status: RoundStatusRevealed, OpenedAt: round1At, RevealedAt: &revealedAt},
			{Number: 2, Status: RoundStatusVoting, OpenedAt: round2At},
		},
	}

	return viewFixture{
		snapshot: snapshot,
		alice:    alice,
		bob:      bob,
		carol:    carol,
		openedAt: openedAt,
		round1At: round1At,
		round2At: round2At,
	}
}

// participantByID busca en la vista al participante con el identificador dado, para no atar las
// aserciones al orden de la lista.
func participantByID(t *testing.T, view SessionView, id uuid.UUID) ParticipantView {
	t.Helper()
	for _, p := range view.Participants {
		if p.User.ID == id {
			return p
		}
	}
	require.Failf(t, "participante ausente", "la vista no incluye al participante %s", id)
	return ParticipantView{}
}

// Escenario: Unirse a una sesión - Éxito (005/US1)
// Quien se une recibe la historia con título, descripción y criterios en su orden, la escala de
// cartas, los participantes con su presencia y quién facilita, y las rondas con número y estado
// (FR-012). Cada parte de la vista se verifica en su propio subtest.
func TestUS1_DominioUnirseConExito(t *testing.T) {
	// La historia llega con su texto y sus criterios en el orden de la instantánea, sin reordenar.
	t.Run("historia con titulo descripcion y criterios en orden", func(t *testing.T) {
		f := newOpenViewFixture()

		view := ViewFor(f.snapshot, f.alice.ID)

		assert.Equal(t, f.snapshot.Story.ID, view.Story.ID)
		assert.Equal(t, "Inicio de sesión", view.Story.Title)
		require.NotNil(t, view.Story.Description)
		assert.Equal(t, *f.snapshot.Story.Description, *view.Story.Description)
		assert.Equal(t, []string{
			"Zeta: el formulario pide correo y contraseña",
			"Alfa: las credenciales inválidas se rechazan",
			"Media: la sesión dura ocho horas",
		}, view.Story.AcceptanceCriteria)
	})

	// La escala de cartas es la lista completa de once cartas en su orden.
	t.Run("cartas iguales a las once de la escala", func(t *testing.T) {
		f := newOpenViewFixture()

		view := ViewFor(f.snapshot, f.alice.ID)

		require.Len(t, Cards, 11)
		assert.Equal(t, Cards, view.Cards)
	})

	// Cada participante conserva su presencia y solo el facilitador de la instantánea lo es en la
	// vista, aunque no sea quien abrió la sesión.
	t.Run("participantes con presencia y un unico facilitador", func(t *testing.T) {
		f := newOpenViewFixture()

		view := ViewFor(f.snapshot, f.carol.ID)

		require.Len(t, view.Participants, 3)
		facilitators := 0
		for _, p := range view.Participants {
			if p.IsFacilitator {
				facilitators++
			}
		}
		assert.Equal(t, 1, facilitators, "debe haber exactamente un facilitador")

		alice := participantByID(t, view, f.alice.ID)
		assert.Equal(t, f.alice, alice.User)
		assert.True(t, alice.Connected)
		assert.False(t, alice.IsFacilitator)

		bob := participantByID(t, view, f.bob.ID)
		assert.Equal(t, f.bob, bob.User)
		assert.True(t, bob.Connected)
		assert.True(t, bob.IsFacilitator)

		carol := participantByID(t, view, f.carol.ID)
		assert.Equal(t, f.carol, carol.User)
		assert.False(t, carol.Connected)
		assert.False(t, carol.IsFacilitator)
	})

	// Las rondas aparecen de la 1 a la vigente con su número, su estado y su momento de apertura.
	t.Run("rondas con numero estado y apertura", func(t *testing.T) {
		f := newOpenViewFixture()

		view := ViewFor(f.snapshot, f.alice.ID)

		require.Len(t, view.Rounds, 2)
		assert.Equal(t, 1, view.Rounds[0].Number)
		assert.Equal(t, RoundStatusRevealed, view.Rounds[0].Status)
		assert.True(t, f.round1At.Equal(view.Rounds[0].OpenedAt))
		assert.Equal(t, 2, view.Rounds[1].Number)
		assert.Equal(t, RoundStatusVoting, view.Rounds[1].Status)
		assert.True(t, f.round2At.Equal(view.Rounds[1].OpenedAt))
	})

	// Los datos de la sesión abierta se copian de la instantánea: identificador, versión, estado,
	// facilitador, quién abrió y cuándo; sin cierre todavía.
	t.Run("sesion abierta copia apertura y no tiene cierre", func(t *testing.T) {
		f := newOpenViewFixture()

		view := ViewFor(f.snapshot, f.alice.ID)

		assert.Equal(t, f.snapshot.Session.ID, view.ID)
		assert.Equal(t, int64(7), view.Version)
		assert.Equal(t, SessionStatusOpen, view.Status)
		assert.Equal(t, f.bob, view.Facilitator)
		assert.Equal(t, f.alice, view.OpenedBy)
		assert.True(t, f.openedAt.Equal(view.OpenedAt))
		assert.Nil(t, view.ClosedBy)
		assert.Nil(t, view.ClosedAt)
	})

	// En una sesión cerrada, quién cerró y cuándo se copian de la instantánea junto con la apertura.
	t.Run("sesion cerrada copia apertura y cierre", func(t *testing.T) {
		f := newOpenViewFixture()
		closedAt := f.round2At.Add(10 * time.Minute)
		f.snapshot.Session.Status = SessionStatusFinished
		f.snapshot.Session.AgreedStoryPoints = intPtr(8)
		f.snapshot.Session.ClosedByID = &f.bob.ID
		f.snapshot.Session.ClosedAt = &closedAt
		f.snapshot.ClosedBy = &f.bob

		view := ViewFor(f.snapshot, f.alice.ID)

		assert.Equal(t, SessionStatusFinished, view.Status)
		assert.Equal(t, f.alice, view.OpenedBy)
		assert.True(t, f.openedAt.Equal(view.OpenedAt))
		require.NotNil(t, view.ClosedBy)
		assert.Equal(t, f.bob, *view.ClosedBy)
		require.NotNil(t, view.ClosedAt)
		assert.True(t, closedAt.Equal(*view.ClosedAt))
	})
}

// Escenario: Historia sin criterios de aceptación (005/US1)
// Una historia sin criterios se muestra con la lista vacía, que la vista entrega como [] y no como
// nil para que el JSON lleve un arreglo (US1-4). Las variantes son criterios nil y criterios vacíos
// en la instantánea.
func TestUS1_DominioHistoriaSinCriterios(t *testing.T) {
	cases := []struct {
		name     string
		criteria []string
	}{
		{name: "criterios nil en la instantanea", criteria: nil},
		{name: "criterios vacios en la instantanea", criteria: []string{}},
	}

	// Cada forma de lista vacía se prueba por separado para identificar cuál falla.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOpenViewFixture()
			f.snapshot.Story.AcceptanceCriteria = tc.criteria

			view := ViewFor(f.snapshot, f.alice.ID)

			assert.Equal(t, "Inicio de sesión", view.Story.Title)
			assert.NotNil(t, view.Story.AcceptanceCriteria, "los criterios deben ser [] y no nil")
			assert.Empty(t, view.Story.AcceptanceCriteria)
		})
	}
}

// Pruebas unitarias obligatorias de ViewFor (T035) en la parte que cubre US2, el secreto del voto
// (RN-08, FR-023, FR-024, FR-082): mientras la ronda vigente está en votación nadie recibe sus votos,
// ni siquiera el autor; cada participante muestra solo si ya votó; MyVote lleva la carta de quien
// pide y nunca la de otro; y las rondas anteriores, ya reveladas, muestran sus votos. Los nombres
// llevan el infijo "Vista" para no chocar con las pruebas de ParseCard del mismo paquete.

// withVotes agrega a la ronda con el número dado los votos indicados, para armar cada escenario a
// partir de la instantánea base sin repetirla.
func (f *viewFixture) withVotes(number int, votes ...SessionVote) {
	for i := range f.snapshot.Rounds {
		if f.snapshot.Rounds[i].Number == number {
			f.snapshot.Rounds[i].Votes = append(f.snapshot.Rounds[i].Votes, votes...)
		}
	}
}

// cardPtr devuelve un puntero a la carta dada, para comparar contra MyVote.
func cardPtr(c Card) *Card { return &c }

// Escenario: Voto oculto (005/US2)
// Con la ronda vigente en votación y Alice con la carta 5, Bob y Carol ven que Alice ya votó, pero
// no reciben ningún voto de la ronda ni carta propia, porque no votaron. Cada espectador ajeno al
// voto se verifica en su propio subtest.
func TestUS2_DominioVistaVotoOculto(t *testing.T) {
	cases := []struct {
		name   string
		viewer func(f viewFixture) UserRef
	}{
		{name: "espectador conectado sin voto", viewer: func(f viewFixture) UserRef { return f.bob }},
		{name: "espectador desconectado sin voto", viewer: func(f viewFixture) UserRef { return f.carol }},
	}

	// Cada espectador recibe hasVoted solo para Alice, la ronda vigente sin votos y MyVote nil.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOpenViewFixture()
			f.withVotes(2, SessionVote{User: f.alice, Card: "5"})

			view := ViewFor(f.snapshot, tc.viewer(f).ID)

			require.Len(t, view.Rounds, 2)
			assert.Nil(t, view.Rounds[1].Votes, "la ronda en votación no expone votos")
			assert.True(t, participantByID(t, view, f.alice.ID).HasVoted, "Alice ya votó")
			assert.False(t, participantByID(t, view, f.bob.ID).HasVoted, "Bob no votó")
			assert.False(t, participantByID(t, view, f.carol.ID).HasVoted, "Carol no votó")
			assert.Nil(t, view.MyVote, "quien no votó no tiene carta propia")
		})
	}
}

// Escenario: Propia carta visible (005/US2)
// El autor de un voto ve su propia carta en MyVote, pero tampoco recibe los votos de la ronda en
// votación: su carta viaja solo en MyVote. Con dos autores, cada uno recibe la suya y nunca la del
// otro (FR-024, FR-082).
func TestUS2_DominioVistaPropiaCartaVisible(t *testing.T) {
	// El autor recibe su carta en MyVote, figura como "ya votó" y la ronda vigente no lleva votos.
	t.Run("el autor ve su carta y no los votos de la ronda", func(t *testing.T) {
		f := newOpenViewFixture()
		f.withVotes(2, SessionVote{User: f.alice, Card: "5"})

		view := ViewFor(f.snapshot, f.alice.ID)

		require.Len(t, view.Rounds, 2)
		assert.Nil(t, view.Rounds[1].Votes, "ni el autor recibe los votos de la ronda en votación")
		assert.True(t, participantByID(t, view, f.alice.ID).HasVoted)
		require.NotNil(t, view.MyVote)
		assert.Equal(t, Card("5"), *view.MyVote)
	})

	// Con Alice en 5 y Bob en 8, cada uno recibe solo su carta; Carol, que no votó, ninguna.
	t.Run("dos autores nunca reciben la carta del otro", func(t *testing.T) {
		f := newOpenViewFixture()
		f.withVotes(2,
			SessionVote{User: f.alice, Card: "5"},
			SessionVote{User: f.bob, Card: "8"},
		)

		aliceView := ViewFor(f.snapshot, f.alice.ID)
		bobView := ViewFor(f.snapshot, f.bob.ID)
		carolView := ViewFor(f.snapshot, f.carol.ID)

		assert.Equal(t, cardPtr("5"), aliceView.MyVote)
		assert.Equal(t, cardPtr("8"), bobView.MyVote)
		assert.Nil(t, carolView.MyVote)
		for _, view := range []SessionView{aliceView, bobView, carolView} {
			require.Len(t, view.Rounds, 2)
			assert.Nil(t, view.Rounds[1].Votes)
			assert.True(t, participantByID(t, view, f.alice.ID).HasVoted)
			assert.True(t, participantByID(t, view, f.bob.ID).HasVoted)
			assert.False(t, participantByID(t, view, f.carol.ID).HasVoted)
		}
	})

	// Un voto de una ronda anterior no cuenta como voto de la ronda vigente: quien votó en la 1 y
	// todavía no en la 2 no tiene carta propia ni figura como "ya votó".
	t.Run("un voto de una ronda anterior no es carta propia", func(t *testing.T) {
		f := newOpenViewFixture()
		f.withVotes(1, SessionVote{User: f.alice, Card: "3"})
		f.withVotes(2, SessionVote{User: f.bob, Card: "8"})

		view := ViewFor(f.snapshot, f.alice.ID)

		assert.Nil(t, view.MyVote)
		assert.False(t, participantByID(t, view, f.alice.ID).HasVoted)
		assert.True(t, participantByID(t, view, f.bob.ID).HasVoted)
	})
}

// Escenario: Cambio de voto (005/US2)
// Alice cambió de 8 a 13, así que la instantánea solo trae su voto vigente: ella recibe 13 en MyVote
// y para los demás sigue figurando solamente como "ya votó", sin ningún valor.
func TestUS2_DominioVistaCambioDeVoto(t *testing.T) {
	f := newOpenViewFixture()
	f.withVotes(2, SessionVote{User: f.alice, Card: "13"})

	aliceView := ViewFor(f.snapshot, f.alice.ID)
	bobView := ViewFor(f.snapshot, f.bob.ID)

	assert.Equal(t, cardPtr("13"), aliceView.MyVote)
	assert.Nil(t, bobView.MyVote)
	require.Len(t, bobView.Rounds, 2)
	assert.Nil(t, bobView.Rounds[1].Votes)
	assert.True(t, participantByID(t, bobView, f.alice.ID).HasVoted)
}

// Escenario: Carta desconocida (005/US2)
// La carta "?" cuenta como voto emitido igual que cualquier otra: su autor la recibe en MyVote y los
// demás lo ven como "ya votó", sin el valor.
func TestUS2_DominioVistaCartaDesconocida(t *testing.T) {
	f := newOpenViewFixture()
	f.withVotes(2, SessionVote{User: f.carol, Card: CardUnknown})

	carolView := ViewFor(f.snapshot, f.carol.ID)
	aliceView := ViewFor(f.snapshot, f.alice.ID)

	assert.Equal(t, cardPtr(CardUnknown), carolView.MyVote)
	assert.True(t, participantByID(t, aliceView, f.carol.ID).HasVoted)
	assert.Nil(t, aliceView.MyVote)
	require.Len(t, aliceView.Rounds, 2)
	assert.Nil(t, aliceView.Rounds[1].Votes)
}

// Escenario: Ronda revelada (005/US2)
// Las rondas anteriores ya reveladas muestran sus votos con sus autores a todos los espectadores,
// mientras la ronda vigente en votación sigue sin exponer los suyos.
func TestUS2_DominioVistaRondaRevelada(t *testing.T) {
	f := newOpenViewFixture()
	revealed := []SessionVote{
		{User: f.alice, Card: "3"},
		{User: f.bob, Card: "8"},
		{User: f.carol, Card: CardUnknown},
	}
	f.withVotes(1, revealed...)
	f.withVotes(2, SessionVote{User: f.alice, Card: "5"})

	// Cada espectador, haya votado o no en la ronda vigente, recibe los mismos votos revelados.
	for _, viewer := range []UserRef{f.alice, f.bob, f.carol} {
		t.Run(viewer.FullName, func(t *testing.T) {
			view := ViewFor(f.snapshot, viewer.ID)

			require.Len(t, view.Rounds, 2)
			assert.Equal(t, RoundStatusRevealed, view.Rounds[0].Status)
			assert.ElementsMatch(t, revealed, view.Rounds[0].Votes)
			assert.Nil(t, view.Rounds[1].Votes)
		})
	}
}

// Escenario: Cancelación por inactividad (005/US7)
// Una sesión que canceló el sistema sale con su motivo y sin quien la cerró: la vista copia el
// CancelReason de la instantánea junto con el estado y el momento de cierre, para que los
// participantes sepan por qué terminó (FR-063, RN-30, RN-31; T105). Cada motivo del sistema va en su
// propio subtest.
func TestUS7_DominioVistaCancelacionPorSistema(t *testing.T) {
	for _, reason := range []CancelReason{
		CancelReasonInactivity,
		CancelReasonStoryCompleted,
		CancelReasonProjectFinished,
	} {
		t.Run(string(reason), func(t *testing.T) {
			f := newOpenViewFixture()
			closedAt := f.round2At.Add(60 * time.Minute)
			f.snapshot.Session.Status = SessionStatusCanceled
			f.snapshot.Session.CancelReason = reasonPtr(reason)
			f.snapshot.Session.ClosedAt = &closedAt

			view := ViewFor(f.snapshot, f.alice.ID)

			assert.Equal(t, SessionStatusCanceled, view.Status)
			require.NotNil(t, view.CancelReason, "la cancelación del sistema expone su motivo")
			assert.Equal(t, reason, *view.CancelReason)
			assert.Nil(t, view.ClosedBy, "el sistema no figura como quien cerró la sesión")
			assert.Nil(t, view.AgreedStoryPoints)
			require.NotNil(t, view.ClosedAt)
			assert.True(t, closedAt.Equal(*view.ClosedAt))
		})
	}
}

// Escenario: Cancelación conserva la estimación (005/US7)
// Una sesión que canceló el facilitador sale con quien la cerró y sin motivo del sistema ni valor
// acordado: la vista deja CancelReason nulo porque la instantánea no lo tiene (FR-057, FR-058; T105).
func TestUS7_DominioVistaCancelacionPorFacilitador(t *testing.T) {
	f := newOpenViewFixture()
	closedAt := f.round2At.Add(time.Minute)
	f.snapshot.Session.Status = SessionStatusCanceled
	f.snapshot.Session.ClosedByID = &f.bob.ID
	f.snapshot.Session.ClosedAt = &closedAt
	f.snapshot.ClosedBy = &f.bob

	view := ViewFor(f.snapshot, f.alice.ID)

	assert.Equal(t, SessionStatusCanceled, view.Status)
	assert.Nil(t, view.CancelReason, "la cancelación del facilitador no lleva motivo del sistema")
	assert.Nil(t, view.AgreedStoryPoints)
	require.NotNil(t, view.ClosedBy)
	assert.Equal(t, f.bob, *view.ClosedBy)
	require.NotNil(t, view.ClosedAt)
	assert.True(t, closedAt.Equal(*view.ClosedAt))
}

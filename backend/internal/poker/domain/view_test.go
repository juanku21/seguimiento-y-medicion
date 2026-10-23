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

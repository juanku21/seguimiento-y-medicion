package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
	sprintsdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/sprints/domain"
)

// Pruebas de integración de punta a punta de la historia 1 de Planning Poker (T015): atraviesan el
// router compuesto como cmd/app detrás de httptest.NewServer, el servicio y el repositorio del
// módulo contra smye_test (Principios II y V). Cubren POST
// /api/v1/stories/{storyId}/estimation-sessions y el canal GET
// /api/v1/estimation-sessions/{sessionId}/events: cada instantánea se verifica sin recargar, porque
// llega por el canal ya abierto dentro del plazo de 2 segundos de Next (FR-073). Las cuentas, el
// proyecto, sus integrantes y las historias se crean con los endpoints de la 001 a la 003.

// Textos exactos que fija el contrato para los rechazos de la apertura y el 404 de sesión.
const (
	pokerOpenSessionExistsMessage = "La historia ya tiene una sesión de estimación en curso."
	pokerStoryCompletedMessage    = "Una historia completada no se estima."
	pokerProjectFinishedMessage   = "El proyecto está finalizado y es de solo lectura."
	pokerSessionNotFoundBody      = `{"error":"La sesión de estimación no existe."}`
)

// pokerCards son las once cartas de la escala en su orden (FR-020, RN-06).
var pokerCards = []string{"0", "1", "2", "3", "5", "8", "13", "20", "40", "100", "?"}

// Texto de la historia del caso normal: título, descripción y dos criterios en su orden.
const (
	pokerStoryTitle       = "Inicio de sesión con correo"
	pokerStoryDescription = "Como usuario quiero ingresar con mi correo para ver mis proyectos."
)

// pokerStoryCriteria son los criterios de aceptación de la historia del caso normal, en su orden.
var pokerStoryCriteria = []string{
	"El formulario rechaza un correo mal formado.",
	"Tras ingresar se muestra la lista de proyectos.",
}

// pokerUS1Fixture es el "Dado" común de US1: Ana, propietaria de un proyecto En curso con Bruno y
// Carla como integrantes; Diego, que no integra el proyecto, y una historia Pendiente con
// descripción y dos criterios.
type pokerUS1Fixture struct {
	env                                          *pokerTestEnv
	anaID, brunoID, carlaID, diegoID             uuid.UUID
	anaToken, brunoToken, carlaToken, diegoToken string
	projectID                                    uuid.UUID
	storyID                                      uuid.UUID
}

// newPokerUS1Fixture compone el entorno y prepara el "Dado" común de US1 con los endpoints de la
// 001 a la 003.
func newPokerUS1Fixture(t *testing.T) pokerUS1Fixture {
	t.Helper()
	env := newPokerTestEnv(t)
	f := pokerUS1Fixture{env: env}
	f.anaID, f.anaToken = env.signUp(t, "Ana Pérez", "ana@example.com")
	f.brunoID, f.brunoToken = env.signUp(t, "Bruno Gómez", "bruno@example.com")
	f.carlaID, f.carlaToken = env.signUp(t, "Carla Díaz", "carla@example.com")
	f.diegoID, f.diegoToken = env.signUp(t, "Diego Ruiz", "diego@example.com")

	// Proyecto En curso de Ana, con Bruno y Carla como integrantes.
	f.projectID = env.createProject(t, f.anaToken, "Portal de Clientes")
	env.addMember(t, f.anaToken, f.projectID, "bruno@example.com")
	env.addMember(t, f.anaToken, f.projectID, "carla@example.com")
	env.setProjectStatus(t, f.anaToken, f.projectID, "in_progress")

	// Historia Pendiente con descripción y dos criterios, creada por Bruno.
	description := pokerStoryDescription
	f.storyID = env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{
		Title:       pokerStoryTitle,
		Description: &description,
		Criteria:    pokerStoryCriteria,
	})
	return f
}

// pokerSessionRows cuenta las sesiones de la historia, opcionalmente filtradas por estado.
func pokerSessionRows(t *testing.T, db *gorm.DB, storyID uuid.UUID, status ...domain.SessionStatus) int64 {
	t.Helper()
	query := db.Model(&domain.EstimationSession{}).Where("story_id = ?", storyID)
	if len(status) > 0 {
		query = query.Where("status IN ?", status)
	}
	var n int64
	require.NoError(t, query.Count(&n).Error, "no se pudieron contar las sesiones de la historia")
	return n
}

// pokerStoredParticipant lee de estimation_participants la fila de la persona en la sesión.
func pokerStoredParticipant(t *testing.T, db *gorm.DB, sessionID string, userID uuid.UUID) domain.EstimationParticipant {
	t.Helper()
	var row domain.EstimationParticipant
	require.NoError(t, db.First(&row, "session_id = ? AND user_id = ?", sessionID, userID).Error,
		"no se encontró al participante %s en la sesión %s", userID, sessionID)
	return row
}

// requireConnectedExactly exige que la instantánea liste exactamente a las personas indicadas, todas
// conectadas, con Ana como única facilitadora.
func requireConnectedExactly(t *testing.T, snapshot pokerSessionBody, facilitatorID uuid.UUID, userIDs ...uuid.UUID) {
	t.Helper()
	require.Len(t, snapshot.Participants, len(userIDs), "participantes: %s", snapshot.Raw)
	facilitators := 0
	for _, p := range snapshot.Participants {
		if p.IsFacilitator {
			facilitators++
		}
	}
	assert.Equal(t, 1, facilitators, "debería haber exactamente un facilitador: %s", snapshot.Raw)
	for _, id := range userIDs {
		p := snapshot.participant(id)
		if assert.NotNil(t, p, "falta el participante %s: %s", id, snapshot.Raw) {
			assert.True(t, p.Connected, "%s debería figurar conectado", p.User.FullName)
			assert.Equal(t, id == facilitatorID, p.IsFacilitator, "marca de facilitador de %s", p.User.FullName)
		}
	}
}

// allConnected arma la condición "la instantánea lista exactamente a estas personas y todas están
// conectadas", para esperar con NextUntil.
func allConnected(userIDs ...uuid.UUID) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		if len(s.Participants) != len(userIDs) {
			return false
		}
		for _, id := range userIDs {
			p := s.participant(id)
			if p == nil || !p.Connected {
				return false
			}
		}
		return true
	}
}

// requireStoryText exige que la instantánea muestre el título, la descripción y los criterios de la
// historia del caso normal en su orden, y las once cartas de la escala (FR-012).
func requireStoryText(t *testing.T, snapshot pokerSessionBody, storyID uuid.UUID) {
	t.Helper()
	assert.Equal(t, storyID.String(), snapshot.Story.ID)
	assert.Equal(t, pokerStoryTitle, snapshot.Story.Title)
	if assert.NotNil(t, snapshot.Story.Description, "la descripción no debería ser null") {
		assert.Equal(t, pokerStoryDescription, *snapshot.Story.Description)
	}
	assert.Equal(t, pokerStoryCriteria, snapshot.Story.AcceptanceCriteria, "criterios en su orden")
	assert.Equal(t, pokerCards, snapshot.Cards)
}

// requireFirstRoundVoting exige que la sesión esté open con una única ronda, la 1, en votación.
func requireFirstRoundVoting(t *testing.T, snapshot pokerSessionBody) {
	t.Helper()
	assert.Equal(t, "open", snapshot.Status)
	if assert.Len(t, snapshot.Rounds, 1, "rondas: %s", snapshot.Raw) {
		assert.Equal(t, 1, snapshot.Rounds[0].Number)
		assert.Equal(t, "voting", snapshot.Rounds[0].Status)
	}
}

// Escenario: Apertura de sesión (005/US1)
// Bruno, integrante que no es el propietario, abre una sesión sobre una historia Pendiente de un
// proyecto En curso: responde 201 con la sesión open, la ronda 1 en votación, Bruno como facilitador
// y quien la abrió, y como único participante conectado; en la base queda una sola sesión open con
// facilitator_id y opened_by_id de Bruno, su ronda 1 voting y su participante conectado (FR-001 a
// FR-004, RN-02, RN-03).
func TestUS1_AperturaDeSesion(t *testing.T) {
	f := newPokerUS1Fixture(t)

	session := f.env.requireOpenedSession(t, f.brunoToken, f.storyID)

	// La respuesta es la sesión recién abierta, vista por Bruno.
	_, err := uuid.Parse(session.ID)
	require.NoError(t, err, "id = %q no es un UUID", session.ID)
	requireFirstRoundVoting(t, session)
	assert.Equal(t, f.brunoID.String(), session.Facilitator.ID)
	assert.Equal(t, "Bruno Gómez", session.Facilitator.FullName)
	assert.Equal(t, f.brunoID.String(), session.OpenedBy.ID)
	assert.Nil(t, session.ClosedBy)
	assert.Nil(t, session.ClosedAt)
	assert.Nil(t, session.AgreedStoryPoints)
	assert.Nil(t, session.CancelReason)
	requireStoryText(t, session, f.storyID)
	requireConnectedExactly(t, session, f.brunoID, f.brunoID)

	// En la base, una sola sesión open de la historia, con su ronda 1 y su participante conectado.
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusOpen))
	var stored domain.EstimationSession
	require.NoError(t, f.env.db.First(&stored, "id = ?", session.ID).Error, "no se encontró la sesión en la base")
	assert.Equal(t, f.brunoID, stored.FacilitatorID)
	assert.Equal(t, f.brunoID, stored.OpenedByID)
	var rounds []domain.EstimationRound
	require.NoError(t, f.env.db.Where("session_id = ?", session.ID).Find(&rounds).Error)
	if assert.Len(t, rounds, 1) {
		assert.Equal(t, 1, rounds[0].Number)
		assert.Equal(t, domain.RoundStatusVoting, rounds[0].Status)
	}
	assert.True(t, pokerStoredParticipant(t, f.env.db, session.ID, f.brunoID).Connected,
		"quien abre debería quedar como participante conectado")
}

// Escenario: Incorporación de dos integrantes (005/US1)
// Con la sesión abierta por Ana y su canal abierto, Bruno y luego Carla abren el canal: tras cada
// incorporación, todos los canales ya abiertos reciben, sin recargar y en menos de 2 segundos, la
// lista de los conectados, y al final los tres ven el título, la descripción y los criterios de la
// historia en su orden y la lista de los tres participantes conectados con Ana como facilitadora
// (FR-011 a FR-013, FR-073).
func TestUS1_IncorporacionDeDosIntegrantes(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)

	// Ana abre su canal y recibe la sesión con ella sola.
	ana := f.env.openStream(t, f.anaToken, session.ID)
	requireConnectedExactly(t, ana.NextUntil(t, "Ana conectada", allConnected(f.anaID)), f.anaID, f.anaID)

	// Se une Bruno: Ana y Bruno ven a los dos.
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	requireConnectedExactly(t, bruno.NextUntil(t, "Ana y Bruno conectados", allConnected(f.anaID, f.brunoID)), f.anaID, f.anaID, f.brunoID)
	requireConnectedExactly(t, ana.NextUntil(t, "Ana y Bruno conectados", allConnected(f.anaID, f.brunoID)), f.anaID, f.anaID, f.brunoID)

	// Se une Carla: los tres ven la historia completa y a los tres conectados.
	carla := f.env.openStream(t, f.carlaToken, session.ID)
	three := allConnected(f.anaID, f.brunoID, f.carlaID)
	for name, stream := range map[string]*pokerStream{"Ana": ana, "Bruno": bruno, "Carla": carla} {
		snapshot := stream.NextUntil(t, "los tres conectados en el canal de "+name, three)
		assert.Equal(t, session.ID, snapshot.ID, "canal de %s", name)
		requireFirstRoundVoting(t, snapshot)
		requireStoryText(t, snapshot, f.storyID)
		requireConnectedExactly(t, snapshot, f.anaID, f.anaID, f.brunoID, f.carlaID)
	}
}

// Escenario: Abandono de un participante (005/US1)
// Con Ana, Bruno y Carla conectados, Carla cierra su canal. A los 29 segundos el barrido todavía la
// considera conectada (gracia de FR-014); al cumplirse los 30 segundos el barrido registra su
// desconexión y los canales de Ana y Bruno reciben, sin recargar y en menos de 2 segundos, a Carla
// desconectada y a ellos dos conectados; en la base Carla queda desconectada con disconnected_at
// (FR-013, FR-014, FR-073).
func TestUS1_AbandonoDeUnParticipante(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	three := allConnected(f.anaID, f.brunoID, f.carlaID)

	// Los tres se unen y cada canal llega a verlos conectados.
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	carla := f.env.openStream(t, f.carlaToken, session.ID)
	ana.NextUntil(t, "los tres conectados en el canal de Ana", three)
	bruno.NextUntil(t, "los tres conectados en el canal de Bruno", three)
	carla.NextUntil(t, "los tres conectados en el canal de Carla", three)

	// Carla cierra su canal; dentro de la gracia sigue conectada.
	carla.Close()
	f.env.sweep(t, 29*time.Second)
	assert.True(t, pokerStoredParticipant(t, f.env.db, session.ID, f.carlaID).Connected,
		"a los 29 segundos Carla todavía está dentro de la gracia")

	// Vence la gracia: Ana y Bruno ven a Carla desconectada.
	f.env.sweep(t, time.Second)
	carlaGone := func(s pokerSessionBody) bool {
		p := s.participant(f.carlaID)
		return p != nil && !p.Connected
	}
	for name, stream := range map[string]*pokerStream{"Ana": ana, "Bruno": bruno} {
		snapshot := stream.NextUntil(t, "Carla desconectada en el canal de "+name, carlaGone)
		require.Len(t, snapshot.Participants, 3, "Carla sigue en la lista como desconectada: %s", snapshot.Raw)
		assert.True(t, snapshot.participant(f.anaID).Connected, "Ana sigue conectada (canal de %s)", name)
		assert.True(t, snapshot.participant(f.brunoID).Connected, "Bruno sigue conectado (canal de %s)", name)
	}
	stored := pokerStoredParticipant(t, f.env.db, session.ID, f.carlaID)
	assert.False(t, stored.Connected, "Carla debería quedar desconectada en la base")
	assert.NotNil(t, stored.DisconnectedAt, "debería registrarse el momento de la desconexión")
}

// Escenario: Historia sin criterios de aceptación (005/US1)
// Ana abre una sesión sobre una historia sin descripción ni criterios: la apertura responde 201 y
// tanto la respuesta como la instantánea del canal muestran la historia con la lista de criterios
// vacía, [] y no null, y la descripción null (FR-009, FR-012).
func TestUS1_HistoriaSinCriterios(t *testing.T) {
	f := newPokerUS1Fixture(t)
	storyID := f.env.createStory(t, f.anaToken, f.projectID, pokerStoryInput{Title: "Exportar el reporte"})

	session := f.env.requireOpenedSession(t, f.anaToken, storyID)
	requireFirstRoundVoting(t, session)
	requireEmptyCriteria(t, session, "Exportar el reporte")

	// La instantánea del canal muestra la misma historia.
	stream := f.env.openStream(t, f.anaToken, session.ID)
	requireEmptyCriteria(t, stream.NextUntil(t, "Ana conectada", allConnected(f.anaID)), "Exportar el reporte")
}

// requireEmptyCriteria exige la historia con el título indicado, la descripción null y la lista de
// criterios presente y vacía en el JSON recibido.
func requireEmptyCriteria(t *testing.T, snapshot pokerSessionBody, title string) {
	t.Helper()
	assert.Equal(t, title, snapshot.Story.Title)
	var raw struct {
		Story map[string]json.RawMessage `json:"story"`
	}
	require.NoError(t, json.Unmarshal(snapshot.Raw, &raw), "el cuerpo no es JSON válido: %s", snapshot.Raw)
	assert.JSONEq(t, `[]`, string(raw.Story["acceptanceCriteria"]), "los criterios deberían ser [] y no null ni ausentes")
	assert.JSONEq(t, `null`, string(raw.Story["description"]), "la descripción debería ser null")
}

// Escenario: Facilitador solo (005/US1)
// Ana abre la sesión y nadie más se une: su único canal recibe una sesión válida, open, con la ronda
// 1 en votación y Ana como única participante, conectada y facilitadora; pasados 30 segundos con el
// canal abierto, el barrido no la desconecta y la sesión sigue open (FR-002, FR-003, FR-014).
func TestUS1_FacilitadorSolo(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)

	stream := f.env.openStream(t, f.anaToken, session.ID)
	snapshot := stream.NextUntil(t, "Ana conectada", allConnected(f.anaID))
	requireFirstRoundVoting(t, snapshot)
	requireConnectedExactly(t, snapshot, f.anaID, f.anaID)
	assert.Equal(t, f.anaID.String(), snapshot.Facilitator.ID)

	// Con su canal abierto, la gracia no la alcanza.
	f.env.sweep(t, 30*time.Second)
	assert.True(t, pokerStoredParticipant(t, f.env.db, session.ID, f.anaID).Connected,
		"con su canal abierto Ana sigue conectada")
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusOpen))
}

// pokerClosedSessionSeed es una sesión cerrada sembrada en la base, con su ronda revelada y un voto,
// para comparar después de la apertura de una sesión nueva.
type pokerClosedSessionSeed struct {
	session domain.EstimationSession
	roundID uuid.UUID
}

// pokerSeedClosedSession inserta con GORM una sesión cerrada de la historia con el estado, el valor
// acordado y el motivo indicados, su participante, una ronda 1 revelada y un voto de quien la cerró,
// porque registrar y cancelar llegan en US5 y US7.
func pokerSeedClosedSession(t *testing.T, db *gorm.DB, storyID, userID uuid.UUID, status domain.SessionStatus, agreed *int, openedAt time.Time) pokerClosedSessionSeed {
	t.Helper()
	closedAt := openedAt.Add(20 * time.Minute)
	session := domain.EstimationSession{
		ID:                uuid.New(),
		StoryID:           storyID,
		FacilitatorID:     userID,
		Status:            status,
		AgreedStoryPoints: agreed,
		OpenedByID:        userID,
		ClosedByID:        &userID,
		ClosedAt:          &closedAt,
		LastActivityAt:    closedAt,
		Version:           4,
		CreatedAt:         openedAt,
	}
	require.NoError(t, db.Create(&session).Error, "no se pudo sembrar la sesión cerrada")
	participant := domain.EstimationParticipant{ID: uuid.New(), SessionID: session.ID, UserID: userID, Connected: false, DisconnectedAt: &closedAt}
	require.NoError(t, db.Create(&participant).Error, "no se pudo sembrar el participante")
	revealedAt := openedAt.Add(10 * time.Minute)
	round := domain.EstimationRound{ID: uuid.New(), SessionID: session.ID, Number: 1, Status: domain.RoundStatusRevealed, RevealedAt: &revealedAt, CreatedAt: openedAt}
	require.NoError(t, db.Create(&round).Error, "no se pudo sembrar la ronda")
	vote := domain.EstimationVote{ID: uuid.New(), RoundID: round.ID, ParticipantID: participant.ID, Card: "5"}
	require.NoError(t, db.Create(&vote).Error, "no se pudo sembrar el voto")
	return pokerClosedSessionSeed{session: session, roundID: round.ID}
}

// Escenario: Sesiones anteriores conservadas (005/US1)
// La historia tiene una sesión Finalizada con 5 Story Points acordados y otra Cancelada: Bruno abre
// una sesión nueva, que responde 201 con un identificador distinto y su propia ronda 1 en votación;
// las dos sesiones anteriores conservan su estado, su valor acordado, su cierre, su ronda revelada y
// su voto, y la historia queda con tres sesiones, una sola open (FR-062, FR-078, RN-04).
func TestUS1_SesionesAnterioresConservadas(t *testing.T) {
	f := newPokerUS1Fixture(t)
	five := 5
	finished := pokerSeedClosedSession(t, f.env.db, f.storyID, f.anaID, domain.SessionStatusFinished, &five, pokerFixedNow.Add(-48*time.Hour))
	canceled := pokerSeedClosedSession(t, f.env.db, f.storyID, f.anaID, domain.SessionStatusCanceled, nil, pokerFixedNow.Add(-24*time.Hour))

	session := f.env.requireOpenedSession(t, f.brunoToken, f.storyID)
	assert.NotEqual(t, finished.session.ID.String(), session.ID)
	assert.NotEqual(t, canceled.session.ID.String(), session.ID)
	requireFirstRoundVoting(t, session)

	// Las sesiones anteriores siguen como estaban, con su ronda y su voto.
	for _, seed := range []pokerClosedSessionSeed{finished, canceled} {
		var stored domain.EstimationSession
		require.NoError(t, f.env.db.First(&stored, "id = ?", seed.session.ID).Error, "la sesión anterior debería conservarse")
		assert.Equal(t, seed.session.Status, stored.Status)
		assert.Equal(t, seed.session.AgreedStoryPoints, stored.AgreedStoryPoints)
		if assert.NotNil(t, stored.ClosedAt) {
			assert.True(t, seed.session.ClosedAt.Equal(*stored.ClosedAt), "el cierre no debería cambiar")
		}
		var rounds, votes int64
		require.NoError(t, f.env.db.Model(&domain.EstimationRound{}).Where("session_id = ? AND status = ?", seed.session.ID, domain.RoundStatusRevealed).Count(&rounds).Error)
		require.NoError(t, f.env.db.Model(&domain.EstimationVote{}).Where("round_id = ?", seed.roundID).Count(&votes).Error)
		assert.Equal(t, int64(1), rounds, "la ronda revelada de la sesión anterior debería conservarse")
		assert.Equal(t, int64(1), votes, "el voto de la sesión anterior debería conservarse")
	}
	assert.Equal(t, int64(3), pokerSessionRows(t, f.env.db, f.storyID))
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusOpen))
}

// Escenario: Segunda sesión abierta (005/US1)
// Con la sesión abierta por Ana, Bruno intenta abrir otra sobre la misma historia: responde 409 con
// el mensaje de sesión en curso y el sessionId de la existente para unirse a ella, y en la base la
// historia sigue con una sola sesión, open (FR-006, RN-04).
func TestUS1_SegundaSesionAbierta(t *testing.T) {
	f := newPokerUS1Fixture(t)
	existing := f.env.requireOpenedSession(t, f.anaToken, f.storyID)

	resp := f.env.openSession(t, f.brunoToken, f.storyID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, `{"error":"`+pokerOpenSessionExistsMessage+`","sessionId":"`+existing.ID+`"}`, string(resp.Body))
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID))
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusOpen))
}

// Escenario: Historia completada (005/US1)
// Una historia lista, comprometida en el sprint Activo del proyecto, avanza a En progreso y a
// Completada con PUT /api/v1/stories/{storyId}/status de la 004; Bruno intenta abrir una sesión
// sobre ella: responde 409 con el mensaje de que una historia completada no se estima y no se crea
// ninguna sesión (FR-007, RN-05).
func TestUS1_HistoriaCompletada(t *testing.T) {
	f := newPokerUS1Fixture(t)
	five := 5
	storyID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{
		Title:    "Exportar el reporte",
		Points:   &five,
		Criteria: []string{"El PDF incluye la velocidad de los últimos cinco sprints."},
	})
	pokerSeedActiveCommitment(t, f.env.db, f.projectID, f.anaID, storyID)
	for _, status := range []string{"in_progress", "completed"} {
		requirePokerStatus(t, f.env.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", f.brunoToken,
			map[string]any{"status": status}), http.StatusOK, nil)
	}

	resp := f.env.openSession(t, f.brunoToken, storyID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, `{"error":"`+pokerStoryCompletedMessage+`"}`, string(resp.Body))
	assert.Zero(t, pokerSessionRows(t, f.env.db, storyID), "no debería crearse ninguna sesión")
}

// pokerSeedActiveCommitment inserta con GORM un sprint Activo del proyecto que contiene al reloj
// falso y el compromiso de la historia en él, como lo dejan la creación, la asignación y el inicio
// de la 004, cuyo ciclo completo no es objeto de esta prueba.
func pokerSeedActiveCommitment(t *testing.T, db *gorm.DB, projectID, authorID, storyID uuid.UUID) {
	t.Helper()
	committed := 5
	startedAt := time.Date(2026, time.October, 19, 9, 0, 0, 0, time.UTC)
	sprint := sprintsdomain.Sprint{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "Sprint 1",
		Goal:        "Cerrar el alta de clientes",
		StartDate:   time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC),
		Status:      sprintsdomain.SprintStatusPlanned,
		CreatedByID: authorID,
	}
	require.NoError(t, db.Create(&sprint).Error, "no se pudo sembrar el sprint")
	require.NoError(t, db.Model(&sprintsdomain.Sprint{}).Where("id = ?", sprint.ID).Updates(map[string]any{
		"status": sprintsdomain.SprintStatusActive, "started_at": startedAt, "committed_story_points": committed,
	}).Error, "no se pudo activar el sprint")
	row := sprintsdomain.SprintStory{ID: uuid.New(), SprintID: sprint.ID, StoryID: storyID, CommittedAt: startedAt}
	require.NoError(t, db.Create(&row).Error, "no se pudo sembrar el compromiso")
	require.NoError(t, db.Model(&backlogdomain.UserStory{}).Where("id = ?", storyID).Update("ever_committed", true).Error,
		"no se pudo marcar ever_committed")
}

// Escenario: Proyecto finalizado (005/US1)
// Una historia Pendiente de un proyecto que Ana finaliza con PATCH /api/v1/projects/{id}/status de
// la 002: Bruno intenta abrir una sesión sobre ella y responde 409 con el mensaje de proyecto de
// solo lectura, sin crear ninguna sesión (FR-008, RN-05).
func TestUS1_ProyectoFinalizado(t *testing.T) {
	f := newPokerUS1Fixture(t)
	f.env.setProjectStatus(t, f.anaToken, f.projectID, "finished")

	resp := f.env.openSession(t, f.brunoToken, f.storyID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, `{"error":"`+pokerProjectFinishedMessage+`"}`, string(resp.Body))
	assert.Zero(t, pokerSessionRows(t, f.env.db, f.storyID), "no debería crearse ninguna sesión")
}

// Escenario: No integrante (005/US1)
// Con la sesión abierta por Ana, Diego, que no integra el proyecto, pide el canal de la sesión:
// responde 404 con el mismo código, tipo de contenido y cuerpo byte a byte que el canal de un
// sessionId inexistente ("La sesión de estimación no existe."), y Diego no queda como participante
// (FR-087, FR-088, SC-018).
func TestUS1_NoIntegrante(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)

	missing := f.env.do(t, http.MethodGet, eventsPath(uuid.New().String()), f.diegoToken, nil)
	foreign := f.env.do(t, http.MethodGet, eventsPath(session.ID), f.diegoToken, nil)

	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body)
	assert.Equal(t, pokerSessionNotFoundBody, string(missing.Body), "inexistente: el cuerpo debería ser el del contrato")
	assert.Equal(t, http.StatusNotFound, foreign.Code, "ajena: cuerpo: %s", foreign.Body)
	assert.Equal(t, missing.Code, foreign.Code, "los códigos deberían ser idénticos")
	assert.Equal(t, missing.ContentType, foreign.ContentType, "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missing.Body, foreign.Body, "los cuerpos deberían ser idénticos byte a byte")

	var n int64
	require.NoError(t, f.env.db.Model(&domain.EstimationParticipant{}).Where("user_id = ?", f.diegoID).Count(&n).Error)
	assert.Zero(t, n, "Diego no debería quedar como participante")
}

// Pruebas de integración de punta a punta de la historia 2 de Planning Poker (T036): votan con PUT
// /api/v1/estimation-sessions/{sessionId}/vote sobre el mismo router compuesto como cmd/app y contra
// smye_test, y verifican la respuesta, las instantáneas que llegan sin recargar por los canales ya
// abiertos y las filas de estimation_votes. El "Dado" deja a Ana sin votar, así ningún voto completa
// la ronda y la revelación automática de US3 no interviene. El cambio simultáneo con la revelación
// se prueba en US3 (T047), porque necesita revelar.

// Textos exactos que fija el contrato para los rechazos del voto.
const (
	pokerInvalidFieldsMessage   = "Hay campos inválidos en la solicitud."
	pokerInvalidCardMessage     = "La carta no es válida. Cartas admitidas: 0, 1, 2, 3, 5, 8, 13, 20, 40, 100 y ?."
	pokerWithdrawVoteMessage    = `Un voto emitido no se retira: solo se cambia por otra carta, incluida "?".`
	pokerNotParticipantBodyJSON = `{"error":"No participás de esta sesión de estimación."}`
	pokerRoundRevealedBodyJSON  = `{"error":"La ronda ya está revelada. Esperá una ronda nueva para votar."}`
)

// votePath es la ruta del voto de la sesión según contracts/openapi.yaml.
func votePath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/vote"
}

// vote pide PUT /api/v1/estimation-sessions/{sessionId}/vote con el cuerpo indicado y el token como
// Bearer.
func (e *pokerTestEnv) vote(t *testing.T, token, sessionID string, body any) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPut, votePath(sessionID), token, body)
}

// requireVoted vota la carta indicada como texto, exige el 200 y devuelve la sesión resultante vista
// por quien votó.
func (e *pokerTestEnv) requireVoted(t *testing.T, token, sessionID, card string) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.vote(t, token, sessionID, map[string]any{"card": card})
	requirePokerStatus(t, resp, http.StatusOK, &session)
	session.Raw = resp.Body
	return session
}

// pokerUS2Fixture es el "Dado" común de US2: el de US1 con una sesión abierta por Ana y los canales
// de Ana, Bruno y Carla abiertos, los tres conectados y la ronda 1 en votación.
type pokerUS2Fixture struct {
	pokerUS1Fixture
	session           pokerSessionBody
	ana, bruno, carla *pokerStream
}

// newPokerUS2Fixture prepara el "Dado" común de US2 y espera a que cada canal vea a los tres
// conectados, para que la instantánea siguiente sea la del voto.
func newPokerUS2Fixture(t *testing.T) pokerUS2Fixture {
	t.Helper()
	f := pokerUS2Fixture{pokerUS1Fixture: newPokerUS1Fixture(t)}
	f.session = f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	f.ana = f.env.openStream(t, f.anaToken, f.session.ID)
	f.bruno = f.env.openStream(t, f.brunoToken, f.session.ID)
	f.carla = f.env.openStream(t, f.carlaToken, f.session.ID)
	three := allConnected(f.anaID, f.brunoID, f.carlaID)
	f.ana.NextUntil(t, "los tres conectados en el canal de Ana", three)
	f.bruno.NextUntil(t, "los tres conectados en el canal de Bruno", three)
	f.carla.NextUntil(t, "los tres conectados en el canal de Carla", three)
	return f
}

// hasVoted arma la condición "la persona figura como que ya votó en la ronda vigente", para esperar
// con NextUntil.
func hasVoted(userID uuid.UUID) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		p := s.participant(userID)
		return p != nil && p.HasVoted
	}
}

// pokerCurrentRoundVotes devuelve el JSON del campo votes de la ronda vigente (la última) tal como se
// recibió, para distinguir null de una lista.
func pokerCurrentRoundVotes(t *testing.T, snapshot pokerSessionBody) string {
	t.Helper()
	var raw struct {
		Rounds []map[string]json.RawMessage `json:"rounds"`
	}
	require.NoError(t, json.Unmarshal(snapshot.Raw, &raw), "el cuerpo no es JSON válido: %s", snapshot.Raw)
	require.NotEmpty(t, raw.Rounds, "la sesión debería tener rondas: %s", snapshot.Raw)
	votes, ok := raw.Rounds[len(raw.Rounds)-1]["votes"]
	require.True(t, ok, "la ronda vigente debería incluir el campo votes: %s", snapshot.Raw)
	return string(votes)
}

// requireSecretVote exige que la instantánea, vista por alguien distinto del votante, muestre al
// votante como que ya votó, la ronda vigente en votación con votes null y myVote null, porque quien
// mira no votó (FR-023).
func requireSecretVote(t *testing.T, snapshot pokerSessionBody, voterID uuid.UUID, viewer string) {
	t.Helper()
	if assert.NotEmpty(t, snapshot.Rounds, "%s: rondas: %s", viewer, snapshot.Raw) {
		assert.Equal(t, "voting", snapshot.Rounds[len(snapshot.Rounds)-1].Status, "%s: la ronda sigue en votación", viewer)
	}
	if p := snapshot.participant(voterID); assert.NotNil(t, p, "%s: falta el votante: %s", viewer, snapshot.Raw) {
		assert.True(t, p.HasVoted, "%s debería ver que %s ya votó", viewer, p.User.FullName)
	}
	assert.JSONEq(t, `null`, pokerCurrentRoundVotes(t, snapshot), "%s no debería ver ningún voto de la ronda en votación", viewer)
	assert.Nil(t, snapshot.MyVote, "%s no votó, así que myVote debería ser null", viewer)
}

// requireOwnVote exige que la instantánea, vista por el votante, lleve su carta en myVote, lo muestre
// como que ya votó y mantenga votes null en la ronda vigente (FR-023, FR-024).
func requireOwnVote(t *testing.T, snapshot pokerSessionBody, voterID uuid.UUID, card, where string) {
	t.Helper()
	if assert.NotNil(t, snapshot.MyVote, "%s: myVote no debería ser null: %s", where, snapshot.Raw) {
		assert.Equal(t, card, *snapshot.MyVote, "%s: myVote", where)
	}
	if p := snapshot.participant(voterID); assert.NotNil(t, p, "%s: falta el votante: %s", where, snapshot.Raw) {
		assert.True(t, p.HasVoted, "%s: el votante debería figurar como que ya votó", where)
	}
	assert.JSONEq(t, `null`, pokerCurrentRoundVotes(t, snapshot), "%s: votes es null también para el autor", where)
}

// pokerStoredVotes lee de estimation_votes las filas de la persona en todas las rondas de la sesión.
func pokerStoredVotes(t *testing.T, db *gorm.DB, sessionID string, userID uuid.UUID) []domain.EstimationVote {
	t.Helper()
	var votes []domain.EstimationVote
	require.NoError(t, db.
		Joins("JOIN estimation_participants ON estimation_participants.id = estimation_votes.participant_id").
		Where("estimation_participants.session_id = ? AND estimation_participants.user_id = ?", sessionID, userID).
		Find(&votes).Error, "no se pudieron leer los votos de %s en la sesión %s", userID, sessionID)
	return votes
}

// requireSingleStoredVote exige que la persona tenga exactamente una fila de voto en la sesión, con
// la carta indicada.
func requireSingleStoredVote(t *testing.T, db *gorm.DB, sessionID string, userID uuid.UUID, card string) {
	t.Helper()
	votes := pokerStoredVotes(t, db, sessionID, userID)
	if assert.Len(t, votes, 1, "debería haber exactamente una fila de voto de %s", userID) {
		assert.Equal(t, domain.Card(card), votes[0].Card)
	}
}

// pokerStoredVersion lee la versión vigente de la sesión en la base.
func pokerStoredVersion(t *testing.T, db *gorm.DB, sessionID string) int64 {
	t.Helper()
	var stored domain.EstimationSession
	require.NoError(t, db.First(&stored, "id = ?", sessionID).Error, "no se encontró la sesión %s", sessionID)
	return stored.Version
}

// requireCardRejected exige el 400 del contrato con un único detalle en card y el mensaje indicado.
func requireCardRejected(t *testing.T, resp pokerResponse, message string) {
	t.Helper()
	want, err := json.Marshal(map[string]any{
		"error":  pokerInvalidFieldsMessage,
		"fields": []map[string]string{{"field": "card", "message": message}},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, string(want), string(resp.Body))
}

// Escenario: Voto oculto (005/US2)
// Con Ana, Bruno y Carla conectados y la ronda 1 en votación, Bruno juega la carta 5: responde 200
// con la sesión vista por Bruno; los canales de Ana y Carla reciben, sin recargar y en menos de 2
// segundos, a Bruno como que ya votó, con votes null y sin el valor; en la base queda su voto 5 y
// la versión de la sesión sube en uno (FR-021, FR-023, FR-073, SC-005).
func TestUS2_VotoOculto(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	requireOwnVote(t, resp, f.brunoID, "5", "respuesta de Bruno")

	// Ana y Carla ven que Bruno votó, pero no qué votó.
	for name, stream := range map[string]*pokerStream{"Ana": f.ana, "Carla": f.carla} {
		requireSecretVote(t, stream.NextUntil(t, "Bruno ya votó en el canal de "+name, hasVoted(f.brunoID)), f.brunoID, name)
	}
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "5")
	assert.Equal(t, before+1, pokerStoredVersion(t, f.env.db, f.session.ID), "el voto es un hecho y sube la versión")
}

// Escenario: Propia carta visible (005/US2)
// Bruno juega la carta 5: la respuesta, su canal abierto y una segunda conexión que abre después
// llevan su carta en myVote, igual en todas sus conexiones, mientras el canal de Ana lo muestra solo
// como que ya votó (FR-023, FR-024).
func TestUS2_PropiaCartaVisible(t *testing.T) {
	f := newPokerUS2Fixture(t)

	requireOwnVote(t, f.env.requireVoted(t, f.brunoToken, f.session.ID, "5"), f.brunoID, "5", "respuesta de Bruno")
	requireOwnVote(t, f.bruno.NextUntil(t, "Bruno ya votó en su canal", hasVoted(f.brunoID)), f.brunoID, "5", "primer canal de Bruno")

	// Una segunda conexión de Bruno recibe la misma carta propia.
	second := f.env.openStream(t, f.brunoToken, f.session.ID)
	requireOwnVote(t, second.NextUntil(t, "Bruno ya votó en su segunda conexión", hasVoted(f.brunoID)), f.brunoID, "5", "segunda conexión de Bruno")

	// Ana distingue la carta de Bruno solo como "ya votó".
	requireSecretVote(t, f.ana.NextUntil(t, "Bruno ya votó en el canal de Ana", hasVoted(f.brunoID)), f.brunoID, "Ana")
}

// Escenario: Cambio de voto (005/US2)
// Bruno juega la carta 8 y después la 13 antes de la revelación: la segunda respuesta lleva myVote
// 13; Ana y Carla siguen viendo solo que Bruno ya votó, y en la base queda una única fila de voto de
// Bruno, con 13: el 8 se descarta (FR-022, RN-07).
func TestUS2_CambioDeVoto(t *testing.T) {
	f := newPokerUS2Fixture(t)

	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	changed := f.env.requireVoted(t, f.brunoToken, f.session.ID, "13")
	requireOwnVote(t, changed, f.brunoID, "13", "respuesta del cambio de Bruno")

	// Los demás reciben la instantánea del cambio y siguen sin ver el valor.
	afterChange := func(s pokerSessionBody) bool { return s.Version >= changed.Version && hasVoted(f.brunoID)(s) }
	for name, stream := range map[string]*pokerStream{"Ana": f.ana, "Carla": f.carla} {
		requireSecretVote(t, stream.NextUntil(t, "el cambio de Bruno en el canal de "+name, afterChange), f.brunoID, name)
	}
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "13")
}

// Escenario: Carta desconocida (005/US2)
// Bruno juega la carta "?": responde 200 con myVote "?", Ana y Carla lo ven como que ya votó igual
// que con cualquier otra carta, la ronda sigue en votación y en la base queda su voto "?" (FR-020,
// RC-04).
func TestUS2_CartaDesconocida(t *testing.T) {
	f := newPokerUS2Fixture(t)

	requireOwnVote(t, f.env.requireVoted(t, f.brunoToken, f.session.ID, "?"), f.brunoID, "?", "respuesta de Bruno")
	for name, stream := range map[string]*pokerStream{"Ana": f.ana, "Carla": f.carla} {
		requireSecretVote(t, stream.NextUntil(t, "Bruno ya votó en el canal de "+name, hasVoted(f.brunoID)), f.brunoID, name)
	}
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "?")
}

// Escenario: Extremos de la escala (005/US2)
// Con la ronda en votación, Bruno juega la carta 0 y Carla la 100: los dos votos se aceptan con 200,
// cada respuesta lleva la carta de quien votó, Ana ve a los dos como que ya votaron sin los valores
// y en la base quedan 0 y 100 (FR-020, RN-06).
func TestUS2_ExtremosDeLaEscala(t *testing.T) {
	f := newPokerUS2Fixture(t)

	requireOwnVote(t, f.env.requireVoted(t, f.brunoToken, f.session.ID, "0"), f.brunoID, "0", "respuesta de Bruno")
	requireOwnVote(t, f.env.requireVoted(t, f.carlaToken, f.session.ID, "100"), f.carlaID, "100", "respuesta de Carla")

	// Ana ve a los dos como que ya votaron, sin los valores.
	both := func(s pokerSessionBody) bool { return hasVoted(f.brunoID)(s) && hasVoted(f.carlaID)(s) }
	snapshot := f.ana.NextUntil(t, "Bruno y Carla ya votaron en el canal de Ana", both)
	requireSecretVote(t, snapshot, f.brunoID, "Ana")
	requireSecretVote(t, snapshot, f.carlaID, "Ana")

	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "0")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.carlaID, "100")
}

// Escenario: Valor fuera de la escala (005/US2)
// Bruno ya jugó la carta 13 e intenta votar 4, 50 y 1.5: cada intento responde 400 con el detalle en
// card que enumera las cartas admitidas, su voto sigue siendo 13 en la base y la versión de la sesión
// no cambia, porque el rechazo no es un hecho (FR-025, RN-06, SC-010).
func TestUS2_ValorFueraDeEscala(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "13")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	for _, card := range []string{"4", "50", "1.5"} {
		t.Run(card, func(t *testing.T) {
			resp := f.env.vote(t, f.brunoToken, f.session.ID, map[string]any{"card": card})

			requireCardRejected(t, resp, pokerInvalidCardMessage)
			requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "13")
			assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
		})
	}
}

// pokerSeedRevealedRound deja revelada con GORM la ronda vigente de la sesión, con un voto de la
// persona indicada, porque revelar llega en US3.
func pokerSeedRevealedRound(t *testing.T, db *gorm.DB, sessionID string, voterID uuid.UUID, card string) domain.EstimationRound {
	t.Helper()
	var round domain.EstimationRound
	require.NoError(t, db.Where("session_id = ?", sessionID).Order("number DESC").First(&round).Error,
		"no se encontró la ronda vigente de la sesión %s", sessionID)
	participant := pokerStoredParticipant(t, db, sessionID, voterID)
	vote := domain.EstimationVote{ID: uuid.New(), RoundID: round.ID, ParticipantID: participant.ID, Card: domain.Card(card)}
	require.NoError(t, db.Create(&vote).Error, "no se pudo sembrar el voto")
	revealedAt := pokerFixedNow.Add(time.Minute)
	require.NoError(t, db.Model(&domain.EstimationRound{}).Where("id = ?", round.ID).Updates(map[string]any{
		"status": domain.RoundStatusRevealed, "revealed_at": revealedAt,
	}).Error, "no se pudo revelar la ronda")
	round.Status = domain.RoundStatusRevealed
	round.RevealedAt = &revealedAt
	return round
}

// Escenario: Ronda revelada (005/US2)
// La ronda vigente ya está revelada con el voto 5 de Bruno: Bruno intenta cambiar su voto a 8 y
// Carla intenta votar 3; los dos reciben 409 con el mensaje de que la ronda está revelada y hay que
// esperar una ronda nueva, el voto revelado de Bruno sigue en 5, Carla no tiene voto y la ronda sigue
// revelada (FR-026).
func TestUS2_RondaRevelada(t *testing.T) {
	f := newPokerUS2Fixture(t)
	round := pokerSeedRevealedRound(t, f.env.db, f.session.ID, f.brunoID, "5")

	attempts := []struct {
		name  string
		token string
		card  string
	}{
		{"cambiar el voto", f.brunoToken, "8"},
		{"votar por primera vez", f.carlaToken, "3"},
	}
	for _, tc := range attempts {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.env.vote(t, tc.token, f.session.ID, map[string]any{"card": tc.card})

			assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
			assert.JSONEq(t, pokerRoundRevealedBodyJSON, string(resp.Body))
		})
	}

	// Los votos revelados no cambian y la ronda sigue revelada.
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "5")
	assert.Empty(t, pokerStoredVotes(t, f.env.db, f.session.ID, f.carlaID), "Carla no debería tener voto")
	var stored domain.EstimationRound
	require.NoError(t, f.env.db.First(&stored, "id = ?", round.ID).Error)
	assert.Equal(t, domain.RoundStatusRevealed, stored.Status)
}

// Escenario: Integrante no unido (005/US2)
// Con la sesión abierta por Ana, Carla, integrante del proyecto que nunca abrió el canal, intenta
// votar 5: responde 403 con el mensaje de que no participa de la sesión, Carla no queda como
// participante, la sesión no tiene ningún voto y su versión no cambia (FR-028).
func TestUS2_IntegranteNoUnido(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	before := pokerStoredVersion(t, f.env.db, session.ID)

	resp := f.env.vote(t, f.carlaToken, session.ID, map[string]any{"card": "5"})

	assert.Equal(t, http.StatusForbidden, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerNotParticipantBodyJSON, string(resp.Body))
	var participants, votes int64
	require.NoError(t, f.env.db.Model(&domain.EstimationParticipant{}).
		Where("session_id = ? AND user_id = ?", session.ID, f.carlaID).Count(&participants).Error)
	require.NoError(t, f.env.db.Model(&domain.EstimationVote{}).
		Joins("JOIN estimation_rounds ON estimation_rounds.id = estimation_votes.round_id").
		Where("estimation_rounds.session_id = ?", session.ID).Count(&votes).Error)
	assert.Zero(t, participants, "Carla no debería quedar como participante")
	assert.Zero(t, votes, "la sesión no debería tener votos")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, session.ID), "un rechazo no sube la versión")
}

// Escenario: Retiro de voto (005/US2)
// Bruno ya jugó la carta 5 e intenta retirar su voto enviando la carta nula, vacía o ausente: cada
// intento responde 400 con el detalle en card que indica que solo se cambia por otra carta, incluida
// "?"; su voto sigue en 5 y la versión de la sesión no cambia (FR-022, RN-36).
func TestUS2_RetiroDeVoto(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	attempts := []struct {
		name string
		body map[string]any
	}{
		{"carta nula", map[string]any{"card": nil}},
		{"carta vacía", map[string]any{"card": ""}},
		{"sin carta", map[string]any{}},
	}
	for _, tc := range attempts {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.env.vote(t, f.brunoToken, f.session.ID, tc.body)

			requireCardRejected(t, resp, pokerWithdrawVoteMessage)
			requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "5")
			assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
		})
	}
}

// Pruebas de integración de punta a punta de la historia 3 de Planning Poker (T047): revelan la
// ronda sola, al completarse el último voto de los conectados o al vencer la gracia de quien se
// desconecta sin votar, o a mano con POST /api/v1/estimation-sessions/{sessionId}/reveal, sobre el
// mismo router compuesto como cmd/app y contra smye_test. Verifican la respuesta, las instantáneas
// que llegan sin recargar por los canales ya abiertos y la ronda guardada en estimation_rounds. El
// orden de los votos revelados y de notVoted no lo fija el contrato, así que se comparan como
// conjuntos; los autores de cada extremo sí se esperan ordenados por nombre (T046). También va acá
// el cambio de voto simultáneo con la revelación de US2, porque necesita revelar.

// Textos exactos que fija el contrato para los rechazos de la revelación.
const (
	pokerNoVotesToRevealBodyJSON = `{"error":"Hace falta al menos un voto para revelar la ronda."}`
	pokerFacilitatorOnlyBodyJSON = `{"error":"Esta acción está reservada al facilitador de la sesión."}`
	pokerAlreadyRevealedBodyJSON = `{"error":"La ronda ya fue revelada."}`
)

// revealPath es la ruta de la revelación manual de la sesión según contracts/openapi.yaml.
func revealPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/reveal"
}

// reveal pide POST /api/v1/estimation-sessions/{sessionId}/reveal, sin cuerpo, con el token como
// Bearer.
func (e *pokerTestEnv) reveal(t *testing.T, token, sessionID string) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, revealPath(sessionID), token, nil)
}

// pokerVoteBody refleja EstimationVote del contrato.
type pokerVoteBody struct {
	User pokerUserRef `json:"user"`
	Card string       `json:"card"`
}

// pokerOutcomeExtremeBody refleja OutcomeExtreme del contrato.
type pokerOutcomeExtremeBody struct {
	Value   int            `json:"value"`
	Authors []pokerUserRef `json:"authors"`
}

// pokerOutcomeBody refleja RoundOutcome del contrato.
type pokerOutcomeBody struct {
	Kind  string                   `json:"kind"`
	Value *int                     `json:"value"`
	Min   *pokerOutcomeExtremeBody `json:"min"`
	Max   *pokerOutcomeExtremeBody `json:"max"`
}

// pokerRevealedRoundBody refleja EstimationRound del contrato con los campos de la revelación, que
// pokerRoundBody de US1 no lee.
type pokerRevealedRoundBody struct {
	Number     int               `json:"number"`
	Status     string            `json:"status"`
	RevealedAt *string           `json:"revealedAt"`
	Votes      []pokerVoteBody   `json:"votes"`
	Outcome    *pokerOutcomeBody `json:"outcome"`
	NotVoted   []pokerUserRef    `json:"notVoted"`
}

// pokerCurrentRound decodifica del JSON recibido la ronda vigente (la última) con sus votos, su
// resultado y quienes no votaron.
func pokerCurrentRound(t *testing.T, snapshot pokerSessionBody) pokerRevealedRoundBody {
	t.Helper()
	var raw struct {
		Rounds []pokerRevealedRoundBody `json:"rounds"`
	}
	require.NoError(t, json.Unmarshal(snapshot.Raw, &raw), "el cuerpo no es JSON válido: %s", snapshot.Raw)
	require.NotEmpty(t, raw.Rounds, "la sesión debería tener rondas: %s", snapshot.Raw)
	return raw.Rounds[len(raw.Rounds)-1]
}

// currentRoundRevealed es la condición "la ronda vigente está revelada", para esperar con NextUntil.
func currentRoundRevealed(s pokerSessionBody) bool {
	return len(s.Rounds) > 0 && s.Rounds[len(s.Rounds)-1].Status == "revealed"
}

// pokerRevealedCards devuelve las cartas reveladas de la ronda indexadas por el identificador de su
// autor, para compararlas como conjunto.
func pokerRevealedCards(round pokerRevealedRoundBody) map[string]string {
	cards := make(map[string]string, len(round.Votes))
	for _, v := range round.Votes {
		cards[v.User.ID] = v.Card
	}
	return cards
}

// pokerUserIDs devuelve los identificadores de las referencias en su orden.
func pokerUserIDs(users []pokerUserRef) []string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

// pokerIDStrings convierte los identificadores a texto, en su orden.
func pokerIDStrings(ids ...uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// requireRevealedRound exige que la instantánea tenga la ronda vigente revelada con su momento de
// revelado, exactamente los votos indicados (autor → carta) y, en notVoted, exactamente a las
// personas indicadas; devuelve la ronda para verificar su resultado.
func requireRevealedRound(t *testing.T, snapshot pokerSessionBody, votes map[uuid.UUID]string, notVoted []uuid.UUID, where string) pokerRevealedRoundBody {
	t.Helper()
	round := pokerCurrentRound(t, snapshot)
	require.Equal(t, "revealed", round.Status, "%s: la ronda vigente debería estar revelada: %s", where, snapshot.Raw)
	assert.NotNil(t, round.RevealedAt, "%s: revealedAt no debería ser null", where)
	want := make(map[string]string, len(votes))
	for id, card := range votes {
		want[id.String()] = card
	}
	assert.Equal(t, want, pokerRevealedCards(round), "%s: votos revelados: %s", where, snapshot.Raw)
	assert.ElementsMatch(t, pokerIDStrings(notVoted...), pokerUserIDs(round.NotVoted), "%s: notVoted: %s", where, snapshot.Raw)
	return round
}

// requireConsensus exige un resultado consensus con el valor indicado y sin mínimo ni máximo
// (FR-037, FR-040).
func requireConsensus(t *testing.T, round pokerRevealedRoundBody, value int, where string) {
	t.Helper()
	require.NotNil(t, round.Outcome, "%s: la ronda revelada debería tener resultado", where)
	assert.Equal(t, "consensus", round.Outcome.Kind, "%s: tipo de resultado", where)
	if assert.NotNil(t, round.Outcome.Value, "%s: el consenso debería llevar su valor", where) {
		assert.Equal(t, value, *round.Outcome.Value, "%s: valor del consenso", where)
	}
	assert.Nil(t, round.Outcome.Min, "%s: el consenso no señala mínimo", where)
	assert.Nil(t, round.Outcome.Max, "%s: el consenso no señala máximo", where)
}

// requireDivergence exige un resultado divergence sin valor, con el mínimo y el máximo indicados y
// sus autores en el orden dado, que es el orden por nombre (FR-038, FR-039).
func requireDivergence(t *testing.T, round pokerRevealedRoundBody, minValue int, minAuthors []uuid.UUID, maxValue int, maxAuthors []uuid.UUID, where string) {
	t.Helper()
	require.NotNil(t, round.Outcome, "%s: la ronda revelada debería tener resultado", where)
	assert.Equal(t, "divergence", round.Outcome.Kind, "%s: tipo de resultado", where)
	assert.Nil(t, round.Outcome.Value, "%s: la divergencia no lleva valor", where)
	if assert.NotNil(t, round.Outcome.Min, "%s: la divergencia debería señalar el mínimo", where) {
		assert.Equal(t, minValue, round.Outcome.Min.Value, "%s: valor mínimo", where)
		assert.Equal(t, pokerIDStrings(minAuthors...), pokerUserIDs(round.Outcome.Min.Authors), "%s: autores del mínimo", where)
	}
	if assert.NotNil(t, round.Outcome.Max, "%s: la divergencia debería señalar el máximo", where) {
		assert.Equal(t, maxValue, round.Outcome.Max.Value, "%s: valor máximo", where)
		assert.Equal(t, pokerIDStrings(maxAuthors...), pokerUserIDs(round.Outcome.Max.Authors), "%s: autores del máximo", where)
	}
}

// pokerStoredCurrentRound lee de estimation_rounds la ronda vigente de la sesión.
func pokerStoredCurrentRound(t *testing.T, db *gorm.DB, sessionID string) domain.EstimationRound {
	t.Helper()
	var round domain.EstimationRound
	require.NoError(t, db.Where("session_id = ?", sessionID).Order("number DESC").First(&round).Error,
		"no se encontró la ronda vigente de la sesión %s", sessionID)
	return round
}

// requireStoredRevealed exige que la ronda vigente quede revelada en la base, con revealed_at en el
// instante indicado del reloj falso.
func requireStoredRevealed(t *testing.T, db *gorm.DB, sessionID string, at time.Time) {
	t.Helper()
	round := pokerStoredCurrentRound(t, db, sessionID)
	assert.Equal(t, domain.RoundStatusRevealed, round.Status, "la ronda vigente debería quedar revelada en la base")
	if assert.NotNil(t, round.RevealedAt, "debería registrarse el momento de la revelación") {
		assert.True(t, round.RevealedAt.Equal(at), "revealed_at = %s, se esperaba %s", round.RevealedAt, at)
	}
}

// requireStoredVoting exige que la ronda vigente siga en votación en la base, sin momento de
// revelado.
func requireStoredVoting(t *testing.T, db *gorm.DB, sessionID, why string) {
	t.Helper()
	round := pokerStoredCurrentRound(t, db, sessionID)
	assert.Equal(t, domain.RoundStatusVoting, round.Status, "la ronda vigente debería seguir en votación: %s", why)
	assert.Nil(t, round.RevealedAt, "la ronda en votación no tiene momento de revelado: %s", why)
}

// pokerUS3Fixture es el "Dado" de cuatro participantes de US3: el de US2 con Diego sumado como
// integrante del proyecto y su canal abierto, los cuatro conectados y la ronda 1 en votación.
type pokerUS3Fixture struct {
	pokerUS2Fixture
	diego *pokerStream
}

// newPokerUS3Fixture prepara el "Dado" de cuatro participantes y espera a que cada canal vea a los
// cuatro conectados, para que la instantánea siguiente sea la de un voto.
func newPokerUS3Fixture(t *testing.T) pokerUS3Fixture {
	t.Helper()
	f := pokerUS3Fixture{pokerUS2Fixture: newPokerUS2Fixture(t)}
	f.env.addMember(t, f.anaToken, f.projectID, "diego@example.com")
	f.diego = f.env.openStream(t, f.diegoToken, f.session.ID)
	four := allConnected(f.anaID, f.brunoID, f.carlaID, f.diegoID)
	for name, stream := range f.streams() {
		stream.NextUntil(t, "los cuatro conectados en el canal de "+name, four)
	}
	return f
}

// streams devuelve los canales de los cuatro participantes por nombre.
func (f pokerUS3Fixture) streams() map[string]*pokerStream {
	return map[string]*pokerStream{"Ana": f.ana, "Bruno": f.bruno, "Carla": f.carla, "Diego": f.diego}
}

// threeStreams devuelve los canales de Ana, Bruno y Carla del "Dado" de US2 por nombre.
func (f pokerUS2Fixture) threeStreams() map[string]*pokerStream {
	return map[string]*pokerStream{"Ana": f.ana, "Bruno": f.bruno, "Carla": f.carla}
}

// Escenario: Revelación automática (005/US3)
// Con Ana, Bruno y Carla conectados, votan 3, 5 y 13: los dos primeros votos dejan la ronda en
// votación y, al emitirse el último, la ronda se revela sola en la misma transacción. La respuesta
// de Carla y los tres canales, sin recargar y en menos de 2 segundos, muestran el voto de cada uno,
// la divergencia con el mínimo 3 de Ana y el máximo 13 de Carla, y notVoted vacío; en la base la
// ronda queda revelada con revealed_at en el instante del reloj (FR-030, FR-031, FR-036, FR-038,
// FR-039, FR-073).
func TestUS3_RevelacionAutomatica(t *testing.T) {
	f := newPokerUS2Fixture(t)

	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	requireStoredVoting(t, f.env.db, f.session.ID, "falta el voto de Carla")
	last := f.env.requireVoted(t, f.carlaToken, f.session.ID, "13")

	// La respuesta del último voto ya trae la ronda revelada.
	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "5", f.carlaID: "13"}
	round := requireRevealedRound(t, last, votes, nil, "respuesta de Carla")
	requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, "respuesta de Carla")

	// Los tres canales reciben la revelación sin recargar.
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		round := requireRevealedRound(t, snapshot, votes, nil, "canal de "+name)
		requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, "canal de "+name)
	}
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Revelación manual con consenso (005/US3)
// Con Ana, Bruno, Carla y Diego conectados, los tres primeros votan 8 y Diego no vota: la ronda sigue
// en votación. Ana, facilitadora, revela: responde 200 con la ronda revelada, y los cuatro canales
// muestran los tres votos 8, a Diego en notVoted como "sin voto" y el consenso en 8 sin mínimo ni
// máximo; en la base la ronda queda revelada (FR-032, FR-035, FR-037, FR-040).
func TestUS3_RevelacionManualConConsenso(t *testing.T) {
	f := newPokerUS3Fixture(t)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "8")
	}
	requireStoredVoting(t, f.env.db, f.session.ID, "Diego todavía no votó")

	resp := f.env.reveal(t, f.anaToken, f.session.ID)

	var revealed pokerSessionBody
	requirePokerStatus(t, resp, http.StatusOK, &revealed)
	revealed.Raw = resp.Body
	votes := map[uuid.UUID]string{f.anaID: "8", f.brunoID: "8", f.carlaID: "8"}
	requireConsensus(t, requireRevealedRound(t, revealed, votes, []uuid.UUID{f.diegoID}, "respuesta de Ana"), 8, "respuesta de Ana")

	// Los cuatro canales reciben la revelación con Diego sin voto.
	for name, stream := range f.streams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		round := requireRevealedRound(t, snapshot, votes, []uuid.UUID{f.diegoID}, "canal de "+name)
		requireConsensus(t, round, 8, "canal de "+name)
	}
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Interrogación fuera del cálculo (005/US3)
// Ana y Bruno votan 5 y Carla vota "?": al completarse los votos la ronda se revela sola; los tres
// canales muestran el "?" de Carla tal como se jugó y el consenso en 5, porque el "?" no participa
// del cálculo (FR-036, FR-037, RN-12).
func TestUS3_InterrogacionFueraDelCalculo(t *testing.T) {
	f := newPokerUS2Fixture(t)

	f.env.requireVoted(t, f.anaToken, f.session.ID, "5")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "?")

	votes := map[uuid.UUID]string{f.anaID: "5", f.brunoID: "5", f.carlaID: "?"}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		requireConsensus(t, requireRevealedRound(t, snapshot, votes, nil, "canal de "+name), 5, "canal de "+name)
	}
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Consenso sin extremos (005/US3)
// Ana, Bruno y Carla votan 20: la ronda se revela sola con consenso en 20 y sin mínimo ni máximo,
// porque no hay diferencias que explicar (FR-037, FR-040, RN-13).
func TestUS3_ConsensoSinExtremos(t *testing.T) {
	f := newPokerUS2Fixture(t)

	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "20")
	}

	votes := map[uuid.UUID]string{f.anaID: "20", f.brunoID: "20", f.carlaID: "20"}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		requireConsensus(t, requireRevealedRound(t, snapshot, votes, nil, "canal de "+name), 20, "canal de "+name)
	}
}

// Escenario: Único participante (005/US3)
// Ana abre la sesión y nadie más se une: al votar 13, la ronda se revela sola porque votó el único
// conectado; la respuesta y su canal muestran su voto y el consenso en 13, y en la base la ronda
// queda revelada (FR-030, RN-09, RN-23, SC-006).
func TestUS3_UnicoParticipante(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	stream := f.env.openStream(t, f.anaToken, session.ID)
	stream.NextUntil(t, "Ana conectada", allConnected(f.anaID))

	resp := f.env.requireVoted(t, f.anaToken, session.ID, "13")

	votes := map[uuid.UUID]string{f.anaID: "13"}
	requireConsensus(t, requireRevealedRound(t, resp, votes, nil, "respuesta de Ana"), 13, "respuesta de Ana")
	snapshot := stream.NextUntil(t, "la ronda revelada en el canal de Ana", currentRoundRevealed)
	requireConsensus(t, requireRevealedRound(t, snapshot, votes, nil, "canal de Ana"), 13, "canal de Ana")
	requireStoredRevealed(t, f.env.db, session.ID, f.env.clock.Now())
}

// Escenario: Todos interrogación (005/US3)
// Ana, Bruno y Carla votan "?": la ronda se revela sola sin consenso, sin valor y sin mínimo ni
// máximo, y el resultado indica que no hubo ningún voto numérico (FR-041, RN-14).
func TestUS3_TodosInterrogacion(t *testing.T) {
	f := newPokerUS2Fixture(t)

	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "?")
	}

	votes := map[uuid.UUID]string{f.anaID: "?", f.brunoID: "?", f.carlaID: "?"}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		round := requireRevealedRound(t, snapshot, votes, nil, "canal de "+name)
		if assert.NotNil(t, round.Outcome, "canal de %s: la ronda revelada debería tener resultado", name) {
			assert.Equal(t, "no_numeric_votes", round.Outcome.Kind, "canal de %s: tipo de resultado", name)
			assert.Nil(t, round.Outcome.Value, "canal de %s: sin votos numéricos no hay valor", name)
			assert.Nil(t, round.Outcome.Min, "canal de %s: sin votos numéricos no hay mínimo", name)
			assert.Nil(t, round.Outcome.Max, "canal de %s: sin votos numéricos no hay máximo", name)
		}
	}
}

// Escenario: Desconexión dispara la revelación (005/US3)
// Con Ana, Bruno y Carla conectados que votaron 3, 5 y 8, y Diego conectado sin votar, Diego cierra
// su canal: mientras dura la gracia la ronda sigue en votación. Al avanzar el reloj 30 segundos el
// barrido registra su desconexión y, en la misma transacción, la ronda se revela sola porque todos
// los que siguen conectados ya votaron; los canales de Ana, Bruno y Carla muestran los tres votos, la
// divergencia de 3 a 8 y notVoted vacío, porque Diego ya no está conectado (FR-014, FR-030, FR-031,
// RN-09).
func TestUS3_DesconexionDisparaRevelacion(t *testing.T) {
	f := newPokerUS3Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "8")

	// Diego cierra su canal sin votar; hasta que vence la gracia la ronda no se revela.
	f.diego.Close()
	requireStoredVoting(t, f.env.db, f.session.ID, "Diego sigue dentro de la gracia")
	f.env.sweep(t, 30*time.Second)

	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "5", f.carlaID: "8"}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		round := requireRevealedRound(t, snapshot, votes, nil, "canal de "+name)
		requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 8, []uuid.UUID{f.carlaID}, "canal de "+name)
		if p := snapshot.participant(f.diegoID); assert.NotNil(t, p, "canal de %s: Diego sigue en la lista", name) {
			assert.False(t, p.Connected, "canal de %s: Diego figura desconectado", name)
		}
	}
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Autores múltiples (005/US3)
// Con los cuatro conectados, Ana y Carla votan 3 y Bruno y Diego votan 40: la ronda se revela sola y
// los cuatro canales señalan como autores del mínimo a Ana y Carla y como autores del máximo a Bruno
// y Diego, ordenados por nombre (FR-039).
func TestUS3_AutoresMultiples(t *testing.T) {
	f := newPokerUS3Fixture(t)

	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "40")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.diegoToken, f.session.ID, "40")

	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "40", f.carlaID: "3", f.diegoID: "40"}
	for name, stream := range f.streams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		round := requireRevealedRound(t, snapshot, votes, nil, "canal de "+name)
		requireDivergence(t, round,
			3, []uuid.UUID{f.anaID, f.carlaID},
			40, []uuid.UUID{f.brunoID, f.diegoID},
			"canal de "+name)
	}
}

// Escenario: Revelar sin votos (005/US3)
// Con Ana, Bruno y Carla conectados y ningún voto emitido, Ana, facilitadora, intenta revelar:
// responde 409 con el mensaje de que hace falta al menos un voto, la ronda sigue en votación y la
// versión de la sesión no cambia, porque el rechazo no es un hecho (FR-033, RN-10).
func TestUS3_RevelarSinVotos(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.reveal(t, f.anaToken, f.session.ID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerNoVotesToRevealBodyJSON, string(resp.Body))
	requireStoredVoting(t, f.env.db, f.session.ID, "el rechazo no revela")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// Escenario: Revelar sin ser facilitador (005/US3)
// Con Ana y Bruno ya votados y Carla conectada sin votar, Carla, que no es la facilitadora, intenta
// revelar: responde 403 con el mensaje de que la acción está reservada al facilitador, la ronda sigue
// en votación, la versión no cambia y una conexión nueva de Carla sigue viendo los votos ocultos
// (FR-034, FR-064, SC-011).
func TestUS3_RevelarSinSerFacilitador(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "5")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.reveal(t, f.carlaToken, f.session.ID)

	assert.Equal(t, http.StatusForbidden, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerFacilitatorOnlyBodyJSON, string(resp.Body))
	requireStoredVoting(t, f.env.db, f.session.ID, "el rechazo no revela")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")

	// Una conexión nueva de Carla recibe la ronda en votación con los votos ocultos.
	second := f.env.openStream(t, f.carlaToken, f.session.ID)
	both := func(s pokerSessionBody) bool { return hasVoted(f.anaID)(s) && hasVoted(f.brunoID)(s) }
	snapshot := second.NextUntil(t, "Ana y Bruno ya votaron en la conexión nueva de Carla", both)
	requireSecretVote(t, snapshot, f.anaID, "Carla")
	requireSecretVote(t, snapshot, f.brunoID, "Carla")
}

// Escenario: Ronda ya revelada (005/US3)
// Ana, Bruno y Carla votan 3, 5 y 8 y la ronda se revela sola. Pasado un minuto, Ana, facilitadora,
// intenta revelarla otra vez: responde 409 con el mensaje de que la ronda ya fue revelada, la ronda
// sigue revelada con el mismo momento de revelado y la versión de la sesión no
// cambia, porque el rechazo no es un hecho.
func TestUS3_RevelarRondaYaRevelada(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "8")
	revealedAt := f.env.clock.Now()
	requireStoredRevealed(t, f.env.db, f.session.ID, revealedAt)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	// El reloj avanza para que una segunda revelación aceptada se notara en revealed_at.
	f.env.clock.Advance(time.Minute)
	resp := f.env.reveal(t, f.anaToken, f.session.ID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerAlreadyRevealedBodyJSON, string(resp.Body))
	requireStoredRevealed(t, f.env.db, f.session.ID, revealedAt)
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// pokerSend envía una petición sin usar require, porque corre en una goroutine distinta de la de la
// prueba y ahí no se puede llamar a FailNow; el error de transporte vuelve como valor.
func (e *pokerTestEnv) pokerSend(method, path, token string, body []byte) pokerConcurrentResult {
	req, err := http.NewRequest(method, e.server.URL+path, bytes.NewReader(body))
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: pokerHTTPTimeout}).Do(req)
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return pokerConcurrentResult{err: err}
	}
	return pokerConcurrentResult{resp: pokerResponse{
		Code:        resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        data,
	}}
}

// Escenario: Cambio simultáneo con revelación (005/US2)
// Bruno ya votó 5 y Carla 8, y Ana todavía no votó. Bruno cambia su carta a 13 en el mismo instante en
// que Ana, facilitadora, revela: la revelación responde 200 y el cambio 200 si llegó antes o 409 si
// la ronda ya estaba revelada. Los tres canales reciben la misma instantánea revelada, con la misma
// versión y el mismo conjunto de votos: el 13 de Bruno si su cambio fue aceptado y el 5 si fue
// rechazado, igual que en la base (FR-031, RN-11, SC-007).
func TestUS2_CambioSimultaneoConRevelacion(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "8")
	payload, err := json.Marshal(map[string]any{"card": "13"})
	require.NoError(t, err)

	// El cambio de Bruno y la revelación de Ana salen a la vez tras la misma señal.
	var change, reveal pokerConcurrentResult
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		change = f.env.pokerSend(http.MethodPut, votePath(f.session.ID), f.brunoToken, payload)
	})
	wg.Go(func() {
		<-start
		reveal = f.env.pokerSend(http.MethodPost, revealPath(f.session.ID), f.anaToken, nil)
	})
	close(start)
	wg.Wait()

	// La revelación se acepta y el cambio queda aceptado o rechazado por la ronda revelada.
	require.NoError(t, reveal.err, "la revelación no obtuvo respuesta")
	require.NoError(t, change.err, "el cambio de voto no obtuvo respuesta")
	require.Equal(t, http.StatusOK, reveal.resp.Code, "revelación; cuerpo: %s", reveal.resp.Body)
	brunoCard := "5"
	switch change.resp.Code {
	case http.StatusOK:
		brunoCard = "13"
	case http.StatusConflict:
		assert.JSONEq(t, pokerRoundRevealedBodyJSON, string(change.resp.Body))
	default:
		t.Fatalf("cambio de voto: código %d inesperado; cuerpo: %s", change.resp.Code, change.resp.Body)
	}

	// Los tres canales ven la misma revelación, con la carta de Bruno que corresponde al resultado.
	votes := map[uuid.UUID]string{f.brunoID: brunoCard, f.carlaID: "8"}
	versions := map[string]int64{}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
		requireRevealedRound(t, snapshot, votes, []uuid.UUID{f.anaID}, "canal de "+name)
		versions[name] = snapshot.Version
	}
	assert.Equal(t, versions["Ana"], versions["Bruno"], "Ana y Bruno deberían ver la misma versión revelada")
	assert.Equal(t, versions["Ana"], versions["Carla"], "Ana y Carla deberían ver la misma versión revelada")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, brunoCard)
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Pruebas de integración de punta a punta de la historia 4 de Planning Poker (T056): abren una ronda
// nueva sobre una revelada con POST /api/v1/estimation-sessions/{sessionId}/rounds y consultan la
// sesión con todas sus rondas con GET /api/v1/estimation-sessions/{sessionId}, sobre el mismo router
// compuesto como cmd/app y contra smye_test. Verifican la respuesta, las instantáneas que llegan sin
// recargar por los canales ya abiertos y las filas de estimation_rounds y estimation_votes. El "Dado"
// habitual es la ronda 1 revelada sola con los votos 3, 5 y 13 de Ana, Bruno y Carla. También va acá,
// sin etiqueta de historia, la incorporación sobre una ronda ya revelada de los Casos Límite.

// pokerRoundNotRevealedBodyJSON es el cuerpo exacto que fija el contrato para iniciar una ronda con
// la vigente todavía en votación.
const pokerRoundNotRevealedBodyJSON = `{"error":"Primero hay que revelar la ronda vigente."}`

// roundsPath es la ruta del inicio de una ronda nueva según contracts/openapi.yaml.
func roundsPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/rounds"
}

// sessionPath es la ruta de la consulta de una sesión con todas sus rondas según
// contracts/openapi.yaml.
func sessionPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID
}

// startRound pide POST /api/v1/estimation-sessions/{sessionId}/rounds, sin cuerpo, con el token como
// Bearer.
func (e *pokerTestEnv) startRound(t *testing.T, token, sessionID string) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, roundsPath(sessionID), token, nil)
}

// requireStartedRound inicia una ronda nueva, exige el 201 y devuelve la sesión resultante vista por
// quien la pidió.
func (e *pokerTestEnv) requireStartedRound(t *testing.T, token, sessionID string) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.startRound(t, token, sessionID)
	requirePokerStatus(t, resp, http.StatusCreated, &session)
	session.Raw = resp.Body
	return session
}

// requireSessionFetched pide GET /api/v1/estimation-sessions/{sessionId}, exige el 200 y devuelve la
// sesión vista por quien la pidió.
func (e *pokerTestEnv) requireSessionFetched(t *testing.T, token, sessionID string) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.do(t, http.MethodGet, sessionPath(sessionID), token, nil)
	requirePokerStatus(t, resp, http.StatusOK, &session)
	session.Raw = resp.Body
	return session
}

// pokerAllRounds decodifica del JSON recibido todas las rondas de la sesión, de la 1 a la vigente,
// con sus votos, su resultado y quienes no votaron.
func pokerAllRounds(t *testing.T, snapshot pokerSessionBody) []pokerRevealedRoundBody {
	t.Helper()
	var raw struct {
		Rounds []pokerRevealedRoundBody `json:"rounds"`
	}
	require.NoError(t, json.Unmarshal(snapshot.Raw, &raw), "el cuerpo no es JSON válido: %s", snapshot.Raw)
	return raw.Rounds
}

// roundVoting arma la condición "la ronda vigente es la del número indicado y está en votación",
// para esperar con NextUntil.
func roundVoting(number int) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		n := len(s.Rounds)
		return n > 0 && s.Rounds[n-1].Number == number && s.Rounds[n-1].Status == "voting"
	}
}

// divergentCards devuelve los votos 3, 5 y 13 de Ana, Bruno y Carla del "Dado" de US4, indexados por
// el identificador de su autor como texto, para compararlos con pokerRevealedCards.
func (f pokerUS2Fixture) divergentCards() map[string]string {
	return map[string]string{f.anaID.String(): "3", f.brunoID.String(): "5", f.carlaID.String(): "13"}
}

// revealDivergentRound hace votar 3, 5 y 13 a Ana, Bruno y Carla, con lo que la ronda vigente se
// revela sola, y espera a que los tres canales reciban la revelación.
func (f pokerUS2Fixture) revealDivergentRound(t *testing.T) {
	t.Helper()
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "13")
	for name, stream := range f.threeStreams() {
		stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
	}
}

// requireFreshVotingRound exige que la ronda vigente de la instantánea sea la del número indicado,
// en votación, sin votos visibles, que nadie figure como que ya votó y que quien mira no tenga carta
// en myVote (FR-044, FR-045).
func requireFreshVotingRound(t *testing.T, snapshot pokerSessionBody, number int, where string) {
	t.Helper()
	require.NotEmpty(t, snapshot.Rounds, "%s: rondas: %s", where, snapshot.Raw)
	current := snapshot.Rounds[len(snapshot.Rounds)-1]
	assert.Equal(t, number, current.Number, "%s: número de la ronda vigente", where)
	assert.Equal(t, "voting", current.Status, "%s: la ronda vigente debería estar en votación", where)
	assert.Nil(t, current.RevealedAt, "%s: la ronda nueva no tiene momento de revelado", where)
	assert.JSONEq(t, `null`, pokerCurrentRoundVotes(t, snapshot), "%s: la ronda nueva no muestra votos", where)
	assert.Nil(t, snapshot.MyVote, "%s: nadie tiene voto en la ronda nueva", where)
	for _, p := range snapshot.Participants {
		assert.False(t, p.HasVoted, "%s: %s no debería figurar como que ya votó", where, p.User.FullName)
	}
}

// pokerStoredRounds lee de estimation_rounds todas las rondas de la sesión, ordenadas por número.
func pokerStoredRounds(t *testing.T, db *gorm.DB, sessionID string) []domain.EstimationRound {
	t.Helper()
	var rounds []domain.EstimationRound
	require.NoError(t, db.Where("session_id = ?", sessionID).Order("number").Find(&rounds).Error,
		"no se pudieron leer las rondas de la sesión %s", sessionID)
	return rounds
}

// pokerStoredRoundVotes cuenta las filas de estimation_votes de la ronda indicada.
func pokerStoredRoundVotes(t *testing.T, db *gorm.DB, roundID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.EstimationVote{}).Where("round_id = ?", roundID).Count(&n).Error,
		"no se pudieron contar los votos de la ronda %s", roundID)
	return n
}

// Escenario: Nueva ronda (005/US4)
// Con la ronda 1 revelada con los votos 3, 5 y 13, un minuto después Ana, facilitadora, inicia una
// ronda nueva: responde 201 con la ronda 2 en votación, sin votos visibles y con nadie como que ya
// votó; los tres canales reciben el cambio sin recargar y en menos de 2 segundos. En la base la ronda
// 1 sigue revelada con sus tres votos, la ronda 2 queda en votación sin votos y abierta en el
// instante del reloj, y la versión de la sesión sube en uno (FR-043, FR-044, FR-045, FR-073).
func TestUS4_NuevaRonda(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)
	f.env.clock.Advance(time.Minute)

	resp := f.env.requireStartedRound(t, f.anaToken, f.session.ID)

	// La respuesta y los tres canales muestran la ronda 2 limpia.
	require.Len(t, resp.Rounds, 2, "rondas: %s", resp.Raw)
	requireFreshVotingRound(t, resp, 2, "respuesta de Ana")
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda 2 en el canal de "+name, roundVoting(2))
		requireFreshVotingRound(t, snapshot, 2, "canal de "+name)
	}

	// La ronda 1 queda en la base con sus votos y la 2 se abre vacía en el instante del reloj.
	rounds := pokerStoredRounds(t, f.env.db, f.session.ID)
	require.Len(t, rounds, 2, "la sesión debería tener dos rondas en la base")
	assert.Equal(t, domain.RoundStatusRevealed, rounds[0].Status, "la ronda 1 sigue revelada")
	assert.EqualValues(t, 3, pokerStoredRoundVotes(t, f.env.db, rounds[0].ID), "la ronda 1 conserva sus tres votos")
	assert.Equal(t, 2, rounds[1].Number)
	assert.Equal(t, domain.RoundStatusVoting, rounds[1].Status)
	assert.Nil(t, rounds[1].RevealedAt)
	assert.True(t, rounds[1].CreatedAt.Equal(f.env.clock.Now()), "created_at = %s, se esperaba %s", rounds[1].CreatedAt, f.env.clock.Now())
	assert.Zero(t, pokerStoredRoundVotes(t, f.env.db, rounds[1].ID), "la ronda 2 nace sin votos")
	assert.Equal(t, before+1, pokerStoredVersion(t, f.env.db, f.session.ID), "iniciar una ronda es un hecho y sube la versión")
}

// Escenario: Historial de rondas (005/US4)
// Con la ronda 2 en curso y el voto 8 de Ana todavía oculto, Bruno consulta la sesión con GET
// /api/v1/estimation-sessions/{sessionId}: responde 200 con las dos rondas; la 1 revelada con el voto
// de cada participante y la divergencia de 3 (Ana) a 13 (Carla), sin notVoted porque ya no es la
// vigente, y la 2 en votación sin votos visibles, sin resultado y sin carta propia de Bruno (FR-044,
// FR-078, FR-082).
func TestUS4_HistorialDeRondas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "8")

	session := f.env.requireSessionFetched(t, f.brunoToken, f.session.ID)

	rounds := pokerAllRounds(t, session)
	require.Len(t, rounds, 2, "rondas: %s", session.Raw)

	// La ronda 1 se consulta completa, con sus votos y su resultado.
	first := rounds[0]
	assert.Equal(t, 1, first.Number)
	assert.Equal(t, "revealed", first.Status)
	assert.NotNil(t, first.RevealedAt, "la ronda 1 conserva su momento de revelado")
	assert.Equal(t, f.divergentCards(), pokerRevealedCards(first), "votos de la ronda 1: %s", session.Raw)
	requireDivergence(t, first, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, "ronda 1 del historial")
	assert.Empty(t, first.NotVoted, "notVoted solo se informa en la ronda vigente")

	// La ronda 2 sigue secreta para Bruno, aunque Ana ya votó.
	second := rounds[1]
	assert.Equal(t, 2, second.Number)
	assert.Equal(t, "voting", second.Status)
	assert.JSONEq(t, `null`, pokerCurrentRoundVotes(t, session), "la ronda en votación no muestra votos")
	assert.Nil(t, second.Outcome, "la ronda en votación no tiene resultado")
	assert.Nil(t, session.MyVote, "Bruno no votó en la ronda 2")
	if p := session.participant(f.anaID); assert.NotNil(t, p, "falta Ana: %s", session.Raw) {
		assert.True(t, p.HasVoted, "Bruno debería ver que Ana ya votó en la ronda 2")
	}
}

// Escenario: Nueva ronda tras consenso (005/US4)
// Ana, Bruno y Carla votan 8 y la ronda 1 se revela sola con consenso en 8. Ana, facilitadora, inicia
// una ronda nueva de todos modos: responde 201 con la ronda 2 en votación y la sesión sigue abierta
// sin valor acordado, porque el consenso no obliga a cerrarla (FR-043, Casos Límite).
func TestUS4_NuevaRondaTrasConsenso(t *testing.T) {
	f := newPokerUS2Fixture(t)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "8")
	}
	f.ana.NextUntil(t, "la ronda revelada en el canal de Ana", currentRoundRevealed)

	resp := f.env.requireStartedRound(t, f.anaToken, f.session.ID)

	assert.Equal(t, "open", resp.Status, "la sesión sigue abierta")
	assert.Nil(t, resp.AgreedStoryPoints, "el consenso no registra ningún valor")
	requireFreshVotingRound(t, resp, 2, "respuesta de Ana")
	rounds := pokerAllRounds(t, resp)
	if assert.Len(t, rounds, 2, "rondas: %s", resp.Raw) {
		requireConsensus(t, rounds[0], 8, "ronda 1")
	}
	assert.Len(t, pokerStoredRounds(t, f.env.db, f.session.ID), 2, "la sesión debería tener dos rondas en la base")
}

// Escenario: Incorporación en ronda posterior (005/US4)
// Con la ronda 2 en curso, Diego, integrante del proyecto que no había participado, abre su canal:
// recibe la sesión con él como participante conectado de la ronda 2 en votación, sin voto y sin
// ningún voto suyo en la ronda 1. Al votar 5 responde 200 con su carta en myVote, Ana lo ve como que
// ya votó y en la base su voto queda en la ronda 2 (FR-011, FR-017).
func TestUS4_IncorporacionEnRondaPosterior(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.env.addMember(t, f.anaToken, f.projectID, "diego@example.com")

	diego := f.env.openStream(t, f.diegoToken, f.session.ID)

	// Diego queda conectado en la ronda 2, sin voto en ninguna ronda.
	four := allConnected(f.anaID, f.brunoID, f.carlaID, f.diegoID)
	joined := diego.NextUntil(t, "Diego conectado en la ronda 2", func(s pokerSessionBody) bool { return four(s) && roundVoting(2)(s) })
	requireFreshVotingRound(t, joined, 2, "canal de Diego")
	rounds := pokerAllRounds(t, joined)
	require.Len(t, rounds, 2, "rondas: %s", joined.Raw)
	assert.NotContains(t, pokerRevealedCards(rounds[0]), f.diegoID.String(), "Diego no tiene voto en la ronda 1")

	// Diego vota en la ronda 2 y Ana lo ve como que ya votó.
	requireOwnVote(t, f.env.requireVoted(t, f.diegoToken, f.session.ID, "5"), f.diegoID, "5", "respuesta de Diego")
	requireSecretVote(t, f.ana.NextUntil(t, "Diego ya votó en el canal de Ana", hasVoted(f.diegoID)), f.diegoID, "Ana")
	votes := pokerStoredVotes(t, f.env.db, f.session.ID, f.diegoID)
	if assert.Len(t, votes, 1, "Diego debería tener exactamente un voto") {
		assert.Equal(t, pokerStoredCurrentRound(t, f.env.db, f.session.ID).ID, votes[0].RoundID, "el voto de Diego es de la ronda 2")
	}
}

// Escenario: Sin límite de rondas (005/US4)
// Ana, Bruno y Carla repiten diez rondas sin consenso, con los votos 3, 5 y 13 en cada una. Con la
// ronda 10 revelada, Ana, facilitadora, inicia la ronda 11: responde 201 porque no hay límite de
// rondas, y la sesión muestra las rondas 1 a 10 completas, cada una revelada con sus tres votos y la
// divergencia de 3 a 13; en la base hay once rondas y treinta votos (FR-048, Casos Límite).
func TestUS4_SinLimiteDeRondas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	for number := 2; number <= 10; number++ {
		f.env.requireStartedRound(t, f.anaToken, f.session.ID)
		f.revealDivergentRound(t)
	}

	resp := f.env.requireStartedRound(t, f.anaToken, f.session.ID)

	// La ronda 11 se abre y las diez anteriores siguen completas.
	rounds := pokerAllRounds(t, resp)
	require.Len(t, rounds, 11, "rondas: %s", resp.Raw)
	requireFreshVotingRound(t, resp, 11, "respuesta de Ana")
	for i, round := range rounds[:10] {
		where := fmt.Sprintf("ronda %d", i+1)
		assert.Equal(t, i+1, round.Number, "%s: número", where)
		assert.Equal(t, "revealed", round.Status, "%s: estado", where)
		assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "%s: votos", where)
		requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, where)
	}

	// En la base quedan las once rondas y los treinta votos.
	assert.Len(t, pokerStoredRounds(t, f.env.db, f.session.ID), 11, "la sesión debería tener once rondas en la base")
	var votes int64
	require.NoError(t, f.env.db.Model(&domain.EstimationVote{}).
		Joins("JOIN estimation_rounds ON estimation_rounds.id = estimation_votes.round_id").
		Where("estimation_rounds.session_id = ?", f.session.ID).Count(&votes).Error)
	assert.EqualValues(t, 30, votes, "las diez rondas conservan sus votos")
}

// Escenario: Facilitador solo en nueva ronda (005/US4)
// Con la ronda 1 revelada, Bruno y Carla cierran su canal y, al avanzar el reloj 30 segundos, el
// barrido registra su desconexión. Ana, facilitadora y única conectada, inicia una ronda nueva:
// responde 201 con la ronda 2 en votación y Ana como única participante conectada; su canal recibe la
// ronda 2 sin recargar (FR-014, FR-043).
func TestUS4_FacilitadorSoloEnNuevaRonda(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.bruno.Close()
	f.carla.Close()
	f.env.sweep(t, 30*time.Second)
	f.ana.NextUntil(t, "Bruno y Carla desconectados en el canal de Ana", func(s pokerSessionBody) bool {
		b, c := s.participant(f.brunoID), s.participant(f.carlaID)
		return b != nil && c != nil && !b.Connected && !c.Connected
	})

	resp := f.env.requireStartedRound(t, f.anaToken, f.session.ID)

	requireFreshVotingRound(t, resp, 2, "respuesta de Ana")
	connected := []string{}
	for _, p := range resp.Participants {
		if p.Connected {
			connected = append(connected, p.User.ID)
		}
	}
	assert.Equal(t, []string{f.anaID.String()}, connected, "Ana debería ser la única conectada: %s", resp.Raw)
	requireFreshVotingRound(t, f.ana.NextUntil(t, "la ronda 2 en el canal de Ana", roundVoting(2)), 2, "canal de Ana")
}

// Escenario: Ronda sin revelar (005/US4)
// Con la ronda 1 en votación y el voto 5 de Bruno todavía oculto, Ana, facilitadora, intenta iniciar
// una ronda nueva: responde 409 con el mensaje de que primero hay que revelar la ronda vigente; la
// sesión sigue con una sola ronda en votación, el voto de Bruno se conserva y la versión no cambia,
// porque el rechazo no es un hecho (FR-046).
func TestUS4_RondaSinRevelar(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.startRound(t, f.anaToken, f.session.ID)

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerRoundNotRevealedBodyJSON, string(resp.Body))
	assert.Len(t, pokerStoredRounds(t, f.env.db, f.session.ID), 1, "el rechazo no abre ninguna ronda")
	requireStoredVoting(t, f.env.db, f.session.ID, "el rechazo no revela")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "5")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// Escenario: Nueva ronda sin ser facilitador (005/US4)
// Con la ronda 1 revelada, Bruno, que no es el facilitador, intenta iniciar una ronda nueva: responde
// 403 con el mensaje de que la acción está reservada al facilitador; la sesión sigue con la ronda 1
// revelada como vigente y la versión no cambia (FR-047, FR-064, SC-011).
func TestUS4_NuevaRondaSinSerFacilitador(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.startRound(t, f.brunoToken, f.session.ID)

	assert.Equal(t, http.StatusForbidden, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerFacilitatorOnlyBodyJSON, string(resp.Body))
	rounds := pokerStoredRounds(t, f.env.db, f.session.ID)
	if assert.Len(t, rounds, 1, "el rechazo no abre ninguna ronda") {
		assert.Equal(t, domain.RoundStatusRevealed, rounds[0].Status, "la ronda 1 sigue revelada")
	}
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// TestPokerJoin_RondaRevelada cubre el caso límite de incorporarse sobre una ronda ya revelada
// (Casos Límite, FR-017). Con la ronda 1 revelada con los votos 3, 5 y 13, Diego, integrante del
// proyecto que no había participado, abre su canal: recibe la ronda 1 revelada con los tres votos y
// la divergencia de 3 a 13. Su voto responde 409 con el mensaje de ronda revelada y no queda escrito.
// Cuando Ana inicia la ronda nueva, el canal de Diego recibe la ronda 2 en votación y su voto 8
// responde 200 con su carta en myVote.
func TestPokerJoin_RondaRevelada(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.env.addMember(t, f.anaToken, f.projectID, "diego@example.com")

	diego := f.env.openStream(t, f.diegoToken, f.session.ID)

	// Diego ve los votos revelados y el resultado de la ronda 1.
	joined := diego.NextUntil(t, "Diego conectado con la ronda revelada", func(s pokerSessionBody) bool {
		p := s.participant(f.diegoID)
		return p != nil && p.Connected && currentRoundRevealed(s)
	})
	round := pokerCurrentRound(t, joined)
	assert.Equal(t, 1, round.Number, "la ronda vigente sigue siendo la 1")
	assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "Diego debería ver los votos revelados: %s", joined.Raw)
	requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, "canal de Diego")

	// En la ronda revelada Diego no puede votar.
	rejected := f.env.vote(t, f.diegoToken, f.session.ID, map[string]any{"card": "8"})
	assert.Equal(t, http.StatusConflict, rejected.Code, "cuerpo: %s", rejected.Body)
	assert.JSONEq(t, pokerRoundRevealedBodyJSON, string(rejected.Body))
	assert.Empty(t, pokerStoredVotes(t, f.env.db, f.session.ID, f.diegoID), "el voto rechazado no queda escrito")

	// Con la ronda nueva, Diego participa y vota.
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	requireFreshVotingRound(t, diego.NextUntil(t, "la ronda 2 en el canal de Diego", roundVoting(2)), 2, "canal de Diego")
	requireOwnVote(t, f.env.requireVoted(t, f.diegoToken, f.session.ID, "8"), f.diegoID, "8", "respuesta de Diego")
}

// Pruebas de integración de punta a punta de la historia 5 de Planning Poker (T063): registran la
// estimación acordada con POST /api/v1/estimation-sessions/{sessionId}/finalize sobre el mismo router
// compuesto como cmd/app y contra smye_test. Verifican la respuesta, el último estado que llega por
// los canales ya abiertos antes de que el servidor los cierre, la sesión guardada en
// estimation_sessions y los Story Points de la historia leídos con GET /api/v1/stories/{storyId} de
// la 003, porque registrar escribe en el backlog en la misma operación (FR-049 a FR-054, FR-075).

// Textos exactos que fija el contrato para los rechazos del registro.
const (
	pokerAgreedValueMessage      = "El valor acordado debe ser uno de 0, 1, 2, 3, 5, 8, 13, 20, 40 o 100."
	pokerNoRevealedRoundBodyJSON = `{"error":"Primero hay que revelar al menos una ronda o cancelar la sesión."}`
)

// finalizePath es la ruta del registro de la estimación acordada según contracts/openapi.yaml.
func finalizePath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/finalize"
}

// finalize pide POST /api/v1/estimation-sessions/{sessionId}/finalize con el cuerpo indicado y el
// token como Bearer.
func (e *pokerTestEnv) finalize(t *testing.T, token, sessionID string, body any) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, finalizePath(sessionID), token, body)
}

// requireFinalized registra el valor indicado, exige el 200 y devuelve la sesión resultante vista por
// quien registró.
func (e *pokerTestEnv) requireFinalized(t *testing.T, token, sessionID string, storyPoints int) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.finalize(t, token, sessionID, map[string]any{"storyPoints": storyPoints})
	requirePokerStatus(t, resp, http.StatusOK, &session)
	session.Raw = resp.Body
	return session
}

// pokerStoryDetailBody refleja, del StoryDetail de la 003, los Story Points y la condición de lista
// para planificar.
type pokerStoryDetailBody struct {
	StoryPoints *int `json:"storyPoints"`
	Readiness   struct {
		Ready   bool     `json:"ready"`
		Missing []string `json:"missing"`
	} `json:"readiness"`
}

// requireStoryDetail pide GET /api/v1/stories/{storyId} de la 003, exige el 200 y devuelve el
// detalle de la historia.
func (e *pokerTestEnv) requireStoryDetail(t *testing.T, token string, storyID uuid.UUID) pokerStoryDetailBody {
	t.Helper()
	var detail pokerStoryDetailBody
	requirePokerStatus(t, e.do(t, http.MethodGet, "/api/v1/stories/"+storyID.String(), token, nil), http.StatusOK, &detail)
	return detail
}

// requireStoryEstimated reestima la historia directamente desde el backlog con PUT
// /api/v1/stories/{storyId}/story-points de la 003 y exige el 200.
func (e *pokerTestEnv) requireStoryEstimated(t *testing.T, token string, storyID uuid.UUID, storyPoints int) {
	t.Helper()
	requirePokerStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/story-points", token,
		map[string]any{"storyPoints": storyPoints}), http.StatusOK, nil)
}

// requireStoryPoints exige que el detalle de la historia de la 003 muestre los Story Points
// indicados; nil es "sin estimar".
func requireStoryPoints(t *testing.T, detail pokerStoryDetailBody, want *int, where string) {
	t.Helper()
	if want == nil {
		assert.Nil(t, detail.StoryPoints, "%s: la historia debería seguir sin estimar", where)
		return
	}
	if assert.NotNil(t, detail.StoryPoints, "%s: la historia debería quedar estimada", where) {
		assert.Equal(t, *want, *detail.StoryPoints, "%s: Story Points de la historia", where)
	}
}

// sessionFinished es la condición "la sesión está finalizada", para esperar con NextUntil.
func sessionFinished(s pokerSessionBody) bool {
	return s.Status == "finished"
}

// requireFinishedSession exige que la instantánea muestre la sesión finalizada con el valor acordado,
// cerrada por la persona indicada y con su momento de cierre.
func requireFinishedSession(t *testing.T, snapshot pokerSessionBody, agreed int, closedBy uuid.UUID, where string) {
	t.Helper()
	assert.Equal(t, "finished", snapshot.Status, "%s: estado de la sesión: %s", where, snapshot.Raw)
	if assert.NotNil(t, snapshot.AgreedStoryPoints, "%s: agreedStoryPoints no debería ser null", where) {
		assert.Equal(t, agreed, *snapshot.AgreedStoryPoints, "%s: valor acordado", where)
	}
	if assert.NotNil(t, snapshot.ClosedBy, "%s: closedBy no debería ser null", where) {
		assert.Equal(t, closedBy.String(), snapshot.ClosedBy.ID, "%s: quien cerró la sesión", where)
	}
	assert.NotNil(t, snapshot.ClosedAt, "%s: closedAt no debería ser null", where)
	assert.Nil(t, snapshot.CancelReason, "%s: una sesión finalizada no tiene motivo de cancelación", where)
}

// pokerStoredSession lee de estimation_sessions la fila de la sesión.
func pokerStoredSession(t *testing.T, db *gorm.DB, sessionID string) domain.EstimationSession {
	t.Helper()
	var stored domain.EstimationSession
	require.NoError(t, db.First(&stored, "id = ?", sessionID).Error, "no se encontró la sesión %s", sessionID)
	return stored
}

// requireStoredFinished exige que la sesión quede finalizada en la base con el valor acordado,
// cerrada por la persona indicada en el instante indicado del reloj falso.
func requireStoredFinished(t *testing.T, db *gorm.DB, sessionID string, agreed int, closedBy uuid.UUID, at time.Time) {
	t.Helper()
	stored := pokerStoredSession(t, db, sessionID)
	assert.Equal(t, domain.SessionStatusFinished, stored.Status, "la sesión debería quedar finalizada en la base")
	if assert.NotNil(t, stored.AgreedStoryPoints, "agreed_story_points no debería ser nulo") {
		assert.Equal(t, agreed, *stored.AgreedStoryPoints, "agreed_story_points")
	}
	if assert.NotNil(t, stored.ClosedByID, "closed_by_id no debería ser nulo") {
		assert.Equal(t, closedBy, *stored.ClosedByID, "closed_by_id")
	}
	if assert.NotNil(t, stored.ClosedAt, "closed_at no debería ser nulo") {
		assert.True(t, stored.ClosedAt.Equal(at), "closed_at = %s, se esperaba %s", stored.ClosedAt, at)
	}
}

// requireStoredOpen exige que la sesión siga abierta en la base, sin valor acordado ni cierre.
func requireStoredOpen(t *testing.T, db *gorm.DB, sessionID, why string) {
	t.Helper()
	stored := pokerStoredSession(t, db, sessionID)
	assert.Equal(t, domain.SessionStatusOpen, stored.Status, "la sesión debería seguir abierta: %s", why)
	assert.Nil(t, stored.AgreedStoryPoints, "la sesión abierta no tiene valor acordado: %s", why)
	assert.Nil(t, stored.ClosedAt, "la sesión abierta no tiene momento de cierre: %s", why)
}

// requireEnded exige que el servidor cierre el canal dentro del plazo de 2 segundos, sin enviar
// ningún evento más.
func (s *pokerStream) requireEnded(t *testing.T, where string) {
	t.Helper()
	timer := time.NewTimer(pokerStreamDeadline)
	defer timer.Stop()
	select {
	case ev, ok := <-s.events:
		if ok {
			t.Fatalf("%s: el canal debería cerrarse tras el último estado y llegó %q con datos %s", where, ev.Name, ev.Data)
		}
	case <-timer.C:
		t.Fatalf("%s: el servidor no cerró el canal en %s", where, pokerStreamDeadline)
	}
}

// requireAgreedValueRejected exige el 400 del contrato con un único detalle en storyPoints que
// enumera los valores admitidos.
func requireAgreedValueRejected(t *testing.T, resp pokerResponse) {
	t.Helper()
	want, err := json.Marshal(map[string]any{
		"error":  pokerInvalidFieldsMessage,
		"fields": []map[string]string{{"field": "storyPoints", "message": pokerAgreedValueMessage}},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, string(want), string(resp.Body))
}

// pokerIntPtr devuelve un puntero al entero indicado, para comparar Story Points.
func pokerIntPtr(v int) *int {
	return &v
}

// newPokerSoloSession abre, como Ana, una sesión sobre la historia indicada, abre su canal y espera a
// verla conectada como única participante; devuelve la sesión y el canal.
func (f pokerUS1Fixture) newPokerSoloSession(t *testing.T, storyID uuid.UUID) (pokerSessionBody, *pokerStream) {
	t.Helper()
	session := f.env.requireOpenedSession(t, f.anaToken, storyID)
	stream := f.env.openStream(t, f.anaToken, session.ID)
	stream.NextUntil(t, "Ana conectada", allConnected(f.anaID))
	return session, stream
}

// Escenario: Registro con consenso (005/US5)
// Ana, Bruno y Carla votan 8 y la ronda se revela sola con consenso en 8. Un minuto después Ana,
// facilitadora, registra 8: responde 200 con la sesión finalizada con valor 8 y cerrada por Ana; los
// tres canales reciben ese último estado sin recargar y en menos de 2 segundos, y el servidor los
// cierra. En la base la sesión queda finalizada con closed_at en el instante del reloj y la versión
// sube en uno, y la historia de la 003 queda en 8 Story Points (FR-049, FR-052, FR-053, FR-073,
// SC-013).
func TestUS5_RegistroConConsenso(t *testing.T) {
	f := newPokerUS2Fixture(t)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "8")
	}
	for name, stream := range f.threeStreams() {
		stream.NextUntil(t, "la ronda revelada en el canal de "+name, currentRoundRevealed)
	}
	before := pokerStoredVersion(t, f.env.db, f.session.ID)
	f.env.clock.Advance(time.Minute)

	resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 8)
	requireFinishedSession(t, resp, 8, f.anaID, "respuesta de Ana")

	// Cada canal recibe el estado final con el consenso de la ronda y después se cierra.
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la sesión finalizada en el canal de "+name, sessionFinished)
		requireFinishedSession(t, snapshot, 8, f.anaID, "canal de "+name)
		requireConsensus(t, pokerCurrentRound(t, snapshot), 8, "canal de "+name)
		stream.requireEnded(t, "canal de "+name)
	}

	requireStoredFinished(t, f.env.db, f.session.ID, 8, f.anaID, f.env.clock.Now())
	assert.Equal(t, before+1, pokerStoredVersion(t, f.env.db, f.session.ID), "registrar es un hecho y sube la versión")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.brunoToken, f.storyID), pokerIntPtr(8), "historia de la 003")
}

// Escenario: Registro sin consenso (005/US5)
// Ana, Bruno y Carla votan 5, 8 y 13 y la ronda se revela sola con divergencia. Ana, facilitadora,
// registra 8: responde 200 con la sesión finalizada con valor 8, el canal de Ana recibe el cierre y
// la historia queda en 8 Story Points, aunque no hubo consenso (FR-049, FR-052).
func TestUS5_RegistroSinConsenso(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "5")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "13")
	revealed := f.ana.NextUntil(t, "la ronda revelada en el canal de Ana", currentRoundRevealed)
	requireDivergence(t, pokerCurrentRound(t, revealed), 5, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, "ronda 1")

	resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 8)

	requireFinishedSession(t, resp, 8, f.anaID, "respuesta de Ana")
	requireFinishedSession(t, f.ana.NextUntil(t, "la sesión finalizada en el canal de Ana", sessionFinished), 8, f.anaID, "canal de Ana")
	requireStoredFinished(t, f.env.db, f.session.ID, 8, f.anaID, f.env.clock.Now())
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(8), "historia de la 003")
}

// Escenario: Valor no votado (005/US5)
// Ana vota 5 y Bruno 8; Carla no vota y Ana revela a mano. Ana registra 13, un valor que nadie votó:
// responde 200 con la sesión finalizada con valor 13 y la historia queda en 13 Story Points, porque
// el valor acordado no tiene que coincidir con ningún voto (FR-049, RN-18).
func TestUS5_ValorNoVotado(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "5")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	requirePokerStatus(t, f.env.reveal(t, f.anaToken, f.session.ID), http.StatusOK, nil)

	resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 13)

	requireFinishedSession(t, resp, 13, f.anaID, "respuesta de Ana")
	requireStoredFinished(t, f.env.db, f.session.ID, 13, f.anaID, f.env.clock.Now())
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(13), "historia de la 003")
}

// Escenario: Reemplaza estimación anterior (005/US5)
// La historia ya estaba estimada en 3 Story Points desde el backlog de la 003. Ana abre una sesión
// sola, vota 20 y la ronda se revela sola; al registrar 20 la sesión queda finalizada con ese valor
// y la historia pasa de 3 a 20 Story Points (FR-052).
func TestUS5_ReemplazaEstimacionAnterior(t *testing.T) {
	f := newPokerUS1Fixture(t)
	f.env.requireStoryEstimated(t, f.brunoToken, f.storyID, 3)
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(3), "historia antes de la sesión")
	session, _ := f.newPokerSoloSession(t, f.storyID)
	f.env.requireVoted(t, f.anaToken, session.ID, "20")

	resp := f.env.requireFinalized(t, f.anaToken, session.ID, 20)

	requireFinishedSession(t, resp, 20, f.anaID, "respuesta de Ana")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(20), "historia de la 003")
}

// Escenario: Reestimación directa durante la sesión (005/US5)
// Con la ronda 1 en votación sobre una historia sin estimar, Bruno la reestima en 3 con PUT
// /api/v1/stories/{storyId}/story-points de la 003: responde 200, la versión de la sesión no cambia y
// por el canal de Ana no llega ningún estado con otra versión, porque no es un hecho de la sesión;
// el siguiente estado es el del voto de Bruno, con la versión siguiente. La sesión sigue su curso:
// la ronda se revela y, al registrar 8, la historia queda en 8 Story Points (FR-075, RN-32).
func TestUS5_ReestimacionDirectaDuranteLaSesion(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	f.env.requireStoryEstimated(t, f.brunoToken, f.storyID, 3)
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "la reestimación directa no sube la versión")

	// Entre la reestimación y el voto de Bruno, el canal de Ana no recibe ninguna otra versión.
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	for {
		snapshot := f.ana.Next(t)
		if hasVoted(f.brunoID)(snapshot) {
			assert.Equal(t, before+1, snapshot.Version, "el voto de Bruno es el hecho siguiente: %s", snapshot.Raw)
			break
		}
		assert.Equal(t, before, snapshot.Version, "la reestimación no produce un estado con otra versión: %s", snapshot.Raw)
	}

	// La sesión sigue su curso y el registro reemplaza la reestimación directa.
	f.env.requireVoted(t, f.anaToken, f.session.ID, "8")
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "8")
	f.ana.NextUntil(t, "la ronda revelada en el canal de Ana", currentRoundRevealed)
	resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 8)

	requireFinishedSession(t, resp, 8, f.anaID, "respuesta de Ana")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.brunoToken, f.storyID), pokerIntPtr(8), "historia de la 003")
}

// Escenario: Registro tras todos interrogación (005/US5)
// Ana, Bruno y Carla votan "?" y la ronda se revela sola sin votos numéricos. Ana registra 40 de todos
// modos: responde 200 con la sesión finalizada con valor 40 y la historia queda en 40 Story Points,
// porque el valor acordado lo decide el facilitador (FR-049, RN-18).
func TestUS5_RegistroTrasTodosInterrogacion(t *testing.T) {
	f := newPokerUS2Fixture(t)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "?")
	}
	f.ana.NextUntil(t, "la ronda revelada en el canal de Ana", currentRoundRevealed)

	resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 40)

	requireFinishedSession(t, resp, 40, f.anaID, "respuesta de Ana")
	requireStoredFinished(t, f.env.db, f.session.ID, 40, f.anaID, f.env.clock.Now())
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(40), "historia de la 003")
}

// Escenario: Historia sin criterios no queda lista (005/US5)
// Sobre una historia sin estimar y sin criterios de aceptación, Ana abre una sesión sola, vota 5 y la
// ronda se revela sola. Al registrar 5, el detalle de la 003 muestra la historia con 5 Story Points
// pero todavía no lista para planificar, porque le falta al menos un criterio (FR-052, FR-028 de la
// 003).
func TestUS5_HistoriaSinCriteriosNoQuedaLista(t *testing.T) {
	f := newPokerUS1Fixture(t)
	storyID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{Title: "Historia sin criterios"})
	session, _ := f.newPokerSoloSession(t, storyID)
	f.env.requireVoted(t, f.anaToken, session.ID, "5")

	resp := f.env.requireFinalized(t, f.anaToken, session.ID, 5)

	requireFinishedSession(t, resp, 5, f.anaID, "respuesta de Ana")
	detail := f.env.requireStoryDetail(t, f.anaToken, storyID)
	requireStoryPoints(t, detail, pokerIntPtr(5), "historia de la 003")
	assert.False(t, detail.Readiness.Ready, "sin criterios la historia no está lista para planificar")
	assert.Equal(t, []string{"acceptance_criteria"}, detail.Readiness.Missing, "solo le falta el criterio")
}

// Escenario: Rondas conservadas (005/US5)
// Con 1, 3 y 10 rondas reveladas con los votos 3, 5 y 13 de Ana, Bruno y Carla, Ana registra 8: la
// consulta de la sesión de Bruno con GET /api/v1/estimation-sessions/{sessionId} muestra la sesión
// finalizada con todas sus rondas reveladas, cada una con sus tres votos y la divergencia de 3 a 13,
// y en la base quedan todas las rondas con sus votos (FR-053, RN-28, SC-016).
func TestUS5_RondasConservadas(t *testing.T) {
	for _, total := range []int{1, 3, 10} {
		t.Run(fmt.Sprintf("%d rondas", total), func(t *testing.T) {
			f := newPokerUS2Fixture(t)
			f.revealDivergentRound(t)
			for number := 2; number <= total; number++ {
				f.env.requireStartedRound(t, f.anaToken, f.session.ID)
				f.revealDivergentRound(t)
			}

			f.env.requireFinalized(t, f.anaToken, f.session.ID, 8)

			// La consulta de la sesión cerrada muestra todas las rondas completas.
			session := f.env.requireSessionFetched(t, f.brunoToken, f.session.ID)
			requireFinishedSession(t, session, 8, f.anaID, "consulta de Bruno")
			rounds := pokerAllRounds(t, session)
			require.Len(t, rounds, total, "rondas: %s", session.Raw)
			for i, round := range rounds {
				where := fmt.Sprintf("ronda %d", i+1)
				assert.Equal(t, i+1, round.Number, "%s: número", where)
				assert.Equal(t, "revealed", round.Status, "%s: estado", where)
				assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "%s: votos", where)
				requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, where)
			}

			// En la base quedan todas las rondas con sus tres votos cada una.
			stored := pokerStoredRounds(t, f.env.db, f.session.ID)
			require.Len(t, stored, total, "la sesión debería conservar sus rondas en la base")
			for _, round := range stored {
				assert.Equal(t, domain.RoundStatusRevealed, round.Status, "ronda %d", round.Number)
				assert.EqualValues(t, 3, pokerStoredRoundVotes(t, f.env.db, round.ID), "ronda %d conserva sus votos", round.Number)
			}
		})
	}
}

// Escenario: Valor inválido (005/US5)
// Con la ronda 1 revelada, Ana, facilitadora, intenta registrar 7, un decimal y null ("sin estimar"):
// cada intento responde 400 con el detalle en storyPoints que enumera los valores admitidos; la
// sesión sigue abierta, la versión no cambia y la historia sigue sin estimar (FR-050, RN-18).
func TestUS5_ValorInvalido(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	attempts := []struct {
		name string
		body map[string]any
	}{
		{"7", map[string]any{"storyPoints": 7}},
		{"decimal", map[string]any{"storyPoints": 2.5}},
		{"null", map[string]any{"storyPoints": nil}},
	}
	for _, tc := range attempts {
		t.Run(tc.name, func(t *testing.T) {
			resp := f.env.finalize(t, f.anaToken, f.session.ID, tc.body)

			requireAgreedValueRejected(t, resp)
			requireStoredOpen(t, f.env.db, f.session.ID, "el rechazo no cierra la sesión")
			assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
			requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), nil, "historia de la 003")
		})
	}
}

// Escenario: Sin ronda revelada (005/US5)
// Con la ronda 1 en votación y el voto 5 de Bruno todavía oculto, Ana, facilitadora, intenta registrar
// 8: responde 409 con el mensaje de que primero hay que revelar al menos una ronda o cancelar la
// sesión; la sesión sigue abierta con la ronda en votación, la versión no cambia y la historia sigue
// sin estimar (FR-051, RN-20).
func TestUS5_SinRondaRevelada(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.finalize(t, f.anaToken, f.session.ID, map[string]any{"storyPoints": 8})

	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerNoRevealedRoundBodyJSON, string(resp.Body))
	requireStoredOpen(t, f.env.db, f.session.ID, "el rechazo no cierra la sesión")
	requireStoredVoting(t, f.env.db, f.session.ID, "el rechazo no revela")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), nil, "historia de la 003")
}

// Escenario: Registro sin ser facilitador (005/US5)
// Con la ronda 1 revelada, Bruno, que no es el facilitador, intenta registrar 8: responde 403 con el
// mensaje de que la acción está reservada al facilitador; la sesión sigue abierta, la versión no
// cambia y los Story Points de la historia no cambian (FR-054, FR-064, SC-011).
func TestUS5_RegistroSinSerFacilitador(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.finalize(t, f.brunoToken, f.session.ID, map[string]any{"storyPoints": 8})

	assert.Equal(t, http.StatusForbidden, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerFacilitatorOnlyBodyJSON, string(resp.Body))
	requireStoredOpen(t, f.env.db, f.session.ID, "el rechazo no cierra la sesión")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), nil, "historia de la 003")
}

// Pruebas de integración de punta a punta de la historia 6 de Planning Poker (T071): cortan y reabren
// canales, avanzan el reloj falso y llaman al barrido para vencer la gracia de 30 segundos y los 5
// minutos del relevo, y toman el rol con POST /api/v1/estimation-sessions/{sessionId}/facilitator,
// sobre el mismo router compuesto como cmd/app y contra smye_test. Verifican la respuesta, las
// instantáneas que llegan sin recargar por los canales ya abiertos y las filas de estimation_sessions,
// estimation_participants y estimation_votes. El "Dado" habitual es el de US2: Ana facilitadora, con
// Bruno y Carla, los tres conectados y la ronda 1 en votación. También van acá, sin etiqueta de
// historia, el barrido de la membresía revocada y de los cambios de la historia, y el reinicio del
// servidor.

// Textos exactos que fija el contrato para los rechazos de la toma del rol de facilitador.
const (
	pokerFacilitatorPresentMessage = "El facilitador está conectado."
	pokerFacilitatorNotYetMessage  = "El rol de facilitador todavía no está disponible."
)

// pokerTakeoverWait son los 5 minutos de desconexión continua del facilitador que habilitan el
// relevo (FR-068).
const pokerTakeoverWait = 5 * time.Minute

// facilitatorPath es la ruta de la toma del rol de facilitador según contracts/openapi.yaml.
func facilitatorPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/facilitator"
}

// takeFacilitator pide POST /api/v1/estimation-sessions/{sessionId}/facilitator, sin cuerpo, con el
// token como Bearer.
func (e *pokerTestEnv) takeFacilitator(t *testing.T, token, sessionID string) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, facilitatorPath(sessionID), token, nil)
}

// requireTookFacilitator toma el rol, exige el 200 y devuelve la sesión resultante vista por quien lo
// tomó.
func (e *pokerTestEnv) requireTookFacilitator(t *testing.T, token, sessionID string) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.takeFacilitator(t, token, sessionID)
	requirePokerStatus(t, resp, http.StatusOK, &session)
	session.Raw = resp.Body
	return session
}

// dropAndExpire cierra el canal y vence la gracia de 30 segundos con el barrido, de modo que la
// persona queda desconectada con disconnected_at en el instante del reloj (FR-014).
func (e *pokerTestEnv) dropAndExpire(t *testing.T, stream *pokerStream) {
	t.Helper()
	stream.Close()
	e.sweep(t, 30*time.Second)
}

// facilitatorIs arma la condición "la persona es la facilitadora de la sesión", para esperar con
// NextUntil.
func facilitatorIs(userID uuid.UUID) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		return s.Facilitator.ID == userID.String()
	}
}

// participantGone arma la condición "la persona figura en la lista como desconectada", para esperar
// con NextUntil.
func participantGone(userID uuid.UUID) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		p := s.participant(userID)
		return p != nil && !p.Connected
	}
}

// participantBack arma la condición "la persona figura conectada", para esperar con NextUntil.
func participantBack(userID uuid.UUID) func(pokerSessionBody) bool {
	return func(s pokerSessionBody) bool {
		p := s.participant(userID)
		return p != nil && p.Connected
	}
}

// requireFacilitatorRole exige que la instantánea muestre a la persona indicada como única
// facilitadora, también en la marca de la lista de participantes.
func requireFacilitatorRole(t *testing.T, snapshot pokerSessionBody, facilitatorID uuid.UUID, where string) {
	t.Helper()
	assert.Equal(t, facilitatorID.String(), snapshot.Facilitator.ID, "%s: facilitador de la sesión: %s", where, snapshot.Raw)
	for _, p := range snapshot.Participants {
		assert.Equal(t, p.User.ID == facilitatorID.String(), p.IsFacilitator, "%s: marca de facilitador de %s", where, p.User.FullName)
	}
}

// requireFacilitatorUnavailable exige el 409 del contrato con el mensaje, el facilitador vigente y,
// si retryAfter no es nil, los segundos que faltan; si es nil, retryAfterSeconds debe estar ausente.
func requireFacilitatorUnavailable(t *testing.T, resp pokerResponse, message string, facilitatorID uuid.UUID, fullName string, retryAfter *int) {
	t.Helper()
	want := map[string]any{
		"error":       message,
		"facilitator": map[string]string{"id": facilitatorID.String(), "fullName": fullName},
	}
	if retryAfter != nil {
		want["retryAfterSeconds"] = *retryAfter
	}
	payload, err := json.Marshal(want)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, string(payload), string(resp.Body))
}

// requireStoredFacilitator exige que la sesión tenga en la base el facilitator_id indicado.
func requireStoredFacilitator(t *testing.T, db *gorm.DB, sessionID string, facilitatorID uuid.UUID, why string) {
	t.Helper()
	assert.Equal(t, facilitatorID, pokerStoredSession(t, db, sessionID).FacilitatorID, "facilitator_id: %s", why)
}

// requireRemoved lee el canal hasta el evento removed, descartando los state que lleguen antes, y
// falla si no llega dentro del plazo de 2 segundos, si llega otro evento o si el canal se cierra sin
// enviarlo (FR-019).
func (s *pokerStream) requireRemoved(t *testing.T, where string) {
	t.Helper()
	timer := time.NewTimer(pokerStreamDeadline)
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				t.Fatalf("%s: el canal se cerró sin enviar el evento removed", where)
			}
			if ev.Name == "removed" {
				return
			}
			if ev.Name != "state" {
				t.Fatalf("%s: se esperaba el evento removed y llegó %q con datos %s", where, ev.Name, ev.Data)
			}
		case <-timer.C:
			t.Fatalf("%s: no llegó el evento removed en %s", where, pokerStreamDeadline)
		}
	}
}

// Escenario: Desconexión dispara la revelación (005/US6)
// Con Ana, Bruno y Carla conectados en la ronda 1, Ana vota 3 y Bruno 5. Carla cierra su canal y, al
// vencer la gracia, el barrido registra su desconexión: los canales de Ana y Bruno la muestran
// desconectada en menos de 2 segundos y con la ronda revelada sola, porque todos los conectados ya
// votaron, con los dos votos y notVoted vacío (FR-014, FR-030, FR-031, FR-073).
func TestUS6_DesconexionDisparaRevelacion(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")

	f.env.dropAndExpire(t, f.carla)

	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "5"}
	for name, stream := range map[string]*pokerStream{"Ana": f.ana, "Bruno": f.bruno} {
		snapshot := stream.NextUntil(t, "Carla desconectada y la ronda revelada en el canal de "+name, func(s pokerSessionBody) bool {
			return participantGone(f.carlaID)(s) && currentRoundRevealed(s)
		})
		requireDivergence(t, requireRevealedRound(t, snapshot, votes, nil, "canal de "+name),
			3, []uuid.UUID{f.anaID}, 5, []uuid.UUID{f.brunoID}, "canal de "+name)
	}
	assert.False(t, pokerStoredParticipant(t, f.env.db, f.session.ID, f.carlaID).Connected, "Carla queda desconectada en la base")
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Reconexión con voto (005/US6)
// Bruno vota 13 y cierra su canal; al vencer la gracia queda desconectado con la ronda todavía en
// votación, porque Ana y Carla no votaron. Bruno abre un canal nuevo en la misma ronda: su canal lo
// muestra conectado, con su voto contado y su carta 13 en myVote; Ana lo ve conectado y como que ya
// votó, y en la base sigue su única fila de voto con 13 (FR-015, FR-016).
func TestUS6_ReconexionConVoto(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "13")
	f.env.dropAndExpire(t, f.bruno)
	f.ana.NextUntil(t, "Bruno desconectado en el canal de Ana", participantGone(f.brunoID))
	requireStoredVoting(t, f.env.db, f.session.ID, "Ana y Carla todavía no votaron")

	bruno := f.env.openStream(t, f.brunoToken, f.session.ID)

	// Bruno vuelve conectado y ve su propia carta.
	back := bruno.NextUntil(t, "Bruno conectado en su canal nuevo", participantBack(f.brunoID))
	requireOwnVote(t, back, f.brunoID, "13", "canal nuevo de Bruno")

	// Ana ve a Bruno conectado y con su voto contado, sin el valor.
	seen := f.ana.NextUntil(t, "Bruno conectado en el canal de Ana", participantBack(f.brunoID))
	requireSecretVote(t, seen, f.brunoID, "Ana")
	assert.True(t, pokerStoredParticipant(t, f.env.db, f.session.ID, f.brunoID).Connected, "Bruno vuelve a estar conectado en la base")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "13")
}

// Escenario: Reconexión sin voto (005/US6)
// Ana vota 3 y Carla cierra su canal sin votar; al vencer la gracia queda desconectada y la ronda
// sigue en votación porque Bruno no votó. Carla abre un canal nuevo en la misma ronda: figura
// conectada y sin voto. Cuando Bruno vota 5 la ronda no se revela, porque Carla vuelve a contarse
// para la revelación automática; cuando Carla vota 8 la ronda se revela sola con los tres votos
// (FR-015, FR-030, FR-031).
func TestUS6_ReconexionSinVoto(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.dropAndExpire(t, f.carla)
	f.ana.NextUntil(t, "Carla desconectada en el canal de Ana", participantGone(f.carlaID))

	carla := f.env.openStream(t, f.carlaToken, f.session.ID)

	// Carla vuelve conectada, sin voto, en la ronda 1 en votación.
	back := carla.NextUntil(t, "Carla conectada en su canal nuevo", participantBack(f.carlaID))
	assert.True(t, roundVoting(1)(back), "la ronda 1 sigue en votación: %s", back.Raw)
	assert.False(t, back.participant(f.carlaID).HasVoted, "Carla no tiene voto")
	assert.Nil(t, back.MyVote, "Carla no tiene carta propia")

	// El voto de Bruno no completa la ronda, porque Carla cuenta otra vez.
	afterBruno := f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	assert.True(t, roundVoting(1)(afterBruno), "con Carla conectada sin votar la ronda no se revela: %s", afterBruno.Raw)
	requireStoredVoting(t, f.env.db, f.session.ID, "Carla vuelve a contarse para la revelación")

	// Carla vota y la ronda se revela sola con los tres votos.
	last := f.env.requireVoted(t, f.carlaToken, f.session.ID, "8")
	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "5", f.carlaID: "8"}
	requireRevealedRound(t, last, votes, nil, "respuesta de Carla")
	requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
}

// Escenario: Reconexión en ronda posterior (005/US6)
// Carla vota 13 en la ronda 1 y cierra su canal; al vencer la gracia queda desconectada. Ana y Bruno
// votan 3 y 5 y la ronda 1 se revela con los tres votos; la ronda 2 se revela con los votos de Ana y
// Bruno, y Ana inicia la ronda 3. Carla abre un canal nuevo: se incorpora a la ronda 3 en votación
// sin voto, su 13 sigue en la ronda 1 del historial y en la base conserva una sola fila de voto, la de
// la ronda 1 (FR-015, FR-017, FR-078).
func TestUS6_ReconexionEnRondaPosterior(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.carlaToken, f.session.ID, "13")
	f.env.dropAndExpire(t, f.carla)
	f.ana.NextUntil(t, "Carla desconectada en el canal de Ana", participantGone(f.carlaID))

	// Ronda 1 con el voto de Carla y ronda 2 sin ella; Ana deja abierta la ronda 3.
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)

	carla := f.env.openStream(t, f.carlaToken, f.session.ID)

	// Carla entra a la ronda 3 sin voto y ve su voto de la ronda 1 en el historial.
	joined := carla.NextUntil(t, "Carla conectada en la ronda 3", func(s pokerSessionBody) bool {
		return participantBack(f.carlaID)(s) && roundVoting(3)(s)
	})
	requireFreshVotingRound(t, joined, 3, "canal de Carla")
	rounds := pokerAllRounds(t, joined)
	require.Len(t, rounds, 3, "rondas: %s", joined.Raw)
	assert.Equal(t, map[string]string{f.anaID.String(): "3", f.brunoID.String(): "5", f.carlaID.String(): "13"},
		pokerRevealedCards(rounds[0]), "la ronda 1 conserva el voto de Carla: %s", joined.Raw)
	assert.NotContains(t, pokerRevealedCards(rounds[1]), f.carlaID.String(), "Carla no votó en la ronda 2")
	votes := pokerStoredVotes(t, f.env.db, f.session.ID, f.carlaID)
	if assert.Len(t, votes, 1, "Carla debería conservar solo su voto de la ronda 1") {
		assert.Equal(t, pokerStoredRounds(t, f.env.db, f.session.ID)[0].ID, votes[0].RoundID, "el voto de Carla es de la ronda 1")
		assert.Equal(t, domain.Card("13"), votes[0].Card)
	}
}

// newPokerRejoinedFacilitatorFixture prepara el "Dado" de US6-5: Ana, facilitadora, cierra su canal
// y al vencer la gracia queda desconectada; dos minutos después abre un canal nuevo y vuelve
// conectada y como facilitadora, sin momento de relevo disponible, y Bruno vota 5.
func newPokerRejoinedFacilitatorFixture(t *testing.T) pokerUS2Fixture {
	t.Helper()
	f := newPokerUS2Fixture(t)
	f.env.dropAndExpire(t, f.ana)
	f.bruno.NextUntil(t, "Ana desconectada en el canal de Bruno", participantGone(f.anaID))
	f.env.sweep(t, 2*time.Minute)

	f.ana = f.env.openStream(t, f.anaToken, f.session.ID)

	// Ana vuelve conectada y sigue siendo la facilitadora.
	back := f.ana.NextUntil(t, "Ana conectada en su canal nuevo", participantBack(f.anaID))
	requireFacilitatorRole(t, back, f.anaID, "canal nuevo de Ana")
	assert.Nil(t, back.FacilitatorAvailableAt, "con el facilitador conectado el rol no está disponible: %s", back.Raw)
	requireStoredFacilitator(t, f.env.db, f.session.ID, f.anaID, "Ana retoma el rol al reconectarse")
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	return f
}

// Escenario: Facilitador retoma el rol (005/US6)
// Ana, facilitadora, cierra su canal y al vencer la gracia queda desconectada. Dos minutos después
// abre un canal nuevo: vuelve conectada y como facilitadora, sin momento de relevo disponible. Con el
// voto 5 de Bruno, Ana revela, inicia una ronda y registra la estimación, cada acción en su subtest
// (FR-065, FR-067). El subtest de cancelar, escrito en T079 cuando llegó la cancelación, prepara su
// propia sesión, porque los anteriores terminan registrando la estimación.
func TestUS6_FacilitadorRetomaElRol(t *testing.T) {
	f := newPokerRejoinedFacilitatorFixture(t)

	t.Run("revela", func(t *testing.T) {
		resp := f.env.reveal(t, f.anaToken, f.session.ID)

		requirePokerStatus(t, resp, http.StatusOK, nil)
		requireStoredRevealed(t, f.env.db, f.session.ID, f.env.clock.Now())
	})
	t.Run("inicia ronda", func(t *testing.T) {
		resp := f.env.requireStartedRound(t, f.anaToken, f.session.ID)

		requireFreshVotingRound(t, resp, 2, "respuesta de Ana")
	})
	t.Run("registra", func(t *testing.T) {
		resp := f.env.requireFinalized(t, f.anaToken, f.session.ID, 5)

		requireFinishedSession(t, resp, 5, f.anaID, "respuesta de Ana")
		requireStoredFinished(t, f.env.db, f.session.ID, 5, f.anaID, f.env.clock.Now())
	})
	// Escenario: Facilitador retoma el rol (005/US6)
	// Con una sesión propia en la que Ana retomó el rol, Ana cancela: responde 200 con la sesión
	// cancelada por ella y en la base queda cancelada por Ana, sin motivo del sistema (FR-065, FR-057).
	t.Run("cancela", func(t *testing.T) {
		g := newPokerRejoinedFacilitatorFixture(t)

		resp := g.env.requireCanceled(t, g.anaToken, g.session.ID)

		requireCanceledByFacilitator(t, resp, g.anaID, "respuesta de Ana")
		requireStoredCanceled(t, g.env.db, g.session.ID, &g.anaID, nil, g.env.clock.Now())
	})
}

// Escenario: Todos desconectados (005/US6)
// Ana abre la sesión y ella y Bruno abren su canal sin votar. Los dos cierran su canal y, al vencer la
// gracia, el barrido los desconecta: no se revela nada, la sesión sigue abierta con su única ronda en
// votación y sin votos, y los dos quedan desconectados en la base. Cuando Bruno vuelve, recibe la
// sesión abierta en la misma ronda 1 en votación (FR-014, FR-030, RN-09).
func TestUS6_TodosDesconectados(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	both := allConnected(f.anaID, f.brunoID)
	ana.NextUntil(t, "Ana y Bruno conectados en el canal de Ana", both)
	bruno.NextUntil(t, "Ana y Bruno conectados en el canal de Bruno", both)

	ana.Close()
	bruno.Close()
	f.env.sweep(t, 30*time.Second)

	// Nada se revela y la sesión queda abierta esperando.
	requireStoredOpen(t, f.env.db, session.ID, "nadie conectado no cierra la sesión")
	requireStoredVoting(t, f.env.db, session.ID, "sin conectados no hay revelación")
	assert.Len(t, pokerStoredRounds(t, f.env.db, session.ID), 1, "la sesión sigue en la ronda 1")
	for _, id := range []uuid.UUID{f.anaID, f.brunoID} {
		assert.False(t, pokerStoredParticipant(t, f.env.db, session.ID, id).Connected, "%s queda desconectado", id)
	}

	// Bruno vuelve a la misma ronda en votación.
	back := f.env.openStream(t, f.brunoToken, session.ID)
	snapshot := back.NextUntil(t, "Bruno conectado de nuevo", participantBack(f.brunoID))
	requireFirstRoundVoting(t, snapshot)
	assert.False(t, snapshot.participant(f.anaID).Connected, "Ana sigue desconectada")
}

// Escenario: Facilitador original relevado (005/US6)
// Ana, facilitadora, cierra su canal y queda desconectada al vencer la gracia; 5 minutos después
// Bruno toma el rol: responde 200 con Bruno como facilitador y Carla ve el cambio sin recargar. Ana
// abre un canal nuevo: figura conectada como participante común y Bruno sigue como facilitador. Con
// el voto 5 de Bruno, Ana intenta revelar y recibe 403 de acción reservada al facilitador, y Bruno
// revela con 200; en la base facilitator_id queda en Bruno (FR-068, FR-070, FR-071).
func TestUS6_FacilitadorOriginalRelevado(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.dropAndExpire(t, f.ana)
	f.carla.NextUntil(t, "Ana desconectada en el canal de Carla", participantGone(f.anaID))
	f.env.clock.Advance(pokerTakeoverWait)

	taken := f.env.requireTookFacilitator(t, f.brunoToken, f.session.ID)

	// La respuesta y el canal de Carla muestran a Bruno como facilitador.
	requireFacilitatorRole(t, taken, f.brunoID, "respuesta de Bruno")
	requireFacilitatorRole(t, f.carla.NextUntil(t, "Bruno facilitador en el canal de Carla", facilitatorIs(f.brunoID)), f.brunoID, "canal de Carla")

	// Ana vuelve como participante común.
	ana := f.env.openStream(t, f.anaToken, f.session.ID)
	back := ana.NextUntil(t, "Ana conectada en su canal nuevo", participantBack(f.anaID))
	requireFacilitatorRole(t, back, f.brunoID, "canal nuevo de Ana")

	// Solo Bruno puede revelar.
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	rejected := f.env.reveal(t, f.anaToken, f.session.ID)
	assert.Equal(t, http.StatusForbidden, rejected.Code, "cuerpo: %s", rejected.Body)
	assert.JSONEq(t, pokerFacilitatorOnlyBodyJSON, string(rejected.Body))
	requirePokerStatus(t, f.env.reveal(t, f.brunoToken, f.session.ID), http.StatusOK, nil)
	requireStoredFacilitator(t, f.env.db, f.session.ID, f.brunoID, "el original no recupera el rol al volver")
}

// Escenario: Único participante toma el rol (005/US6)
// Ana abre la sesión y ella y Bruno abren su canal. Ana cierra el suyo y queda desconectada al vencer
// la gracia; 5 minutos después Bruno, único conectado, toma el rol: responde 200 con él como
// facilitador. Bruno vota 8, la ronda se revela sola y registra 8 por sí solo: la sesión queda
// finalizada y cerrada por Bruno (FR-068, FR-070). La cancelación por sí solo llega con US7.
func TestUS6_UnicoParticipanteTomaElRol(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	both := allConnected(f.anaID, f.brunoID)
	ana.NextUntil(t, "Ana y Bruno conectados en el canal de Ana", both)
	bruno.NextUntil(t, "Ana y Bruno conectados en el canal de Bruno", both)
	f.env.dropAndExpire(t, ana)
	bruno.NextUntil(t, "Ana desconectada en el canal de Bruno", participantGone(f.anaID))
	f.env.clock.Advance(pokerTakeoverWait)

	taken := f.env.requireTookFacilitator(t, f.brunoToken, session.ID)

	requireFacilitatorRole(t, taken, f.brunoID, "respuesta de Bruno")
	requireStoredFacilitator(t, f.env.db, session.ID, f.brunoID, "Bruno toma el rol")

	// Bruno, solo, cierra la sesión con el valor acordado.
	f.env.requireVoted(t, f.brunoToken, session.ID, "8")
	resp := f.env.requireFinalized(t, f.brunoToken, session.ID, 8)
	requireFinishedSession(t, resp, 8, f.brunoID, "respuesta de Bruno")
	requireStoredFinished(t, f.env.db, session.ID, 8, f.brunoID, f.env.clock.Now())
}

// Escenario: Dos pestañas (005/US6)
// Bruno abre la sesión en una segunda pestaña, vota 8 y Ana vota 3. Bruno cierra una de las dos y
// pasa la gracia: sigue conectado en la base y su pestaña abierta muestra su carta 8. Cuando Carla
// vota 5, la ronda se revela sola con un voto por persona: la respuesta y los canales muestran los
// tres votos, notVoted vacío y a Bruno conectado (FR-013, RN-34, SC-024).
func TestUS6_DosPestanas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	second := f.env.openStream(t, f.brunoToken, f.session.ID)
	second.NextUntil(t, "los tres conectados en la segunda pestaña de Bruno", allConnected(f.anaID, f.brunoID, f.carlaID))
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	f.env.requireVoted(t, f.anaToken, f.session.ID, "3")

	f.env.dropAndExpire(t, f.bruno)

	// Bruno sigue conectado y su pestaña abierta muestra su carta.
	assert.True(t, pokerStoredParticipant(t, f.env.db, f.session.ID, f.brunoID).Connected,
		"con una pestaña abierta Bruno sigue conectado")
	requireOwnVote(t, second.NextUntil(t, "Bruno ya votó en su pestaña abierta", hasVoted(f.brunoID)), f.brunoID, "8", "pestaña abierta de Bruno")

	// El voto de Carla completa la ronda con un voto por persona.
	last := f.env.requireVoted(t, f.carlaToken, f.session.ID, "5")
	votes := map[uuid.UUID]string{f.anaID: "3", f.brunoID: "8", f.carlaID: "5"}
	requireRevealedRound(t, last, votes, nil, "respuesta de Carla")
	for name, stream := range map[string]*pokerStream{"Ana": f.ana, "pestaña abierta de Bruno": second} {
		snapshot := stream.NextUntil(t, "la ronda revelada en "+name, currentRoundRevealed)
		requireRevealedRound(t, snapshot, votes, nil, name)
		assert.True(t, snapshot.participant(f.brunoID).Connected, "%s: Bruno figura conectado", name)
	}
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "8")
}

// Escenario: Facilitador presente (005/US6)
// Con Ana, facilitadora, conectada, Bruno intenta tomar el rol: responde 409 con el mensaje de que el
// facilitador está conectado, Ana como facilitadora vigente y sin retryAfterSeconds; facilitator_id
// sigue en Ana y la versión no cambia, porque el rechazo no es un hecho (FR-069).
func TestUS6_FacilitadorPresente(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.takeFacilitator(t, f.brunoToken, f.session.ID)

	requireFacilitatorUnavailable(t, resp, pokerFacilitatorPresentMessage, f.anaID, "Ana Pérez", nil)
	requireStoredFacilitator(t, f.env.db, f.session.ID, f.anaID, "el rechazo no cambia el facilitador")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// Escenario: Rol todavía no disponible (005/US6)
// Ana, facilitadora, cierra su canal y queda desconectada al vencer la gracia: el canal de Bruno
// muestra facilitatorAvailableAt 5 minutos después de su desconexión. Dos minutos más tarde Bruno
// intenta tomar el rol: responde 409 con el mensaje de que el rol todavía no está disponible, Ana
// como facilitadora vigente y retryAfterSeconds 180; facilitator_id sigue en Ana y la versión no
// cambia (FR-068, FR-069).
func TestUS6_RolTodaviaNoDisponible(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.dropAndExpire(t, f.ana)

	// El canal de Bruno informa desde cuándo queda disponible el rol.
	disconnectedAt := pokerStoredParticipant(t, f.env.db, f.session.ID, f.anaID).DisconnectedAt
	require.NotNil(t, disconnectedAt, "Ana debería tener momento de desconexión")
	gone := f.bruno.NextUntil(t, "Ana desconectada en el canal de Bruno", participantGone(f.anaID))
	if assert.NotNil(t, gone.FacilitatorAvailableAt, "facilitatorAvailableAt no debería ser null: %s", gone.Raw) {
		availableAt, err := time.Parse(time.RFC3339, *gone.FacilitatorAvailableAt)
		require.NoError(t, err, "facilitatorAvailableAt = %q no es una fecha RFC 3339", *gone.FacilitatorAvailableAt)
		assert.True(t, availableAt.Equal(disconnectedAt.Add(pokerTakeoverWait)),
			"facilitatorAvailableAt = %s, se esperaba %s", availableAt, disconnectedAt.Add(pokerTakeoverWait))
	}
	f.env.clock.Advance(2 * time.Minute)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.takeFacilitator(t, f.brunoToken, f.session.ID)

	requireFacilitatorUnavailable(t, resp, pokerFacilitatorNotYetMessage, f.anaID, "Ana Pérez", pokerIntPtr(180))
	requireStoredFacilitator(t, f.env.db, f.session.ID, f.anaID, "el rechazo no cambia el facilitador")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// Escenario: Tomas simultáneas (005/US6)
// Ana, facilitadora, lleva 5 minutos desconectada. Bruno y Carla piden el rol en el mismo instante:
// uno responde 200 y el otro 409 con el mensaje de que el facilitador está conectado y el nuevo
// facilitador como vigente. En la base facilitator_id queda en quien ganó y la versión sube en uno,
// y los canales de Bruno y Carla reciben ese único cambio de facilitador (FR-071, FR-072, SC-025).
func TestUS6_TomasSimultaneas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.dropAndExpire(t, f.ana)
	f.bruno.NextUntil(t, "Ana desconectada en el canal de Bruno", participantGone(f.anaID))
	f.carla.NextUntil(t, "Ana desconectada en el canal de Carla", participantGone(f.anaID))
	f.env.clock.Advance(pokerTakeoverWait)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	// Bruno y Carla piden el rol a la vez tras la misma señal.
	var bruno, carla pokerConcurrentResult
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		bruno = f.env.pokerSend(http.MethodPost, facilitatorPath(f.session.ID), f.brunoToken, nil)
	})
	wg.Go(func() {
		<-start
		carla = f.env.pokerSend(http.MethodPost, facilitatorPath(f.session.ID), f.carlaToken, nil)
	})
	close(start)
	wg.Wait()
	require.NoError(t, bruno.err, "el pedido de Bruno no obtuvo respuesta")
	require.NoError(t, carla.err, "el pedido de Carla no obtuvo respuesta")

	// Exactamente uno obtiene el rol y el otro recibe al nuevo facilitador.
	winnerID, winnerName, loser := f.brunoID, "Bruno Gómez", carla.resp
	if carla.resp.Code == http.StatusOK {
		winnerID, winnerName, loser = f.carlaID, "Carla Díaz", bruno.resp
	}
	codes := []int{bruno.resp.Code, carla.resp.Code}
	require.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes,
		"un pedido debería prosperar y el otro rechazarse; Bruno: %s; Carla: %s", bruno.resp.Body, carla.resp.Body)
	requireFacilitatorUnavailable(t, loser, pokerFacilitatorPresentMessage, winnerID, winnerName, nil)

	// Un único cambio de facilitador, en la base y en los canales.
	requireStoredFacilitator(t, f.env.db, f.session.ID, winnerID, "prospera solo el primer pedido")
	assert.Equal(t, before+1, pokerStoredVersion(t, f.env.db, f.session.ID), "un único cambio de facilitador sube la versión en uno")
	for name, stream := range map[string]*pokerStream{"Bruno": f.bruno, "Carla": f.carla} {
		snapshot := stream.NextUntil(t, "el nuevo facilitador en el canal de "+name, facilitatorIs(winnerID))
		requireFacilitatorRole(t, snapshot, winnerID, "canal de "+name)
		assert.Equal(t, before+1, snapshot.Version, "canal de %s: el cambio de facilitador es un único hecho", name)
	}
}

// TestPokerSweep_MembresiaRevocada cubre la baja de un participante conectado del proyecto (FR-019).
// Con Ana, Bruno, Carla y Diego conectados, la ronda 1 se revela con el 8 de Diego y en la ronda 2
// Ana, Bruno y Carla votan 5 y Diego no. Ana da de baja a Diego con DELETE
// /api/v1/projects/{projectId}/members/{userId} de la 002 y el barrido lo detecta: el canal de Diego
// recibe el evento removed y se cierra; Diego deja de contar para la revelación, así que los canales
// de los demás reciben la ronda 2 revelada con los tres votos y a Diego desconectado; en la base
// Diego queda desconectado y su voto de la ronda 1 se conserva.
func TestPokerSweep_MembresiaRevocada(t *testing.T) {
	f := newPokerUS3Fixture(t)
	for token, card := range map[string]string{f.anaToken: "3", f.brunoToken: "5", f.carlaToken: "13", f.diegoToken: "8"} {
		f.env.requireVoted(t, token, f.session.ID, card)
	}
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "5")
	}
	requireStoredVoting(t, f.env.db, f.session.ID, "Diego todavía no votó en la ronda 2")

	requirePokerStatus(t, f.env.do(t, http.MethodDelete,
		"/api/v1/projects/"+f.projectID.String()+"/members/"+f.diegoID.String(), f.anaToken, nil), http.StatusNoContent, nil)
	f.env.sweep(t, time.Second)

	// El canal de Diego recibe removed y se cierra.
	f.diego.requireRemoved(t, "canal de Diego")
	f.diego.requireEnded(t, "canal de Diego")

	// Sin Diego, la ronda 2 se revela con los votos de los demás.
	votes := map[uuid.UUID]string{f.anaID: "5", f.brunoID: "5", f.carlaID: "5"}
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la ronda 2 revelada sin Diego en el canal de "+name, func(s pokerSessionBody) bool {
			return participantGone(f.diegoID)(s) && currentRoundRevealed(s)
		})
		requireConsensus(t, requireRevealedRound(t, snapshot, votes, nil, "canal de "+name), 5, "canal de "+name)
	}
	assert.False(t, pokerStoredParticipant(t, f.env.db, f.session.ID, f.diegoID).Connected, "Diego queda desconectado en la base")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.diegoID, "8")
}

// TestPokerSweep_CambioDeHistoria cubre el reflejo en la sesión de los cambios de la historia
// (FR-074). Con Ana, Bruno y Carla conectados, Bruno cambia el título y los criterios de la historia
// con PATCH /api/v1/stories/{storyId} de la 003 y el barrido lo detecta: los tres canales reciben, sin
// recargar y en menos de 2 segundos, un state con el título y los criterios nuevos en su orden y la
// misma version, porque no es un hecho de la sesión; en la base la versión no cambia.
func TestPokerSweep_CambioDeHistoria(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)
	title := "Inicio de sesión con correo o teléfono"
	criteria := []string{
		"El formulario acepta un teléfono con código de área.",
		"El formulario rechaza un correo mal formado.",
		"Tras ingresar se muestra la lista de proyectos.",
	}

	requirePokerStatus(t, f.env.do(t, http.MethodPatch, "/api/v1/stories/"+f.storyID.String(), f.brunoToken,
		map[string]any{"title": title, "acceptanceCriteria": criteria}), http.StatusOK, nil)
	f.env.sweep(t, time.Second)

	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "el título nuevo en el canal de "+name, func(s pokerSessionBody) bool {
			return s.Story.Title == title
		})
		assert.Equal(t, criteria, snapshot.Story.AcceptanceCriteria, "canal de %s: criterios nuevos en su orden", name)
		assert.Equal(t, before, snapshot.Version, "canal de %s: el cambio de la historia no sube la versión", name)
	}
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "el cambio de la historia no es un hecho de la sesión")
}

// TestPokerPresence_ReinicioDelServidor cubre el arranque del servidor con sesiones en curso
// (research.md sección 9). Con Ana y Bruno conectados a una sesión abierta y Carla figurando
// conectada en una sesión ya finalizada de otra historia, el servidor se reinicia: ResetPresence deja
// desconectados solo a Ana y Bruno, con disconnected_at igual al instante recibido, sin cambiar la
// versión ni last_activity_at de la sesión, y Carla sigue como estaba. Al reabrir su canal, Ana y
// Bruno se reincorporan cada uno como un hecho nuevo: quedan conectados y la versión sube en dos.
func TestPokerPresence_ReinicioDelServidor(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	both := allConnected(f.anaID, f.brunoID)
	ana.NextUntil(t, "Ana y Bruno conectados en el canal de Ana", both)
	bruno.NextUntil(t, "Ana y Bruno conectados en el canal de Bruno", both)

	// Carla figura conectada en una sesión finalizada de otra historia.
	five := 5
	otherStoryID := f.env.createStory(t, f.anaToken, f.projectID, pokerStoryInput{Title: "Exportar el reporte"})
	closed := pokerSeedClosedSession(t, f.env.db, otherStoryID, f.carlaID, domain.SessionStatusFinished, &five, pokerFixedNow.Add(-2*time.Hour))
	require.NoError(t, f.env.db.Model(&domain.EstimationParticipant{}).
		Where("session_id = ? AND user_id = ?", closed.session.ID, f.carlaID).
		Updates(map[string]any{"connected": true, "disconnected_at": nil}).Error, "no se pudo dejar conectada a Carla")
	stored := pokerStoredSession(t, f.env.db, session.ID)

	f.env.clock.Advance(10 * time.Minute)
	f.env.restart(t)

	// Solo los participantes de la sesión abierta quedan desconectados, sin un hecho.
	resetAt := f.env.clock.Now()
	for _, id := range []uuid.UUID{f.anaID, f.brunoID} {
		p := pokerStoredParticipant(t, f.env.db, session.ID, id)
		assert.False(t, p.Connected, "%s queda desconectado tras el reinicio", id)
		if assert.NotNil(t, p.DisconnectedAt, "%s debería tener momento de desconexión", id) {
			assert.True(t, p.DisconnectedAt.Equal(resetAt), "disconnected_at = %s, se esperaba %s", p.DisconnectedAt, resetAt)
		}
	}
	afterReset := pokerStoredSession(t, f.env.db, session.ID)
	assert.Equal(t, stored.Version, afterReset.Version, "el reinicio no sube la versión")
	assert.True(t, stored.LastActivityAt.Equal(afterReset.LastActivityAt), "el reinicio no toca last_activity_at")
	assert.True(t, pokerStoredParticipant(t, f.env.db, closed.session.ID.String(), f.carlaID).Connected,
		"la sesión cerrada no se toca")

	// Al reabrir el canal, cada uno se reincorpora como un hecho nuevo.
	f.env.openStream(t, f.anaToken, session.ID).NextUntil(t, "Ana conectada tras el reinicio", participantBack(f.anaID))
	f.env.openStream(t, f.brunoToken, session.ID).NextUntil(t, "Ana y Bruno conectados tras el reinicio", both)
	rejoined := pokerStoredSession(t, f.env.db, session.ID)
	assert.Equal(t, stored.Version+2, rejoined.Version, "cada reincorporación es un hecho")
	assert.True(t, rejoined.LastActivityAt.Equal(resetAt), "last_activity_at = %s, se esperaba %s", rejoined.LastActivityAt, resetAt)
	for _, id := range []uuid.UUID{f.anaID, f.brunoID} {
		p := pokerStoredParticipant(t, f.env.db, session.ID, id)
		assert.True(t, p.Connected, "%s vuelve a estar conectado", id)
		assert.Nil(t, p.DisconnectedAt, "%s ya no tiene momento de desconexión", id)
	}
}

// Pruebas de integración de punta a punta de la historia 7 de Planning Poker (T079): cancelan la
// sesión con POST /api/v1/estimation-sessions/{sessionId}/cancel y llaman al barrido con el reloj
// falso para la cancelación automática por inactividad, por historia completada y por proyecto
// finalizado, sobre el mismo router compuesto como cmd/app y contra smye_test. Verifican la
// respuesta, el último estado que llega por los canales ya abiertos antes de que el servidor los
// cierre, la sesión guardada en estimation_sessions y los Story Points de la historia leídos con la
// 003, que la cancelación no toca (FR-057 a FR-063, RN-21, RN-30, RN-31). También van acá, sin
// etiqueta de historia, la cancelación por historia completada y por proyecto finalizado, la acción
// que llega antes del barrido y el canal pedido sobre una sesión cerrada (FR-018).

// Textos exactos que fija el contrato para el 409 de sesión cerrada (research.md sección 19), la
// ventana de inactividad de FR-063 y el margen con que cada hecho llega antes de cumplirse.
const (
	pokerSessionClosedMessage     = "La sesión de estimación ya está cerrada."
	pokerInactivityClosedMessage  = "La sesión de estimación se cerró por inactividad."
	pokerInactivityWindow         = 60 * time.Minute
	pokerInactivityFactLeadMargin = time.Minute
)

// cancelPath es la ruta de la cancelación de la sesión según contracts/openapi.yaml.
func cancelPath(sessionID string) string {
	return "/api/v1/estimation-sessions/" + sessionID + "/cancel"
}

// cancel pide POST /api/v1/estimation-sessions/{sessionId}/cancel, sin cuerpo, con el token como
// Bearer.
func (e *pokerTestEnv) cancel(t *testing.T, token, sessionID string) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodPost, cancelPath(sessionID), token, nil)
}

// requireCanceled cancela la sesión, exige el 200 y devuelve la sesión resultante vista por quien la
// canceló.
func (e *pokerTestEnv) requireCanceled(t *testing.T, token, sessionID string) pokerSessionBody {
	t.Helper()
	var session pokerSessionBody
	resp := e.cancel(t, token, sessionID)
	requirePokerStatus(t, resp, http.StatusOK, &session)
	session.Raw = resp.Body
	return session
}

// sweepAt adelanta el reloj falso hasta el instante indicado y ejecuta una pasada del barrido.
func (e *pokerTestEnv) sweepAt(t *testing.T, at time.Time) {
	t.Helper()
	e.sweep(t, at.Sub(e.clock.Now()))
}

// sessionCanceled es la condición "la sesión está cancelada", para esperar con NextUntil.
func sessionCanceled(s pokerSessionBody) bool {
	return s.Status == "canceled"
}

// requireCanceledByFacilitator exige que la instantánea muestre la sesión cancelada por la persona
// indicada, con su momento de cierre, sin valor acordado ni motivo del sistema (FR-058).
func requireCanceledByFacilitator(t *testing.T, snapshot pokerSessionBody, closedBy uuid.UUID, where string) {
	t.Helper()
	assert.Equal(t, "canceled", snapshot.Status, "%s: estado de la sesión: %s", where, snapshot.Raw)
	assert.Nil(t, snapshot.AgreedStoryPoints, "%s: una sesión cancelada no tiene valor acordado", where)
	if assert.NotNil(t, snapshot.ClosedBy, "%s: closedBy no debería ser null", where) {
		assert.Equal(t, closedBy.String(), snapshot.ClosedBy.ID, "%s: quien canceló la sesión", where)
	}
	assert.NotNil(t, snapshot.ClosedAt, "%s: closedAt no debería ser null", where)
	assert.Nil(t, snapshot.CancelReason, "%s: la cancelación del facilitador no lleva motivo del sistema", where)
}

// requireCanceledBySystem exige que la instantánea muestre la sesión cancelada por el sistema con el
// motivo indicado: sin quien la cerró, con su momento de cierre y sin valor acordado (FR-061, FR-063).
func requireCanceledBySystem(t *testing.T, snapshot pokerSessionBody, reason domain.CancelReason, where string) {
	t.Helper()
	assert.Equal(t, "canceled", snapshot.Status, "%s: estado de la sesión: %s", where, snapshot.Raw)
	assert.Nil(t, snapshot.AgreedStoryPoints, "%s: una sesión cancelada no tiene valor acordado", where)
	assert.Nil(t, snapshot.ClosedBy, "%s: la cancelación del sistema no tiene quien la cerró", where)
	assert.NotNil(t, snapshot.ClosedAt, "%s: closedAt no debería ser null", where)
	if assert.NotNil(t, snapshot.CancelReason, "%s: cancelReason no debería ser null", where) {
		assert.Equal(t, string(reason), *snapshot.CancelReason, "%s: motivo de la cancelación", where)
	}
}

// requireStoredCanceled exige que la sesión quede cancelada en la base en el instante indicado del
// reloj falso, sin valor acordado, con closed_by_id igual a closedBy (nil en las cancelaciones del
// sistema) y cancel_reason igual a reason (nil en la del facilitador), según data-model.md sección 3.
func requireStoredCanceled(t *testing.T, db *gorm.DB, sessionID string, closedBy *uuid.UUID, reason *domain.CancelReason, at time.Time) {
	t.Helper()
	stored := pokerStoredSession(t, db, sessionID)
	assert.Equal(t, domain.SessionStatusCanceled, stored.Status, "la sesión debería quedar cancelada en la base")
	assert.Nil(t, stored.AgreedStoryPoints, "una sesión cancelada no tiene agreed_story_points")
	assert.Equal(t, closedBy, stored.ClosedByID, "closed_by_id")
	assert.Equal(t, reason, stored.CancelReason, "cancel_reason")
	if assert.NotNil(t, stored.ClosedAt, "closed_at no debería ser nulo") {
		assert.True(t, stored.ClosedAt.Equal(at), "closed_at = %s, se esperaba %s", stored.ClosedAt, at)
	}
}

// requireSessionClosedBody exige el 409 de sesión cerrada del contrato con el mensaje, el estado, el
// valor acordado y el motivo indicados; nil en agreed o en reason es null en el cuerpo.
func requireSessionClosedBody(t *testing.T, resp pokerResponse, message, status string, agreed *int, reason *domain.CancelReason) {
	t.Helper()
	want, err := json.Marshal(map[string]any{
		"error":             message,
		"status":            status,
		"agreedStoryPoints": agreed,
		"cancelReason":      reason,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, string(want), string(resp.Body))
}

// pokerReason devuelve un puntero al motivo de cancelación indicado.
func pokerReason(reason domain.CancelReason) *domain.CancelReason {
	return &reason
}

// pokerSessionVotes cuenta las filas de estimation_votes de todas las rondas de la sesión.
func pokerSessionVotes(t *testing.T, db *gorm.DB, sessionID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.EstimationVote{}).
		Joins("JOIN estimation_rounds ON estimation_rounds.id = estimation_votes.round_id").
		Where("estimation_rounds.session_id = ?", sessionID).Count(&n).Error,
		"no se pudieron contar los votos de la sesión %s", sessionID)
	return n
}

// requireStreamsCanceled exige que cada canal reciba la sesión cancelada por el sistema con el motivo
// indicado, sin recargar y en menos de 2 segundos, y que el servidor lo cierre después.
func requireStreamsCanceled(t *testing.T, streams map[string]*pokerStream, reason domain.CancelReason) {
	t.Helper()
	for name, stream := range streams {
		snapshot := stream.NextUntil(t, "la sesión cancelada en el canal de "+name, sessionCanceled)
		requireCanceledBySystem(t, snapshot, reason, "canal de "+name)
		stream.requireEnded(t, "canal de "+name)
	}
}

// Escenario: Cancelación conserva la estimación (005/US7)
// Con Ana, Bruno y Carla conectados sobre una historia estimada en 5 Story Points desde el backlog de
// la 003 y el voto 8 de Bruno, un minuto después Ana, facilitadora, cancela: responde 200 con la
// sesión cancelada por Ana, sin valor acordado ni motivo del sistema; los tres canales reciben ese
// último estado sin recargar y en menos de 2 segundos, y el servidor los cierra. En la base la sesión
// queda cancelada con closed_at en el instante del reloj y la versión sube en uno, y la historia sigue
// en 5 Story Points (FR-057, FR-058, FR-073, RN-21).
func TestUS7_CancelacionConservaEstimacion(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireStoryEstimated(t, f.brunoToken, f.storyID, 5)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	before := pokerStoredVersion(t, f.env.db, f.session.ID)
	f.env.clock.Advance(time.Minute)

	resp := f.env.requireCanceled(t, f.anaToken, f.session.ID)
	requireCanceledByFacilitator(t, resp, f.anaID, "respuesta de Ana")

	// Cada canal recibe el estado final y después se cierra.
	for name, stream := range f.threeStreams() {
		snapshot := stream.NextUntil(t, "la sesión cancelada en el canal de "+name, sessionCanceled)
		requireCanceledByFacilitator(t, snapshot, f.anaID, "canal de "+name)
		stream.requireEnded(t, "canal de "+name)
	}

	requireStoredCanceled(t, f.env.db, f.session.ID, &f.anaID, nil, f.env.clock.Now())
	assert.Equal(t, before+1, pokerStoredVersion(t, f.env.db, f.session.ID), "cancelar cierra la sesión y sube la versión")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.brunoToken, f.storyID), pokerIntPtr(5), "historia de la 003")
}

// Escenario: Nueva sesión tras cancelar (005/US7)
// Ana abre una sesión sola y la cancela. Bruno abre una sesión nueva sobre la misma historia: responde
// 201 con un identificador distinto y su ronda 1 en votación, porque la historia ya no tiene ninguna
// sesión abierta; la historia queda con dos sesiones, una cancelada y otra open (FR-062, RN-04).
func TestUS7_NuevaSesionTrasCancelar(t *testing.T) {
	f := newPokerUS1Fixture(t)
	canceled, _ := f.newPokerSoloSession(t, f.storyID)
	f.env.requireCanceled(t, f.anaToken, canceled.ID)

	session := f.env.requireOpenedSession(t, f.brunoToken, f.storyID)

	assert.NotEqual(t, canceled.ID, session.ID, "la sesión nueva tiene su propio identificador")
	requireFirstRoundVoting(t, session)
	assert.Equal(t, int64(2), pokerSessionRows(t, f.env.db, f.storyID))
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusOpen))
	assert.Equal(t, int64(1), pokerSessionRows(t, f.env.db, f.storyID, domain.SessionStatusCanceled))
}

// Escenario: Rondas conservadas (005/US7)
// Con tres rondas reveladas con los votos 3, 5 y 13 de Ana, Bruno y Carla, Ana cancela: la consulta de
// la sesión de Bruno con GET /api/v1/estimation-sessions/{sessionId} muestra la sesión cancelada con
// las tres rondas reveladas, cada una con sus tres votos y la divergencia de 3 a 13, y en la base
// quedan las tres rondas con sus nueve votos (FR-058, RN-28).
func TestUS7_RondasConservadas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	for number := 2; number <= 3; number++ {
		f.env.requireStartedRound(t, f.anaToken, f.session.ID)
		f.revealDivergentRound(t)
	}

	f.env.requireCanceled(t, f.anaToken, f.session.ID)

	// La consulta de la sesión cancelada muestra las tres rondas completas.
	session := f.env.requireSessionFetched(t, f.brunoToken, f.session.ID)
	requireCanceledByFacilitator(t, session, f.anaID, "consulta de Bruno")
	rounds := pokerAllRounds(t, session)
	require.Len(t, rounds, 3, "rondas: %s", session.Raw)
	for i, round := range rounds {
		where := fmt.Sprintf("ronda %d", i+1)
		assert.Equal(t, i+1, round.Number, "%s: número", where)
		assert.Equal(t, "revealed", round.Status, "%s: estado", where)
		assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "%s: votos", where)
		requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, where)
	}

	// En la base quedan las tres rondas con sus tres votos cada una.
	stored := pokerStoredRounds(t, f.env.db, f.session.ID)
	require.Len(t, stored, 3, "la sesión debería conservar sus rondas en la base")
	for _, round := range stored {
		assert.Equal(t, domain.RoundStatusRevealed, round.Status, "ronda %d", round.Number)
		assert.EqualValues(t, 3, pokerStoredRoundVotes(t, f.env.db, round.ID), "ronda %d conserva sus votos", round.Number)
	}
}

// Escenario: Cancelación sin votos (005/US7)
// Ana abre una sesión sola y, sin que nadie vote, la cancela: responde 200 con la sesión cancelada por
// Ana y su única ronda, la 1, todavía en votación; en la base la sesión queda cancelada sin ninguna
// ronda revelada y sin votos (FR-057, FR-058).
func TestUS7_CancelacionSinVotos(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session, _ := f.newPokerSoloSession(t, f.storyID)

	resp := f.env.requireCanceled(t, f.anaToken, session.ID)

	requireCanceledByFacilitator(t, resp, f.anaID, "respuesta de Ana")
	if assert.Len(t, resp.Rounds, 1, "rondas: %s", resp.Raw) {
		assert.Equal(t, "voting", resp.Rounds[0].Status, "la ronda 1 nunca se reveló")
	}
	requireStoredCanceled(t, f.env.db, session.ID, &f.anaID, nil, f.env.clock.Now())
	rounds := pokerStoredRounds(t, f.env.db, session.ID)
	if assert.Len(t, rounds, 1, "la sesión conserva su única ronda") {
		assert.Equal(t, domain.RoundStatusVoting, rounds[0].Status, "la sesión queda sin ninguna ronda revelada")
	}
	assert.Zero(t, pokerSessionVotes(t, f.env.db, session.ID), "la sesión no tiene votos")
}

// Escenario: Cancelación por inactividad (005/US7)
// Con Ana, Bruno y Carla conectados sobre una historia estimada en 5 Story Points y el voto 8 de
// Bruno como último hecho, a los 59:59 minutos el barrido no cancela nada; a los 60 minutos cancela la
// sesión por inactividad: los tres canales reciben la sesión cancelada con cancelReason inactivity y
// sin quien la cerró, y el servidor los cierra. En la base la sesión queda cancelada con closed_by_id
// nulo, cancel_reason inactivity y closed_at en el instante del barrido; el voto se conserva, la
// historia sigue en 5 y Bruno puede abrir una sesión nueva sobre ella (FR-063, RN-31, SC-021).
func TestUS7_CancelacionPorInactividad(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireStoryEstimated(t, f.brunoToken, f.storyID, 5)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "8")
	lastFact := pokerStoredSession(t, f.env.db, f.session.ID).LastActivityAt

	// Un segundo antes del límite la sesión sigue abierta.
	f.env.sweepAt(t, lastFact.Add(pokerInactivityWindow-time.Second))
	requireStoredOpen(t, f.env.db, f.session.ID, "a los 59:59 minutos sin hechos todavía no caduca")

	// Al cumplirse los 60 minutos el barrido la cancela y avisa a los conectados.
	f.env.sweepAt(t, lastFact.Add(pokerInactivityWindow))
	requireStreamsCanceled(t, f.threeStreams(), domain.CancelReasonInactivity)

	requireStoredCanceled(t, f.env.db, f.session.ID, nil, pokerReason(domain.CancelReasonInactivity), f.env.clock.Now())
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, "8")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.brunoToken, f.storyID), pokerIntPtr(5), "historia de la 003")
	session := f.env.requireOpenedSession(t, f.brunoToken, f.storyID)
	assert.NotEqual(t, f.session.ID, session.ID, "la sesión nueva tiene su propio identificador")
}

// Escenario: Hecho reinicia la inactividad (005/US7)
// Sobre el "Dado" de US2, cada tipo de hecho de data-model.md sección 7 ocurre a los 59 minutos del
// último hecho anterior, en su subtest: la incorporación de Diego, la desconexión de Carla al vencer
// su gracia, el voto de Bruno, la revelación manual de Ana, la ronda nueva de Ana y el cambio de
// facilitador a Bruno. El hecho fija last_activity_at en ese instante; a los 60 minutos del inicio el
// barrido deja la sesión open, y recién a los 60 minutos del hecho la cancela por inactividad (FR-063,
// RN-31, SC-021).
func TestUS7_HechoReiniciaInactividad(t *testing.T) {
	// Cada caso prepara su "Dado" y produce su hecho; lead es cuánto antes del minuto 59 empieza la
	// acción, para que el hecho quede registrado justo en él.
	facts := []struct {
		name    string
		lead    time.Duration
		prepare func(t *testing.T, f pokerUS2Fixture)
		act     func(t *testing.T, f pokerUS2Fixture)
	}{
		{
			name: "incorporación",
			prepare: func(t *testing.T, f pokerUS2Fixture) {
				f.env.addMember(t, f.anaToken, f.projectID, "diego@example.com")
			},
			act: func(t *testing.T, f pokerUS2Fixture) {
				f.env.openStream(t, f.diegoToken, f.session.ID).NextUntil(t, "Diego conectado", participantBack(f.diegoID))
			},
		},
		{
			name: "desconexión",
			lead: 30 * time.Second,
			act: func(t *testing.T, f pokerUS2Fixture) {
				f.env.dropAndExpire(t, f.carla)
			},
		},
		{
			name: "voto",
			act: func(t *testing.T, f pokerUS2Fixture) {
				f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
			},
		},
		{
			name: "revelación",
			prepare: func(t *testing.T, f pokerUS2Fixture) {
				f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
			},
			act: func(t *testing.T, f pokerUS2Fixture) {
				requirePokerStatus(t, f.env.reveal(t, f.anaToken, f.session.ID), http.StatusOK, nil)
			},
		},
		{
			name: "ronda nueva",
			prepare: func(t *testing.T, f pokerUS2Fixture) {
				f.revealDivergentRound(t)
			},
			act: func(t *testing.T, f pokerUS2Fixture) {
				f.env.requireStartedRound(t, f.anaToken, f.session.ID)
			},
		},
		{
			name: "cambio de facilitador",
			prepare: func(t *testing.T, f pokerUS2Fixture) {
				f.env.dropAndExpire(t, f.ana)
			},
			act: func(t *testing.T, f pokerUS2Fixture) {
				f.env.requireTookFacilitator(t, f.brunoToken, f.session.ID)
			},
		},
	}
	for _, tc := range facts {
		t.Run(tc.name, func(t *testing.T) {
			f := newPokerUS2Fixture(t)
			if tc.prepare != nil {
				tc.prepare(t, f)
			}
			start := pokerStoredSession(t, f.env.db, f.session.ID).LastActivityAt
			factAt := start.Add(pokerInactivityWindow - pokerInactivityFactLeadMargin)

			// El hecho ocurre a los 59 minutos del inicio y reinicia la cuenta.
			f.env.clock.Advance(factAt.Add(-tc.lead).Sub(f.env.clock.Now()))
			tc.act(t, f)
			stored := pokerStoredSession(t, f.env.db, f.session.ID)
			require.True(t, stored.LastActivityAt.Equal(factAt),
				"last_activity_at = %s, se esperaba el instante del hecho %s", stored.LastActivityAt, factAt)

			// A los 60 minutos del inicio la sesión sigue abierta.
			f.env.sweepAt(t, start.Add(pokerInactivityWindow))
			requireStoredOpen(t, f.env.db, f.session.ID, "el hecho del minuto 59 reinició la inactividad")

			// A los 60 minutos del hecho caduca por inactividad.
			f.env.sweepAt(t, factAt.Add(pokerInactivityWindow))
			requireStoredCanceled(t, f.env.db, f.session.ID, nil, pokerReason(domain.CancelReasonInactivity), f.env.clock.Now())
		})
	}
}

// Escenario: Cancelar sin ser facilitador (005/US7)
// Con Ana, facilitadora, Bruno y Carla conectados, Bruno intenta cancelar: responde 403 con el
// mensaje de que la acción está reservada al facilitador; la sesión sigue abierta y la versión no
// cambia, porque el rechazo no es un hecho (FR-059, FR-064, SC-011).
func TestUS7_CancelarSinSerFacilitador(t *testing.T) {
	f := newPokerUS2Fixture(t)
	before := pokerStoredVersion(t, f.env.db, f.session.ID)

	resp := f.env.cancel(t, f.brunoToken, f.session.ID)

	assert.Equal(t, http.StatusForbidden, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerFacilitatorOnlyBodyJSON, string(resp.Body))
	requireStoredOpen(t, f.env.db, f.session.ID, "el rechazo no cancela la sesión")
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, f.session.ID), "un rechazo no sube la versión")
}

// Escenario: Cancelar sesión finalizada (005/US7)
// Ana abre una sesión sola, vota 8 y registra 8. Después intenta cancelarla: responde 409 de sesión
// cerrada con el estado finished y el valor 8; la sesión sigue finalizada con su valor y su cierre, la
// versión no cambia y la historia sigue en 8 Story Points (FR-060, RN-22).
func TestUS7_CancelarSesionFinalizada(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session, _ := f.newPokerSoloSession(t, f.storyID)
	f.env.requireVoted(t, f.anaToken, session.ID, "8")
	f.env.requireFinalized(t, f.anaToken, session.ID, 8)
	finishedAt := f.env.clock.Now()
	before := pokerStoredVersion(t, f.env.db, session.ID)
	f.env.clock.Advance(time.Minute)

	resp := f.env.cancel(t, f.anaToken, session.ID)

	requireSessionClosedBody(t, resp, pokerSessionClosedMessage, "finished", pokerIntPtr(8), nil)
	requireStoredFinished(t, f.env.db, session.ID, 8, f.anaID, finishedAt)
	assert.Equal(t, before, pokerStoredVersion(t, f.env.db, session.ID), "un rechazo no sube la versión")
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), pokerIntPtr(8), "historia de la 003")
}

// Escenario: Acción tras inactividad (005/US7)
// Con Ana, Bruno y Carla conectados y el voto 5 de Bruno, la sesión caduca por inactividad al pasar 60
// minutos y el barrido la cancela. Carla intenta votar y Ana intenta registrar 8, cada uno en su
// subtest: los dos reciben 409 con el mensaje de que la sesión se cerró por inactividad, el estado
// canceled y cancelReason inactivity; Carla no tiene voto y la historia sigue sin estimar (FR-063,
// research.md sección 19).
func TestUS7_AccionTrasInactividad(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.env.requireVoted(t, f.brunoToken, f.session.ID, "5")
	lastFact := pokerStoredSession(t, f.env.db, f.session.ID).LastActivityAt
	f.env.sweepAt(t, lastFact.Add(pokerInactivityWindow))
	inactivity := pokerReason(domain.CancelReasonInactivity)

	t.Run("votar", func(t *testing.T) {
		resp := f.env.vote(t, f.carlaToken, f.session.ID, map[string]any{"card": "3"})

		requireSessionClosedBody(t, resp, pokerInactivityClosedMessage, "canceled", nil, inactivity)
		assert.Empty(t, pokerStoredVotes(t, f.env.db, f.session.ID, f.carlaID), "el voto rechazado no queda escrito")
	})
	t.Run("registrar", func(t *testing.T) {
		resp := f.env.finalize(t, f.anaToken, f.session.ID, map[string]any{"storyPoints": 8})

		requireSessionClosedBody(t, resp, pokerInactivityClosedMessage, "canceled", nil, inactivity)
		requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, f.storyID), nil, "historia de la 003")
	})
}

// newPokerCommittedStory crea, como Bruno, una historia lista con 5 Story Points y un criterio, y la
// compromete en un sprint Activo del proyecto, para poder llevarla a Completada con la 004.
func (f pokerUS1Fixture) newPokerCommittedStory(t *testing.T) uuid.UUID {
	t.Helper()
	storyID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{
		Title:    "Exportar el reporte",
		Points:   pokerIntPtr(5),
		Criteria: []string{"El PDF incluye la velocidad de los últimos cinco sprints."},
	})
	pokerSeedActiveCommitment(t, f.env.db, f.projectID, f.anaID, storyID)
	return storyID
}

// completeStory lleva la historia a En progreso y después a Completada con PUT
// /api/v1/stories/{storyId}/status de la 004.
func (e *pokerTestEnv) completeStory(t *testing.T, token string, storyID uuid.UUID) {
	t.Helper()
	for _, status := range []string{"in_progress", "completed"} {
		requirePokerStatus(t, e.do(t, http.MethodPut, "/api/v1/stories/"+storyID.String()+"/status", token,
			map[string]any{"status": status}), http.StatusOK, nil)
	}
}

// TestPokerAutoCancel_HistoriaCompletada cubre la cancelación del sistema por historia completada
// (FR-061, RN-30). Con Ana conectada a una sesión sobre una historia comprometida en el sprint Activo,
// Bruno la avanza a Completada con la 004 y el barrido lo detecta: el canal de Ana recibe la sesión
// cancelada con cancelReason story_completed y sin quien la cerró, y el servidor lo cierra. En la base
// la sesión queda cancelada con closed_by_id nulo y ese motivo, y la historia conserva sus 5 Story
// Points.
func TestPokerAutoCancel_HistoriaCompletada(t *testing.T) {
	f := newPokerUS1Fixture(t)
	storyID := f.newPokerCommittedStory(t)
	session, ana := f.newPokerSoloSession(t, storyID)

	f.env.completeStory(t, f.brunoToken, storyID)
	f.env.sweep(t, time.Second)

	requireStreamsCanceled(t, map[string]*pokerStream{"Ana": ana}, domain.CancelReasonStoryCompleted)
	requireStoredCanceled(t, f.env.db, session.ID, nil, pokerReason(domain.CancelReasonStoryCompleted), f.env.clock.Now())
	requireStoryPoints(t, f.env.requireStoryDetail(t, f.anaToken, storyID), pokerIntPtr(5), "historia de la 003")
}

// TestPokerAutoCancel_ProyectoFinalizado cubre la cancelación del sistema por proyecto finalizado
// (FR-061, RN-30). Con Ana, Bruno y Carla conectados, Ana finaliza el proyecto con la 002 y el barrido
// lo detecta: los tres canales reciben la sesión cancelada con cancelReason project_finished y se
// cierran, y en la base queda cancelada con closed_by_id nulo y ese motivo. Un voto posterior de Bruno
// responde 409 con el mensaje de proyecto de solo lectura y no queda escrito (research.md sección
// 19).
func TestPokerAutoCancel_ProyectoFinalizado(t *testing.T) {
	f := newPokerUS2Fixture(t)

	f.env.setProjectStatus(t, f.anaToken, f.projectID, "finished")
	f.env.sweep(t, time.Second)

	requireStreamsCanceled(t, f.threeStreams(), domain.CancelReasonProjectFinished)
	requireStoredCanceled(t, f.env.db, f.session.ID, nil, pokerReason(domain.CancelReasonProjectFinished), f.env.clock.Now())

	resp := f.env.vote(t, f.brunoToken, f.session.ID, map[string]any{"card": "5"})
	requireSessionClosedBody(t, resp, pokerProjectFinishedMessage, "canceled", nil, pokerReason(domain.CancelReasonProjectFinished))
	assert.Empty(t, pokerStoredVotes(t, f.env.db, f.session.ID, f.brunoID), "el voto rechazado no queda escrito")
}

// TestPokerJoin_SesionCerrada cubre la incorporación a una sesión cerrada (FR-018, RN-22). Con una
// sesión que Ana dejó finalizada con valor 8 y otra que canceló, cada una en su subtest, Bruno, que
// nunca participó, pide el canal: responde 409 en JSON y no como text/event-stream, con el
// SessionClosedResponse del contrato con el estado y el valor acordado registrados, sin ningún evento
// en el cuerpo, y Bruno no queda como participante.
func TestPokerJoin_SesionCerrada(t *testing.T) {
	// Cada caso cierra la sesión de una forma y declara el estado y el valor que debe informar el 409.
	closures := []struct {
		name   string
		close  func(t *testing.T, f pokerUS1Fixture, sessionID string)
		status string
		agreed *int
	}{
		{
			name: "finalizada con valor 8",
			close: func(t *testing.T, f pokerUS1Fixture, sessionID string) {
				f.env.requireVoted(t, f.anaToken, sessionID, "8")
				f.env.requireFinalized(t, f.anaToken, sessionID, 8)
			},
			status: "finished",
			agreed: pokerIntPtr(8),
		},
		{
			name: "cancelada por el facilitador",
			close: func(t *testing.T, f pokerUS1Fixture, sessionID string) {
				f.env.requireCanceled(t, f.anaToken, sessionID)
			},
			status: "canceled",
		},
	}
	for _, tc := range closures {
		t.Run(tc.name, func(t *testing.T) {
			f := newPokerUS1Fixture(t)
			session, _ := f.newPokerSoloSession(t, f.storyID)
			tc.close(t, f, session.ID)

			resp := f.env.do(t, http.MethodGet, eventsPath(session.ID), f.brunoToken, nil)

			requireSessionClosedBody(t, resp, pokerSessionClosedMessage, tc.status, tc.agreed, nil)
			assert.Contains(t, resp.ContentType, "application/json", "el rechazo no abre el canal de eventos")
			assert.NotContains(t, string(resp.Body), "event:", "el rechazo no envía ningún evento")
			var n int64
			require.NoError(t, f.env.db.Model(&domain.EstimationParticipant{}).
				Where("session_id = ? AND user_id = ?", session.ID, f.brunoID).Count(&n).Error)
			assert.Zero(t, n, "Bruno no debería quedar como participante")
		})
	}
}

// TestPokerAutoCancel_AccionAntesDelBarrido cubre la acción que llega después de que la historia se
// completa y antes de la pasada del barrido (data-model.md sección 9). Con Ana conectada a una sesión
// sobre una historia comprometida, Bruno la avanza a Completada con la 004 y, sin barrido, Ana vota 5:
// responde 409 de sesión cerrada con el estado canceled y cancelReason story_completed, el voto no
// queda escrito, la cancelación queda confirmada en la base y el canal de Ana recibe la sesión
// cancelada y se cierra.
func TestPokerAutoCancel_AccionAntesDelBarrido(t *testing.T) {
	f := newPokerUS1Fixture(t)
	storyID := f.newPokerCommittedStory(t)
	session, ana := f.newPokerSoloSession(t, storyID)
	f.env.completeStory(t, f.brunoToken, storyID)

	resp := f.env.vote(t, f.anaToken, session.ID, map[string]any{"card": "5"})

	storyCompleted := pokerReason(domain.CancelReasonStoryCompleted)
	requireSessionClosedBody(t, resp, pokerSessionClosedMessage, "canceled", nil, storyCompleted)
	assert.Empty(t, pokerStoredVotes(t, f.env.db, session.ID, f.anaID), "el voto rechazado no queda escrito")
	requireStoredCanceled(t, f.env.db, session.ID, nil, storyCompleted, f.env.clock.Now())
	requireStreamsCanceled(t, map[string]*pokerStream{"Ana": ana}, domain.CancelReasonStoryCompleted)
}

// Pruebas de integración de punta a punta de la historia 8 de Planning Poker (T085): consultan el
// historial de estimación de una historia con GET /api/v1/stories/{storyId}/estimation-sessions sobre
// el mismo router compuesto como cmd/app y contra smye_test. El historial trae el resumen de cada
// sesión cerrada, sin votos, y la sesión abierta sin votos (research.md sección 8); las rondas con el
// voto de cada participante se leen con GET /api/v1/estimation-sessions/{sessionId}. Las sesiones se
// abren, se votan, se registran y se cancelan con los endpoints de US1 a US7, y la eliminación
// bloqueada se pide con el DELETE /api/v1/stories/{storyId} de la 003.

// Textos exactos del 404 de historia de la 003 y del 409 de eliminación bloqueada por sesiones de
// estimación (data-model.md sección 7 de la 003).
const (
	pokerStoryNotFoundBody         = `{"error":"La historia no existe."}`
	pokerDeletionBlockedMessage    = "La historia tiene historial asociado y no se puede eliminar."
	pokerEstimationSessionsBlocker = "estimation_sessions"
	pokerEstimationSessionsMessage = "Tiene sesiones de estimación por Planning Poker."
	pokerHistoryEmptyBody          = `{"openSession":null,"sessions":[],"emptyReason":"no_sessions"}`
)

// pokerSessionSummaryBody refleja EstimationSessionSummary del contrato.
type pokerSessionSummaryBody struct {
	ID                string        `json:"id"`
	Status            string        `json:"status"`
	OpenedBy          pokerUserRef  `json:"openedBy"`
	OpenedAt          string        `json:"openedAt"`
	ClosedBy          *pokerUserRef `json:"closedBy"`
	ClosedAt          string        `json:"closedAt"`
	AgreedStoryPoints *int          `json:"agreedStoryPoints"`
	CancelReason      *string       `json:"cancelReason"`
	RoundCount        int           `json:"roundCount"`
}

// pokerOpenSessionSummaryBody refleja OpenEstimationSession del contrato.
type pokerOpenSessionSummaryBody struct {
	ID          string       `json:"id"`
	Facilitator pokerUserRef `json:"facilitator"`
	RoundNumber int          `json:"roundNumber"`
	RoundStatus string       `json:"roundStatus"`
}

// pokerHistoryBody refleja StoryEstimationHistory del contrato. Raw guarda el JSON recibido, para
// comprobar que un campo vale [] o null y que no viajan campos de más.
type pokerHistoryBody struct {
	OpenSession *pokerOpenSessionSummaryBody `json:"openSession"`
	Sessions    []pokerSessionSummaryBody    `json:"sessions"`
	EmptyReason *string                      `json:"emptyReason"`
	Raw         []byte                       `json:"-"`
}

// history pide GET /api/v1/stories/{storyId}/estimation-sessions con el token como Bearer.
func (e *pokerTestEnv) history(t *testing.T, token string, storyID uuid.UUID) pokerResponse {
	t.Helper()
	return e.do(t, http.MethodGet, openSessionPath(storyID), token, nil)
}

// requireHistory consulta el historial de la historia, exige el 200 y lo devuelve decodificado.
func (e *pokerTestEnv) requireHistory(t *testing.T, token string, storyID uuid.UUID) pokerHistoryBody {
	t.Helper()
	var history pokerHistoryBody
	resp := e.history(t, token, storyID)
	requirePokerStatus(t, resp, http.StatusOK, &history)
	history.Raw = resp.Body
	return history
}

// finishSoloSession abre como Ana una sesión sola sobre la historia, vota la carta del valor indicado
// (la ronda se revela sola porque es la única conectada) y registra ese valor; devuelve la sesión
// finalizada.
func (f pokerUS1Fixture) finishSoloSession(t *testing.T, storyID uuid.UUID, storyPoints int) pokerSessionBody {
	t.Helper()
	session, _ := f.newPokerSoloSession(t, storyID)
	f.env.requireVoted(t, f.anaToken, session.ID, fmt.Sprint(storyPoints))
	return f.env.requireFinalized(t, f.anaToken, session.ID, storyPoints)
}

// cancelSoloSession abre como Ana una sesión sola sobre la historia y la cancela; devuelve la sesión
// cancelada.
func (f pokerUS1Fixture) cancelSoloSession(t *testing.T, storyID uuid.UUID) pokerSessionBody {
	t.Helper()
	session, _ := f.newPokerSoloSession(t, storyID)
	return f.env.requireCanceled(t, f.anaToken, session.ID)
}

// pokerParseTime interpreta un momento RFC 3339 del contrato.
func pokerParseTime(t *testing.T, value, where string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err, "%s: %q no es un momento RFC 3339", where, value)
	return parsed
}

// pokerJSONKeys devuelve las claves del objeto JSON recibido, para exigir que no viajen campos de
// más (por ejemplo, votos).
func pokerJSONKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &object), "el cuerpo no es un objeto JSON: %s", raw)
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

// requireUserRef exige que la referencia sea la de la persona indicada, con su nombre completo.
func requireUserRef(t *testing.T, got *pokerUserRef, id uuid.UUID, fullName, where string) {
	t.Helper()
	if assert.NotNil(t, got, "%s: no debería ser null", where) {
		assert.Equal(t, id.String(), got.ID, "%s: identificador", where)
		assert.Equal(t, fullName, got.FullName, "%s: nombre completo", where)
	}
}

// requireHistoryKeys exige que el historial traiga exactamente openSession, sessions y emptyReason, y
// que sessions sea una lista y no null.
func requireHistoryKeys(t *testing.T, history pokerHistoryBody) {
	t.Helper()
	assert.ElementsMatch(t, []string{"openSession", "sessions", "emptyReason"}, pokerJSONKeys(t, history.Raw),
		"campos del historial: %s", history.Raw)
	var raw struct {
		Sessions json.RawMessage `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(history.Raw, &raw))
	assert.NotEqual(t, "null", string(raw.Sessions), "sessions debería ser una lista: %s", history.Raw)
}

// requireOnlySession exige que el historial no tenga sesión abierta ni motivo de vacío y que liste
// exactamente una sesión cerrada, que devuelve.
func requireOnlySession(t *testing.T, history pokerHistoryBody) pokerSessionSummaryBody {
	t.Helper()
	requireHistoryKeys(t, history)
	assert.Nil(t, history.OpenSession, "no hay ninguna sesión abierta: %s", history.Raw)
	assert.Nil(t, history.EmptyReason, "el historial no está vacío: %s", history.Raw)
	require.Len(t, history.Sessions, 1, "sesiones del historial: %s", history.Raw)
	return history.Sessions[0]
}

// requireDeletionBlockedBySessions exige el 409 de la 003 con un único motivo, estimation_sessions.
func requireDeletionBlockedBySessions(t *testing.T, resp pokerResponse) {
	t.Helper()
	want, err := json.Marshal(map[string]any{
		"error":   pokerDeletionBlockedMessage,
		"reasons": []map[string]string{{"code": pokerEstimationSessionsBlocker, "message": pokerEstimationSessionsMessage}},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, string(want), string(resp.Body))
}

// Escenario: Sesión finalizada con rondas (005/US8)
// Con Ana, Bruno y Carla conectados, la sesión que Ana abrió a las 15:00 tiene tres rondas: dos
// divergentes de 3 (Ana) a 13 (Carla) y una tercera con consenso en 8. Diez minutos después Ana
// registra 8. Bruno consulta el historial de la historia: responde 200 sin sesión abierta y con una
// sola sesión, finalizada, con su apertura y su cierre, abierta y cerrada por Ana, el valor 8, sin
// motivo de cancelación y con tres rondas. La consulta de esa sesión muestra las tres rondas con el
// voto y el nombre de cada participante y el consenso o la divergencia de cada una (FR-079, FR-080).
func TestUS8_SesionFinalizadaConRondas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	openedAt := f.env.clock.Now()
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	for _, token := range []string{f.anaToken, f.brunoToken, f.carlaToken} {
		f.env.requireVoted(t, token, f.session.ID, "8")
	}
	f.env.clock.Advance(10 * time.Minute)
	f.env.requireFinalized(t, f.anaToken, f.session.ID, 8)
	closedAt := f.env.clock.Now()

	// El historial resume la sesión finalizada con sus tres rondas.
	summary := requireOnlySession(t, f.env.requireHistory(t, f.brunoToken, f.storyID))
	assert.Equal(t, f.session.ID, summary.ID, "identificador de la sesión")
	assert.Equal(t, "finished", summary.Status, "estado de la sesión")
	requireUserRef(t, &summary.OpenedBy, f.anaID, "Ana Pérez", "openedBy")
	assert.True(t, pokerParseTime(t, summary.OpenedAt, "openedAt").Equal(openedAt), "openedAt = %s, se esperaba %s", summary.OpenedAt, openedAt)
	requireUserRef(t, summary.ClosedBy, f.anaID, "Ana Pérez", "closedBy")
	assert.True(t, pokerParseTime(t, summary.ClosedAt, "closedAt").Equal(closedAt), "closedAt = %s, se esperaba %s", summary.ClosedAt, closedAt)
	assert.Equal(t, pokerIntPtr(8), summary.AgreedStoryPoints, "valor acordado")
	assert.Nil(t, summary.CancelReason, "una sesión finalizada no tiene motivo de cancelación")
	assert.Equal(t, 3, summary.RoundCount, "cantidad de rondas")

	// La consulta de la sesión muestra cada ronda con sus votos y su resultado.
	session := f.env.requireSessionFetched(t, f.brunoToken, summary.ID)
	rounds := pokerAllRounds(t, session)
	require.Len(t, rounds, 3, "rondas: %s", session.Raw)
	names := map[string]string{f.anaID.String(): "Ana Pérez", f.brunoID.String(): "Bruno Gómez", f.carlaID.String(): "Carla Díaz"}
	for i, round := range rounds {
		where := fmt.Sprintf("ronda %d", i+1)
		assert.Equal(t, "revealed", round.Status, "%s: estado", where)
		for _, vote := range round.Votes {
			assert.Equal(t, names[vote.User.ID], vote.User.FullName, "%s: nombre de quien votó", where)
		}
		if i < 2 {
			assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "%s: votos", where)
			requireDivergence(t, round, 3, []uuid.UUID{f.anaID}, 13, []uuid.UUID{f.carlaID}, where)
			continue
		}
		assert.Equal(t, map[string]string{f.anaID.String(): "8", f.brunoID.String(): "8", f.carlaID.String(): "8"},
			pokerRevealedCards(round), "%s: votos", where)
		requireConsensus(t, round, 8, where)
	}
}

// Escenario: Orden de sesiones (005/US8)
// Ana estima la historia sola con una sesión que registra 5 y, una hora después, con otra que registra
// 8. Bruno consulta el historial: responde 200 con las dos sesiones finalizadas de la más reciente a
// la más antigua por su momento de apertura, la de 8 primero y la de 5 después, cada una con su valor
// acordado (FR-080).
func TestUS8_OrdenDeSesiones(t *testing.T) {
	f := newPokerUS1Fixture(t)
	older := f.finishSoloSession(t, f.storyID, 5)
	f.env.clock.Advance(time.Hour)
	newer := f.finishSoloSession(t, f.storyID, 8)

	history := f.env.requireHistory(t, f.brunoToken, f.storyID)

	requireHistoryKeys(t, history)
	assert.Nil(t, history.OpenSession, "no hay ninguna sesión abierta: %s", history.Raw)
	assert.Nil(t, history.EmptyReason, "el historial no está vacío: %s", history.Raw)
	require.Len(t, history.Sessions, 2, "sesiones del historial: %s", history.Raw)
	first, second := history.Sessions[0], history.Sessions[1]
	assert.Equal(t, newer.ID, first.ID, "la sesión más reciente va primero")
	assert.Equal(t, pokerIntPtr(8), first.AgreedStoryPoints, "valor de la sesión más reciente")
	assert.Equal(t, older.ID, second.ID, "la sesión más antigua va después")
	assert.Equal(t, pokerIntPtr(5), second.AgreedStoryPoints, "valor de la sesión más antigua")
	assert.True(t, pokerParseTime(t, first.OpenedAt, "openedAt de la primera").
		After(pokerParseTime(t, second.OpenedAt, "openedAt de la segunda")), "el orden es por apertura descendente")
	for _, summary := range history.Sessions {
		assert.Equal(t, "finished", summary.Status, "estado de la sesión %s", summary.ID)
	}
}

// Escenario: Sesión cancelada (005/US8)
// Con dos rondas reveladas con los votos 3, 5 y 13 de Ana, Bruno y Carla, Ana cancela la sesión.
// Carla consulta el historial: responde 200 con la sesión cancelada por Ana, con agreedStoryPoints y
// cancelReason en null y dos rondas; la consulta de la sesión muestra las dos rondas completas con sus
// votos (FR-058, FR-079).
func TestUS8_SesionCancelada(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.revealDivergentRound(t)
	f.env.requireCanceled(t, f.anaToken, f.session.ID)

	history := f.env.requireHistory(t, f.carlaToken, f.storyID)

	summary := requireOnlySession(t, history)
	assert.Equal(t, f.session.ID, summary.ID, "identificador de la sesión")
	assert.Equal(t, "canceled", summary.Status, "estado de la sesión")
	assert.Nil(t, summary.AgreedStoryPoints, "una sesión cancelada no tiene valor acordado")
	assert.Nil(t, summary.CancelReason, "la cancelación del facilitador no lleva motivo del sistema")
	requireUserRef(t, summary.ClosedBy, f.anaID, "Ana Pérez", "closedBy")
	assert.Equal(t, 2, summary.RoundCount, "cantidad de rondas")
	var raw struct {
		Sessions []map[string]json.RawMessage `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(history.Raw, &raw))
	if assert.Len(t, raw.Sessions, 1) {
		assert.Equal(t, "null", string(raw.Sessions[0]["agreedStoryPoints"]), "agreedStoryPoints debería viajar como null")
	}

	// Las rondas de la sesión cancelada se consultan completas.
	session := f.env.requireSessionFetched(t, f.carlaToken, summary.ID)
	rounds := pokerAllRounds(t, session)
	require.Len(t, rounds, 2, "rondas: %s", session.Raw)
	for i, round := range rounds {
		where := fmt.Sprintf("ronda %d", i+1)
		assert.Equal(t, "revealed", round.Status, "%s: estado", where)
		assert.Equal(t, f.divergentCards(), pokerRevealedCards(round), "%s: votos", where)
	}
}

// Escenario: Sesión en curso (005/US8)
// Con la ronda 1 revelada, la ronda 2 en votación y el voto 8 de Ana todavía oculto, Bruno consulta
// el historial: responde 200 con openSession, que lleva solo el identificador de la sesión, a Ana como
// facilitadora, el número de ronda 2 y el estado voting, sin votos; sessions es la lista vacía, porque
// no hay sesiones cerradas, y emptyReason es null, porque la historia sí tiene una sesión (FR-081,
// FR-082, FR-083).
func TestUS8_SesionEnCurso(t *testing.T) {
	f := newPokerUS2Fixture(t)
	f.revealDivergentRound(t)
	f.env.requireStartedRound(t, f.anaToken, f.session.ID)
	f.env.requireVoted(t, f.anaToken, f.session.ID, "8")

	history := f.env.requireHistory(t, f.brunoToken, f.storyID)

	requireHistoryKeys(t, history)
	assert.Empty(t, history.Sessions, "no hay sesiones cerradas: %s", history.Raw)
	assert.Nil(t, history.EmptyReason, "la historia tiene una sesión abierta: %s", history.Raw)
	require.NotNil(t, history.OpenSession, "debería informar la sesión en curso: %s", history.Raw)
	assert.Equal(t, f.session.ID, history.OpenSession.ID, "identificador de la sesión en curso")
	requireUserRef(t, &history.OpenSession.Facilitator, f.anaID, "Ana Pérez", "facilitador")
	assert.Equal(t, 2, history.OpenSession.RoundNumber, "número de la ronda vigente")
	assert.Equal(t, "voting", history.OpenSession.RoundStatus, "estado de la ronda vigente")

	// La sesión en curso no expone ningún voto.
	var raw struct {
		OpenSession json.RawMessage `json:"openSession"`
	}
	require.NoError(t, json.Unmarshal(history.Raw, &raw))
	assert.ElementsMatch(t, []string{"id", "facilitator", "roundNumber", "roundStatus"}, pokerJSONKeys(t, raw.OpenSession),
		"campos de openSession: %s", raw.OpenSession)
}

// Escenario: Historia sin sesiones (005/US8)
// Bruno consulta el historial de una historia que nunca se estimó por Planning Poker: responde 200,
// no un error, con openSession null, sessions como lista vacía y emptyReason no_sessions (FR-083).
func TestUS8_HistoriaSinSesiones(t *testing.T) {
	f := newPokerUS1Fixture(t)

	resp := f.env.history(t, f.brunoToken, f.storyID)

	assert.Equal(t, http.StatusOK, resp.Code, "cuerpo: %s", resp.Body)
	assert.JSONEq(t, pokerHistoryEmptyBody, string(resp.Body))
}

// Escenario: Ex integrante (005/US8)
// Carla abre una sesión con Ana y Bruno conectados; votan 3, 5 y 13 y Carla registra 5. Después Ana,
// propietaria, la da de baja del proyecto con el DELETE de miembros de la 002. Bruno consulta el
// historial: la sesión sigue abierta y cerrada por "Carla Díaz"; y la consulta de la sesión muestra
// el voto 13 de Carla con su nombre (FR-084).
func TestUS8_ExIntegrante(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.carlaToken, f.storyID)
	carla := f.env.openStream(t, f.carlaToken, session.ID)
	ana := f.env.openStream(t, f.anaToken, session.ID)
	bruno := f.env.openStream(t, f.brunoToken, session.ID)
	three := allConnected(f.anaID, f.brunoID, f.carlaID)
	for name, stream := range map[string]*pokerStream{"Carla": carla, "Ana": ana, "Bruno": bruno} {
		stream.NextUntil(t, "los tres conectados en el canal de "+name, three)
	}
	f.env.requireVoted(t, f.anaToken, session.ID, "3")
	f.env.requireVoted(t, f.brunoToken, session.ID, "5")
	f.env.requireVoted(t, f.carlaToken, session.ID, "13")
	carla.NextUntil(t, "la ronda revelada en el canal de Carla", currentRoundRevealed)
	f.env.requireFinalized(t, f.carlaToken, session.ID, 5)

	// Ana da de baja a Carla del proyecto.
	requirePokerStatus(t, f.env.do(t, http.MethodDelete,
		"/api/v1/projects/"+f.projectID.String()+"/members/"+f.carlaID.String(), f.anaToken, nil), http.StatusNoContent, nil)

	// El historial sigue nombrando a Carla como quien abrió y cerró la sesión.
	summary := requireOnlySession(t, f.env.requireHistory(t, f.brunoToken, f.storyID))
	requireUserRef(t, &summary.OpenedBy, f.carlaID, "Carla Díaz", "openedBy")
	requireUserRef(t, summary.ClosedBy, f.carlaID, "Carla Díaz", "closedBy")

	// Su voto sigue figurando con su nombre.
	fetched := f.env.requireSessionFetched(t, f.brunoToken, summary.ID)
	rounds := pokerAllRounds(t, fetched)
	require.Len(t, rounds, 1, "rondas: %s", fetched.Raw)
	var found bool
	for _, vote := range rounds[0].Votes {
		if vote.User.ID == f.carlaID.String() {
			found = true
			assert.Equal(t, "Carla Díaz", vote.User.FullName, "nombre de la ex integrante")
			assert.Equal(t, "13", vote.Card, "carta de la ex integrante")
		}
	}
	assert.True(t, found, "el voto de Carla debería seguir en la ronda: %s", fetched.Raw)
}

// Escenario: Proyecto finalizado (005/US8)
// Ana estima la historia sola con una sesión que registra 5 y después finaliza el proyecto con la
// 002. Bruno consulta el historial: responde 200 con la sesión finalizada, porque un proyecto
// Finalizado admite lectura (FR-085).
func TestUS8_ProyectoFinalizado(t *testing.T) {
	f := newPokerUS1Fixture(t)
	finished := f.finishSoloSession(t, f.storyID, 5)
	f.env.setProjectStatus(t, f.anaToken, f.projectID, "finished")

	summary := requireOnlySession(t, f.env.requireHistory(t, f.brunoToken, f.storyID))

	assert.Equal(t, finished.ID, summary.ID, "identificador de la sesión")
	assert.Equal(t, "finished", summary.Status, "estado de la sesión")
	assert.Equal(t, pokerIntPtr(5), summary.AgreedStoryPoints, "valor acordado")
}

// Escenario: No integrante (005/US8)
// La historia tiene una sesión finalizada. Diego, que no integra el proyecto, consulta su historial:
// responde 404 con el mismo código, tipo de contenido y cuerpo byte a byte que el historial de un
// storyId inexistente, el de historia inexistente de la 003, sin revelar ningún dato (FR-087,
// FR-088, SC-018).
func TestUS8_NoIntegrante(t *testing.T) {
	f := newPokerUS1Fixture(t)
	f.finishSoloSession(t, f.storyID, 5)

	missing := f.env.history(t, f.diegoToken, uuid.New())
	foreign := f.env.history(t, f.diegoToken, f.storyID)

	assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body)
	assert.Equal(t, pokerStoryNotFoundBody, string(missing.Body), "inexistente: el cuerpo debería ser el de la 003")
	assert.Equal(t, http.StatusNotFound, foreign.Code, "ajena: cuerpo: %s", foreign.Body)
	assert.Equal(t, missing.ContentType, foreign.ContentType, "los tipos de contenido deberían ser idénticos")
	assert.Equal(t, missing.Body, foreign.Body, "los cuerpos deberían ser idénticos byte a byte")
}

// Escenario: Eliminación bloqueada (005/US8)
// Ana, propietaria, intenta eliminar con el DELETE /api/v1/stories/{storyId} de la 003 una historia
// con historial de estimación, en tres variantes: con una sesión finalizada y otra cancelada, como
// dice la spec, con solo una finalizada y con solo una cancelada. Responde 409 con el único motivo
// estimation_sessions; la historia sigue existiendo y su historial sigue consultable con las mismas
// sesiones (RN-33, SC-023).
func TestUS8_EliminacionBloqueada(t *testing.T) {
	f := newPokerUS1Fixture(t)

	// Cada variante deja la historia con sus sesiones cerradas y devuelve sus identificadores.
	variants := []struct {
		name  string
		close func(t *testing.T, storyID uuid.UUID) []string
	}{
		{
			name: "una sesión finalizada y otra cancelada",
			close: func(t *testing.T, storyID uuid.UUID) []string {
				finished := f.finishSoloSession(t, storyID, 5)
				f.env.clock.Advance(time.Hour)
				canceled := f.cancelSoloSession(t, storyID)
				return []string{canceled.ID, finished.ID}
			},
		},
		{
			name: "solo una sesión finalizada",
			close: func(t *testing.T, storyID uuid.UUID) []string {
				return []string{f.finishSoloSession(t, storyID, 5).ID}
			},
		},
		{
			name: "solo una sesión cancelada",
			close: func(t *testing.T, storyID uuid.UUID) []string {
				return []string{f.cancelSoloSession(t, storyID).ID}
			},
		},
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			storyID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{Title: "Historia de " + tc.name})
			sessionIDs := tc.close(t, storyID)
			f.env.clock.Advance(time.Minute)

			resp := f.env.do(t, http.MethodDelete, "/api/v1/stories/"+storyID.String(), f.anaToken, nil)

			requireDeletionBlockedBySessions(t, resp)
			requirePokerStatus(t, f.env.do(t, http.MethodGet, "/api/v1/stories/"+storyID.String(), f.anaToken, nil),
				http.StatusOK, nil)
			assert.Equal(t, int64(len(sessionIDs)), pokerSessionRows(t, f.env.db, storyID), "las sesiones se conservan")

			// El historial sigue consultable con las mismas sesiones, de la más reciente a la más antigua.
			history := f.env.requireHistory(t, f.brunoToken, storyID)
			got := make([]string, 0, len(history.Sessions))
			for _, summary := range history.Sessions {
				got = append(got, summary.ID)
			}
			assert.Equal(t, sessionIDs, got, "sesiones del historial: %s", history.Raw)
		})
	}
}

// Pruebas de integración de caracterización de la fase Polish (T091): recorren los diez endpoints
// de Planning Poker sobre el mismo router compuesto como cmd/app y contra smye_test, y verifican de
// forma transversal las puertas de seguridad que ya cumplen las historias: sin sesión válida se
// responde 401 sin ningún cambio (SC-017); quien no integra el proyecto recibe un 404 idéntico byte
// a byte al de un identificador inexistente (SC-018); una ronda en votación no expone la carta de su
// autor por ninguna forma de consulta (SC-005), y ninguna respuesta incluye el hash de la contraseña
// (Principio VI). No llevan la etiqueta de historia porque no derivan de un escenario de spec.md.

// pokerSessionRequiredBody es el cuerpo exacto del 401 de la 001 cuando falta la sesión o el token
// no es válido.
const pokerSessionRequiredBody = `{"error":"Se requiere iniciar sesión."}`

// pokerEndpoint es uno de los diez endpoints de la feature: su nombre para el subtest, el método, la
// ruta ya armada y el cuerpo JSON opcional que llevaría una petición válida.
type pokerEndpoint struct {
	name   string
	method string
	path   string
	body   any
}

// pokerStoryEndpoints arma las dos rutas de historia de la feature, la apertura y el historial,
// sobre la historia indicada.
func pokerStoryEndpoints(storyID uuid.UUID) []pokerEndpoint {
	return []pokerEndpoint{
		{name: "abrir sesión", method: http.MethodPost, path: openSessionPath(storyID)},
		{name: "historial de la historia", method: http.MethodGet, path: openSessionPath(storyID)},
	}
}

// pokerSessionEndpoints arma las ocho rutas de sesión de la feature sobre el identificador indicado,
// con un cuerpo válido en el voto y en el registro de la estimación.
func pokerSessionEndpoints(sessionID string) []pokerEndpoint {
	return []pokerEndpoint{
		{name: "consultar sesión", method: http.MethodGet, path: sessionPath(sessionID)},
		{name: "canal de eventos", method: http.MethodGet, path: eventsPath(sessionID)},
		{name: "iniciar ronda", method: http.MethodPost, path: roundsPath(sessionID)},
		{name: "votar", method: http.MethodPut, path: votePath(sessionID), body: map[string]any{"card": "5"}},
		{name: "revelar", method: http.MethodPost, path: revealPath(sessionID)},
		{name: "registrar estimación", method: http.MethodPost, path: finalizePath(sessionID), body: map[string]any{"storyPoints": 5}},
		{name: "cancelar", method: http.MethodPost, path: cancelPath(sessionID)},
		{name: "tomar el rol de facilitador", method: http.MethodPost, path: facilitatorPath(sessionID)},
	}
}

// pokerTamperedToken altera un carácter en el medio de la firma del JWT, de modo que los bytes
// firmados cambian con seguridad (el último carácter podría variar solo bits de relleno) y el token
// deja de ser válido aunque conserve su forma.
func pokerTamperedToken(t *testing.T, token string) string {
	t.Helper()
	require.Greater(t, len(token), 20, "el token de prueba es demasiado corto: %q", token)
	tampered := []byte(token)
	i := len(tampered) - 10
	if tampered[i] == 'A' {
		tampered[i] = 'B'
	} else {
		tampered[i] = 'A'
	}
	return string(tampered)
}

// pokerStateFingerprint serializa, ordenadas por identificador, todas las filas de las cuatro tablas
// de la feature y los Story Points con la fecha de actualización de cada historia, para comparar el
// estado de la base antes y después de una petición rechazada.
func pokerStateFingerprint(t *testing.T, db *gorm.DB) string {
	t.Helper()
	snapshot := map[string][]map[string]any{}
	models := map[string]any{
		"estimation_sessions":     &domain.EstimationSession{},
		"estimation_participants": &domain.EstimationParticipant{},
		"estimation_rounds":       &domain.EstimationRound{},
		"estimation_votes":        &domain.EstimationVote{},
	}
	for name, model := range models {
		var rows []map[string]any
		require.NoError(t, db.Model(model).Order("id").Find(&rows).Error, "no se pudo leer %s", name)
		snapshot[name] = rows
	}
	var stories []map[string]any
	require.NoError(t, db.Table("user_stories").Select("id, story_points, updated_at").Order("id").Find(&stories).Error,
		"no se pudieron leer las historias")
	snapshot["user_stories"] = stories
	data, err := json.Marshal(snapshot)
	require.NoError(t, err, "no se pudo serializar el estado de la base")
	return string(data)
}

// requireNoPasswordHash exige que el cuerpo no incluya el hash de la contraseña bajo ninguno de sus
// dos nombres posibles (Principio VI).
func requireNoPasswordHash(t *testing.T, body []byte, where string) {
	t.Helper()
	assert.False(t, bytes.Contains(body, []byte("password_hash")), "%s: la respuesta incluye password_hash: %s", where, body)
	assert.False(t, bytes.Contains(body, []byte("passwordHash")), "%s: la respuesta incluye passwordHash: %s", where, body)
}

// TestPokerEndpoints_SinSesion verifica que cada uno de los diez endpoints, sin cabecera de
// autorización y con un token adulterado de Ana, propietaria y facilitadora, responde el 401 de la
// 001 y no cambia nada en la base: ni sesiones, ni participantes, ni rondas, ni votos, ni los Story
// Points de las historias (SC-017, Principio VI). La apertura apunta a una segunda historia sin
// sesión, donde una petición válida sí crearía una.
func TestPokerEndpoints_SinSesion(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	freeStoryID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{Title: "Historia sin sesión"})
	endpoints := append(pokerStoryEndpoints(freeStoryID), pokerSessionEndpoints(session.ID)...)
	require.Len(t, endpoints, 10, "la feature expone diez endpoints")

	// Cada endpoint se prueba sin cabecera y con el token adulterado, comparando la base antes y
	// después de la petición.
	credentials := []struct {
		name  string
		token string
	}{
		{name: "sin cabecera", token: ""},
		{name: "token adulterado", token: pokerTamperedToken(t, f.anaToken)},
	}
	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			for _, cred := range credentials {
				before := pokerStateFingerprint(t, f.env.db)
				resp := f.env.do(t, ep.method, ep.path, cred.token, ep.body)

				where := ep.method + " " + ep.path + " " + cred.name
				assert.Equal(t, http.StatusUnauthorized, resp.Code, "%s: cuerpo: %s", where, resp.Body)
				assert.Equal(t, pokerSessionRequiredBody, string(resp.Body), "%s: el cuerpo debería ser el de la 001", where)
				requireNoPasswordHash(t, resp.Body, where)
				assert.Equal(t, before, pokerStateFingerprint(t, f.env.db), "%s: la base no debería cambiar", where)
			}
		})
	}
	assert.Zero(t, pokerSessionRows(t, f.env.db, freeStoryID), "la historia libre no debería tener sesiones")
}

// TestPokerEndpoints_NotFoundIndistinguible verifica que Diego, que no integra el proyecto, recibe en
// las ocho rutas de sesión el mismo código, tipo de contenido y cuerpo byte a byte que con un
// sessionId inexistente ("La sesión de estimación no existe."), y en las dos rutas de historia lo
// mismo que con un storyId inexistente ("La historia no existe." de la 003), sin que su petición
// cambie nada en la base (SC-018, Principio VI).
func TestPokerEndpoints_NotFoundIndistinguible(t *testing.T) {
	f := newPokerUS1Fixture(t)
	session := f.env.requireOpenedSession(t, f.anaToken, f.storyID)
	freeStoryID := f.env.createStory(t, f.brunoToken, f.projectID, pokerStoryInput{Title: "Historia sin sesión"})

	// Cada grupo compara la ruta sobre el recurso ajeno con la misma ruta sobre un identificador
	// inexistente, y fija el cuerpo que manda el contrato.
	groups := []struct {
		name     string
		foreign  []pokerEndpoint
		missing  []pokerEndpoint
		wantBody string
	}{
		{
			name:     "sesión",
			foreign:  pokerSessionEndpoints(session.ID),
			missing:  pokerSessionEndpoints(uuid.New().String()),
			wantBody: pokerSessionNotFoundBody,
		},
		{
			name:     "historia",
			foreign:  pokerStoryEndpoints(freeStoryID),
			missing:  pokerStoryEndpoints(uuid.New()),
			wantBody: pokerStoryNotFoundBody,
		},
	}
	total := 0
	for _, group := range groups {
		require.Len(t, group.missing, len(group.foreign))
		total += len(group.foreign)
		for i, ep := range group.foreign {
			t.Run(group.name+" "+ep.name, func(t *testing.T) {
				before := pokerStateFingerprint(t, f.env.db)
				missing := f.env.do(t, group.missing[i].method, group.missing[i].path, f.diegoToken, group.missing[i].body)
				foreign := f.env.do(t, ep.method, ep.path, f.diegoToken, ep.body)

				assert.Equal(t, http.StatusNotFound, missing.Code, "inexistente: cuerpo: %s", missing.Body)
				assert.Equal(t, group.wantBody, string(missing.Body), "inexistente: el cuerpo debería ser el del contrato")
				assert.Equal(t, missing.Code, foreign.Code, "los códigos deberían ser idénticos; ajena: %s", foreign.Body)
				assert.Equal(t, missing.ContentType, foreign.ContentType, "los tipos de contenido deberían ser idénticos")
				assert.Equal(t, missing.Body, foreign.Body, "los cuerpos deberían ser idénticos byte a byte")
				requireNoPasswordHash(t, missing.Body, "inexistente")
				requireNoPasswordHash(t, foreign.Body, "ajena")
				assert.Equal(t, before, pokerStateFingerprint(t, f.env.db), "la petición de Diego no debería cambiar la base")
			})
		}
	}
	assert.Equal(t, 10, total, "se comparan los diez endpoints")
}

// requireNoHiddenCard exige que un cuerpo recibido mientras la única ronda de la sesión está en
// votación no exponga ningún voto: recorre el JSON completo y falla si algún objeto trae el campo
// card de un voto o si algún myVote vale la carta oculta. Un cuerpo vacío o que no es JSON se
// considera sin votos.
func requireNoHiddenCard(t *testing.T, body []byte, card, where string) {
	t.Helper()
	var doc any
	if len(bytes.TrimSpace(body)) == 0 || json.Unmarshal(body, &doc) != nil {
		return
	}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "card" {
					t.Errorf("%s: %s.card expone un voto de la ronda en votación: %s", where, path, body)
				}
				if key == "myVote" && child == card {
					t.Errorf("%s: %s.myVote expone la carta %q de otra persona: %s", where, path, card, body)
				}
				walk(child, path+"."+key)
			}
		case []any:
			for i, child := range value {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	walk(doc, "$")
}

// TestPokerSecreto_TodasLasConsultas verifica que, con la ronda 1 en votación y la carta 40 de
// Bruno ya jugada, ninguna forma de consulta de la sesión desde las cuentas de Ana y de Carla expone
// esa carta (SC-005, FR-023, FR-082): los canales ya abiertos y uno nuevo, GET
// /estimation-sessions/{id}, la respuesta de cada acción que deja la ronda en votación (el voto de
// Ana, que no completa la ronda porque Carla no votó, y los rechazos de iniciar ronda, revelar,
// registrar, cancelar, tomar el rol y abrir otra sesión) y el historial de la historia. Ninguna de
// esas respuestas incluye el hash de la contraseña (Principio VI). La revelación de Ana y el voto de
// Carla no se piden porque revelarían la ronda.
func TestPokerSecreto_TodasLasConsultas(t *testing.T) {
	f := newPokerUS2Fixture(t)
	const secret = "40"
	own := f.env.requireVoted(t, f.brunoToken, f.session.ID, secret)
	requireOwnVote(t, own, f.brunoID, secret, "respuesta de Bruno")
	requireNoPasswordHash(t, own.Raw, "respuesta de Bruno")

	// Cada consulta se anota con su nombre y su cuerpo para revisarlas todas al final.
	type observed struct {
		where string
		body  []byte
	}
	var seen []observed
	add := func(where string, body []byte) { seen = append(seen, observed{where: where, body: body}) }

	// Canal: los de Ana y Carla ya abiertos y una conexión nueva de Carla.
	add("canal abierto de Ana", f.ana.NextUntil(t, "Bruno ya votó en el canal de Ana", hasVoted(f.brunoID)).Raw)
	add("canal abierto de Carla", f.carla.NextUntil(t, "Bruno ya votó en el canal de Carla", hasVoted(f.brunoID)).Raw)
	fresh := f.env.openStream(t, f.carlaToken, f.session.ID)
	add("canal nuevo de Carla", fresh.NextUntil(t, "Bruno ya votó en el canal nuevo de Carla", hasVoted(f.brunoID)).Raw)

	// Consulta de la sesión con todas sus rondas.
	add("GET de la sesión por Ana", f.env.requireSessionFetched(t, f.anaToken, f.session.ID).Raw)
	add("GET de la sesión por Carla", f.env.requireSessionFetched(t, f.carlaToken, f.session.ID).Raw)

	// Respuesta de cada acción que deja la ronda en votación, y la instantánea que provoca el voto de
	// Ana en el canal de Carla.
	anaVote := f.env.requireVoted(t, f.anaToken, f.session.ID, "1")
	add("respuesta del voto de Ana", anaVote.Raw)
	add("canal de Carla tras el voto de Ana", f.carla.NextUntil(t, "Ana ya votó en el canal de Carla", hasVoted(f.anaID)).Raw)
	actions := []struct {
		where string
		token string
		ep    pokerEndpoint
	}{
		{where: "iniciar ronda por Ana", token: f.anaToken, ep: pokerEndpoint{method: http.MethodPost, path: roundsPath(f.session.ID)}},
		{where: "registrar por Ana", token: f.anaToken, ep: pokerEndpoint{method: http.MethodPost, path: finalizePath(f.session.ID), body: map[string]any{"storyPoints": 40}}},
		{where: "tomar el rol por Ana", token: f.anaToken, ep: pokerEndpoint{method: http.MethodPost, path: facilitatorPath(f.session.ID)}},
		{where: "revelar por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: revealPath(f.session.ID)}},
		{where: "iniciar ronda por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: roundsPath(f.session.ID)}},
		{where: "registrar por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: finalizePath(f.session.ID), body: map[string]any{"storyPoints": 40}}},
		{where: "cancelar por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: cancelPath(f.session.ID)}},
		{where: "tomar el rol por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: facilitatorPath(f.session.ID)}},
		{where: "abrir otra sesión por Carla", token: f.carlaToken, ep: pokerEndpoint{method: http.MethodPost, path: openSessionPath(f.storyID)}},
	}
	for _, action := range actions {
		resp := f.env.do(t, action.ep.method, action.ep.path, action.token, action.ep.body)
		assert.Less(t, resp.Code, http.StatusInternalServerError, "%s: no debería fallar internamente: %s", action.where, resp.Body)
		add(action.where, resp.Body)
	}

	// Historial de la historia.
	add("historial por Ana", f.env.requireHistory(t, f.anaToken, f.storyID).Raw)
	add("historial por Carla", f.env.requireHistory(t, f.carlaToken, f.storyID).Raw)

	// Ninguna consulta expone la carta de Bruno ni el hash de una contraseña, y la ronda sigue en
	// votación, así que el secreto seguía vigente durante todas.
	for _, o := range seen {
		requireNoHiddenCard(t, o.body, secret, o.where)
		requireNoPasswordHash(t, o.body, o.where)
	}
	requireStoredVoting(t, f.env.db, f.session.ID, "ninguna consulta debería revelar la ronda")
	requireSingleStoredVote(t, f.env.db, f.session.ID, f.brunoID, secret)
}

// Prueba de integración del registro de la estimación concurrente con el cierre de la historia o del
// proyecto (T102; hallazgo de seguridad detectado en 005/US7: registro de la estimación sin releer el
// estado de la historia bajo bloqueo). Una transacción de prueba bloquea la historia con FOR UPDATE y
// la completa, o finaliza el proyecto, mientras el registro espera ese bloqueo; al confirmarla, el
// registro no debe escribir story_points y la sesión debe quedar cancelada por el sistema con el motivo
// que corresponde (FR-061, data-model.md sección 9). No lleva la etiqueta de un escenario de la spec
// porque verifica un hallazgo de seguridad y no un escenario Given/When/Then.

// Plazos de la espera del registro bloqueado en la historia y de su respuesta tras confirmar.
const (
	pokerLockWaitTimeout  = 5 * time.Second
	pokerLockWaitInterval = 10 * time.Millisecond
)

// pokerWaitForStoryLock consulta pg_stat_activity hasta ver, en la base de test, una sentencia sobre
// user_stories de otra conexión esperando un bloqueo de fila (wait_event_type = 'Lock'). Devuelve
// falso si no aparece dentro del plazo.
func pokerWaitForStoryLock(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	deadline := time.Now().Add(pokerLockWaitTimeout)
	for time.Now().Before(deadline) {
		var waiting int64
		err := db.Raw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND pid <> pg_backend_pid() AND query ILIKE '%user_stories%'`).Scan(&waiting).Error
		require.NoError(t, err, "no se pudo consultar pg_stat_activity")
		if waiting > 0 {
			return true
		}
		time.Sleep(pokerLockWaitInterval)
	}
	return false
}

// pokerStoredStoryPoints lee user_stories.story_points de la historia; nil es "sin estimar".
func pokerStoredStoryPoints(t *testing.T, db *gorm.DB, storyID uuid.UUID) *int {
	t.Helper()
	var points []*int
	require.NoError(t, db.Table("user_stories").Where("id = ?", storyID).Pluck("story_points", &points).Error,
		"no se pudieron leer los Story Points de la historia")
	require.Len(t, points, 1, "la historia %s debería existir", storyID)
	return points[0]
}

// Ana, sola en una sesión sobre una historia de 5 Story Points, vota 8 y la ronda se revela. Una
// transacción de prueba bloquea la historia con FOR UPDATE y, según el caso, la pasa a completed o
// pasa el proyecto a finished. Ana registra 8 desde una goroutine; con el registro ya esperando el
// bloqueo de la historia, la transacción se confirma. El registro responde 409 de sesión cerrada con
// el motivo del sistema, la historia conserva sus 5 Story Points y la sesión queda cancelada sin quien
// la cerró y con el motivo story_completed o project_finished.
func TestPokerFinalize_Concurrency(t *testing.T) {
	// Cada caso declara el cambio que confirma la transacción de prueba, el motivo esperado y el
	// mensaje del 409 que corresponde a ese motivo.
	cases := []struct {
		name    string
		change  func(tx *gorm.DB, f pokerUS1Fixture, storyID uuid.UUID) error
		reason  domain.CancelReason
		message string
	}{
		{
			name: "historia completada",
			change: func(tx *gorm.DB, _ pokerUS1Fixture, storyID uuid.UUID) error {
				return tx.Exec("UPDATE user_stories SET status = 'completed' WHERE id = ?", storyID).Error
			},
			reason:  domain.CancelReasonStoryCompleted,
			message: pokerSessionClosedMessage,
		},
		{
			name: "proyecto finalizado",
			change: func(tx *gorm.DB, f pokerUS1Fixture, _ uuid.UUID) error {
				return tx.Exec("UPDATE projects SET status = 'finished' WHERE id = ?", f.projectID).Error
			},
			reason:  domain.CancelReasonProjectFinished,
			message: pokerProjectFinishedMessage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Sesión de Ana sola sobre una historia de 5 Story Points, con la ronda revelada en 8.
			f := newPokerUS1Fixture(t)
			storyID := f.env.createStory(t, f.anaToken, f.projectID, pokerStoryInput{
				Title:  "Exportar el reporte",
				Points: pokerIntPtr(5),
			})
			session, _ := f.newPokerSoloSession(t, storyID)
			f.env.requireVoted(t, f.anaToken, session.ID, "8")

			// La transacción de prueba bloquea la historia y deja sin confirmar el cambio del caso; si
			// la prueba termina antes de confirmarla, se revierte y libera al registro.
			tx := f.env.db.Begin()
			require.NoError(t, tx.Error, "no se pudo abrir la transacción de prueba")
			defer tx.Rollback()
			require.NoError(t, tx.Exec("SELECT id FROM user_stories WHERE id = ? FOR UPDATE", storyID).Error,
				"no se pudo bloquear la historia")
			require.NoError(t, tc.change(tx, f, storyID), "no se pudo aplicar el cambio del caso")

			// El registro corre en una goroutine y queda esperando el bloqueo de la historia.
			payload, err := json.Marshal(map[string]any{"storyPoints": 8})
			require.NoError(t, err)
			done := make(chan pokerConcurrentResult, 1)
			go func() {
				done <- f.env.pokerSend(http.MethodPost, finalizePath(session.ID), f.anaToken, payload)
			}()
			require.True(t, pokerWaitForStoryLock(t, f.env.db),
				"el registro debería quedar esperando el bloqueo de la historia")
			require.NoError(t, tx.Commit().Error, "no se pudo confirmar la transacción de prueba")

			// Confirmado el cambio, el registro responde 409 de sesión cerrada con el motivo del sistema.
			var res pokerConcurrentResult
			select {
			case res = <-done:
			case <-time.After(pokerHTTPTimeout):
				t.Fatalf("el registro no respondió en %s tras confirmar la transacción", pokerHTTPTimeout)
			}
			require.NoError(t, res.err, "el registro no obtuvo respuesta")
			requireSessionClosedBody(t, res.resp, tc.message, "canceled", nil, pokerReason(tc.reason))

			// La historia conserva sus Story Points y la sesión queda cancelada por el sistema.
			points := pokerStoredStoryPoints(t, f.env.db, storyID)
			if assert.NotNil(t, points, "la historia debería conservar sus Story Points") {
				assert.Equal(t, 5, *points, "user_stories.story_points no debería cambiar")
			}
			requireStoredCanceled(t, f.env.db, session.ID, nil, pokerReason(tc.reason), f.env.clock.Now())
		})
	}
}

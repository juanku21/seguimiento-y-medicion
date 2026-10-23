package delivery

import (
	"encoding/json"
	"net/http"
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

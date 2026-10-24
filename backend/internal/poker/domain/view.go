package domain

import (
	"slices"
	"time"

	"github.com/google/uuid"

	backlogdomain "github.com/juanku21/seguimiento-y-medicion/backend/internal/backlog/domain"
)

// SessionAccess es el tipo de lectura que arma FindAccessible para el middleware
// RequireEstimationAccess (data-model.md sección 8; research.md sección 4). No es un modelo
// persistido: sale de una sola consulta que une la sesión con su historia y con la membresía vigente
// del titular de la petición en el proyecto de la historia.
type SessionAccess struct {
	SessionID uuid.UUID
	StoryID   uuid.UUID
	ProjectID uuid.UUID
}

// UserRef identifica a una persona solo por su identificador y su nombre completo, seleccionados con
// un JOIN a users para que el hash de la contraseña nunca entre al módulo. El JOIN no filtra por
// membresía, así que quien dejó el proyecto conserva su nombre (FR-084).
type UserRef struct {
	ID       uuid.UUID
	FullName string
}

// SessionStory es la historia estimada con su texto vigente: título, descripción y los textos de los
// criterios de aceptación en su orden (FR-012, FR-074). No tiene campo de Story Points: mostrar el
// valor vigente anclaría la votación (FR-075).
type SessionStory struct {
	ID                 uuid.UUID
	ProjectID          uuid.UUID
	Title              string
	Description        *string
	AcceptanceCriteria []string
}

// SessionVote es la carta que jugó una persona en una ronda, con su autor. Es el voto de la
// instantánea y también el que muestra la vista en las rondas reveladas.
type SessionVote struct {
	User UserRef
	Card Card
}

// SnapshotParticipant es un participante tal como lo lee FindSnapshot: la persona, si está conectada
// y, si no lo está, desde cuándo, que ViewFor y FacilitatorTakeover usan para calcular cuándo queda
// disponible el rol de facilitador (FR-068).
type SnapshotParticipant struct {
	User           UserRef
	Connected      bool
	DisconnectedAt *time.Time
}

// SnapshotRound es una ronda de la instantánea con todos sus votos y sus autores, sin ocultar
// ninguno: el secreto del voto lo aplica ViewFor (research.md sección 8). OpenedAt es el momento de
// apertura de la ronda.
type SnapshotRound struct {
	Number     int
	Status     RoundStatus
	OpenedAt   time.Time
	RevealedAt *time.Time
	Votes      []SessionVote
}

// SessionSnapshot es la instantánea completa de una sesión que lee FindSnapshot y que es la entrada
// de ViewFor (data-model.md sección 8; research.md sección 7). No es un modelo persistido: lleva la
// sesión, su historia sin Story Points (FR-075), el facilitador, quien abrió y quien cerró (nil
// mientras está abierta o si la cerró el sistema) con su nombre, los participantes y todas las
// rondas de la 1 a la vigente. Session.Story, Session.Facilitator, Session.OpenedBy y
// Session.ClosedBy nunca se precargan: los datos viajan en Story, Facilitator, OpenedBy y ClosedBy.
type SessionSnapshot struct {
	Session      EstimationSession
	Story        SessionStory
	Facilitator  UserRef
	OpenedBy     UserRef
	ClosedBy     *UserRef
	Participants []SnapshotParticipant
	Rounds       []SnapshotRound
}

// ParticipantView es un participante tal como lo ve quien pide: HasVoted indica si ya votó en la
// ronda vigente, nunca con qué carta mientras está en votación (FR-023).
type ParticipantView struct {
	User          UserRef
	Connected     bool
	IsFacilitator bool
	HasVoted      bool
}

// RoundView es una ronda tal como la ve quien pide. Votes es nil mientras la ronda está en votación,
// para todos, incluido el autor (FR-023, FR-082); Outcome solo se completa en las rondas reveladas,
// y NotVoted lista a los conectados sin voto solo en la ronda vigente revelada de una sesión abierta
// (FR-035).
type RoundView struct {
	Number     int
	Status     RoundStatus
	OpenedAt   time.Time
	RevealedAt *time.Time
	Votes      []SessionVote
	Outcome    *RoundOutcome
	NotVoted   []UserRef
}

// SessionView es la sesión vista por una persona, la única forma en que una sesión sale hacia la API:
// el canal, el GET de la sesión y la respuesta de cada acción (esquema EstimationSession de
// contracts/openapi.yaml; research.md sección 8). La arma ViewFor a partir de la instantánea. MyVote
// es la carta de quien pide en la ronda vigente, y FacilitatorAvailableAt el momento desde el cual
// otro participante puede tomar el rol, nil si el facilitador está conectado o la sesión está
// cerrada (FR-068).
type SessionView struct {
	ID                     uuid.UUID
	Version                int64
	Status                 SessionStatus
	Story                  SessionStory
	Facilitator            UserRef
	OpenedBy               UserRef
	OpenedAt               time.Time
	ClosedBy               *UserRef
	ClosedAt               *time.Time
	AgreedStoryPoints      *int
	CancelReason           *CancelReason
	Cards                  []Card
	Participants           []ParticipantView
	Rounds                 []RoundView
	MyVote                 *Card
	FacilitatorAvailableAt *time.Time
}

// ViewFor arma la vista de la sesión para quien pide a partir de la instantánea completa; es el único
// punto por el que una sesión sale hacia la API (research.md sección 8). Es una función pura sobre la
// instantánea, sin I/O, porque Hub.Publish la ejecuta con su mutex tomado. Copia los datos de la
// sesión, de la historia y de sus participantes y rondas, y aplica el secreto del voto: de la ronda
// vigente solo salen quiénes votaron y la carta propia de quien pide (FR-023, FR-024, FR-082).
func ViewFor(snapshot SessionSnapshot, viewerID uuid.UUID) SessionView {
	session := snapshot.Session
	currentVotes := currentRoundVotes(snapshot.Rounds)
	return SessionView{
		ID:           session.ID,
		Version:      session.Version,
		Status:       session.Status,
		Story:        storyView(snapshot.Story),
		Facilitator:  snapshot.Facilitator,
		OpenedBy:     snapshot.OpenedBy,
		OpenedAt:     session.CreatedAt,
		ClosedBy:     snapshot.ClosedBy,
		ClosedAt:     session.ClosedAt,
		Cards:        slices.Clone(Cards),
		Participants: participantViews(snapshot.Participants, session.FacilitatorID, currentVotes),
		Rounds:       roundViews(snapshot.Rounds),
		MyVote:       myVote(currentVotes, viewerID),
	}
}

// currentRoundVotes devuelve los votos de la ronda vigente, que es la última de la instantánea
// porque las rondas llegan ordenadas de la 1 a la vigente; sin rondas no hay votos.
func currentRoundVotes(rounds []SnapshotRound) []SessionVote {
	if len(rounds) == 0 {
		return nil
	}
	return rounds[len(rounds)-1].Votes
}

// myVote busca la carta de quien pide entre los votos de la ronda vigente y devuelve una copia, o
// nil si todavía no votó en ella.
func myVote(votes []SessionVote, viewerID uuid.UUID) *Card {
	for _, v := range votes {
		if v.User.ID == viewerID {
			card := v.Card
			return &card
		}
	}
	return nil
}

// storyView copia la historia con sus criterios en el orden de la instantánea; una historia sin
// criterios sale con la lista vacía y no nil, para que el JSON lleve un arreglo (US1-4).
func storyView(story SessionStory) SessionStory {
	criteria := make([]string, len(story.AcceptanceCriteria))
	copy(criteria, story.AcceptanceCriteria)
	story.AcceptanceCriteria = criteria
	return story
}

// participantViews arma cada participante con su presencia, marca como facilitador solo a quien
// coincide con el facilitador de la sesión y marca HasVoted a quien tiene voto en la ronda vigente,
// sin exponer la carta.
func participantViews(
	participants []SnapshotParticipant,
	facilitatorID uuid.UUID,
	currentVotes []SessionVote,
) []ParticipantView {
	voters := make(map[uuid.UUID]bool, len(currentVotes))
	for _, v := range currentVotes {
		voters[v.User.ID] = true
	}

	views := make([]ParticipantView, 0, len(participants))
	for _, p := range participants {
		views = append(views, ParticipantView{
			User:          p.User,
			Connected:     p.Connected,
			IsFacilitator: p.User.ID == facilitatorID,
			HasVoted:      voters[p.User.ID],
		})
	}
	return views
}

// roundViews arma las rondas en el orden de la instantánea con su número, su estado y sus momentos
// de apertura y revelado. Solo las rondas reveladas llevan sus votos con sus autores; las que están
// en votación salen con Votes nil para todos (FR-023, FR-082).
func roundViews(rounds []SnapshotRound) []RoundView {
	views := make([]RoundView, 0, len(rounds))
	for _, r := range rounds {
		view := RoundView{
			Number:     r.Number,
			Status:     r.Status,
			OpenedAt:   r.OpenedAt,
			RevealedAt: r.RevealedAt,
		}
		if r.Status == RoundStatusRevealed {
			view.Votes = revealedVotes(r.Votes)
		}
		views = append(views, view)
	}
	return views
}

// revealedVotes copia los votos de una ronda revelada; una ronda sin votos sale con la lista vacía y
// no nil, para que el JSON lleve un arreglo y no se confunda con una ronda en votación.
func revealedVotes(votes []SessionVote) []SessionVote {
	copied := make([]SessionVote, len(votes))
	copy(copied, votes)
	return copied
}

// SessionSummary es una sesión cerrada del historial de una historia (FR-079, FR-080), sin votos de
// ninguna ronda (research.md sección 8). ClosedBy es nil si la cerró el sistema, y RoundCount es la
// cantidad de rondas de la sesión.
type SessionSummary struct {
	ID                uuid.UUID
	Status            SessionStatus
	AgreedStoryPoints *int
	CancelReason      *CancelReason
	OpenedBy          UserRef
	OpenedAt          time.Time
	ClosedBy          *UserRef
	ClosedAt          time.Time
	RoundCount        int
}

// OpenSessionSummary es la sesión abierta de una historia tal como la muestra su historial: el
// facilitador y el número y el estado de la ronda vigente, sin votos (FR-081, FR-082).
type OpenSessionSummary struct {
	ID          uuid.UUID
	Facilitator UserRef
	RoundNumber int
	RoundStatus RoundStatus
}

// OpenSessionStatus es una sesión abierta tal como la lee el barrido periódico (research.md sección
// 13): el estado de su historia, si el proyecto está finalizado y el último hecho alimentan
// AutoCancelReason, y StoryUpdatedAt (user_stories.updated_at) permite detectar cambios en el texto
// de la historia (FR-074).
type OpenSessionStatus struct {
	SessionID       uuid.UUID
	StoryStatus     backlogdomain.StoryStatus
	ProjectFinished bool
	LastActivityAt  time.Time
	StoryUpdatedAt  time.Time
}

// ParticipantRef identifica a un participante conectado cuya membresía en el proyecto fue revocada,
// para que el barrido lo desconecte y cierre sus canales (FR-019).
type ParticipantRef struct {
	SessionID uuid.UUID
	UserID    uuid.UUID
}

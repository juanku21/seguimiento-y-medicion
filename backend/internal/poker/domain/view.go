package domain

import (
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
// punto por el que una sesión sale hacia la API (research.md sección 8). Firma mínima de T013: la
// implementa T021.
func ViewFor(snapshot SessionSnapshot, viewerID uuid.UUID) SessionView {
	return SessionView{}
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

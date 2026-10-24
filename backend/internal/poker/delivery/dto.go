package delivery

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/juanku21/seguimiento-y-medicion/backend/internal/poker/domain"
)

// Los errores reutilizan sin cambios ErrorResponse, FieldError y ValidationErrorResponse de
// auth/delivery (spec 001); acá se declaran solo los esquemas propios del Planning Poker
// (contracts/openapi.yaml). Las fechas salen en UTC con formato RFC 3339, los campos anulables como
// null y las listas como [], nunca como null, salvo votes, que el contrato declara anulable.

// UserRef identifica a una persona por su nombre, aunque haya dejado de ser integrante (FR-084).
type UserRef struct {
	// Identificador de la cuenta.
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo de la persona.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
}

// EstimationStory es la historia estimada con su texto vigente (FR-012, FR-074). No incluye sus
// Story Points vigentes, para no anclar la votación (FR-075).
type EstimationStory struct {
	// Identificador de la historia.
	ID string `json:"id" validate:"required" format:"uuid" example:"7c4e2a19-3b8d-4f60-a1c5-9e2d0b6f8a73"`
	// Identificador del proyecto de la historia.
	ProjectID string `json:"projectId" validate:"required" format:"uuid" example:"9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35"`
	// Título de la historia.
	Title string `json:"title" validate:"required" example:"Exportar el reporte de velocidad a PDF"`
	// Descripción de la historia, o null si no tiene.
	Description *string `json:"description" validate:"required" extensions:"x-nullable" example:"El equipo necesita compartir la velocidad con la gerencia."`
	// Textos de los criterios en su orden; vacía si no tiene (US1-4).
	AcceptanceCriteria []string `json:"acceptanceCriteria" validate:"required" example:"El PDF incluye la velocidad de los últimos cinco sprints."`
}

// EstimationParticipant es un participante de la sesión con su presencia y su rol.
type EstimationParticipant struct {
	// Persona participante.
	User UserRef `json:"user" validate:"required"`
	// Conectado o desconectado (FR-013, FR-014).
	Connected bool `json:"connected" validate:"required" example:"true"`
	// Verdadero si es el facilitador de la sesión.
	IsFacilitator bool `json:"isFacilitator" validate:"required" example:"false"`
	// Si ya votó en la ronda vigente; nunca el valor mientras esté en votación (FR-023).
	HasVoted bool `json:"hasVoted" validate:"required" example:"false"`
}

// EstimationVote es la carta que jugó una persona en una ronda revelada. Se declara acá porque
// EstimationRound la referencia en votes.
type EstimationVote struct {
	// Autor del voto.
	User UserRef `json:"user" validate:"required"`
	// Carta jugada; "?" expresa que la persona no sabe estimar (FR-020, RN-06).
	Card domain.Card `json:"card" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100,?" example:"5"`
}

// OutcomeExtreme es el mínimo o el máximo de una ronda divergente, con todos los autores de ese valor
// (FR-039, US3-8).
type OutcomeExtreme struct {
	// Valor de Story Points del extremo.
	Value int `json:"value" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100" example:"3"`
	// Todos los autores del valor, ordenados por nombre (FR-039, US3-8).
	Authors []UserRef `json:"authors" validate:"required"`
}

// RoundOutcome es el resultado de una ronda revelada. consensus lleva value y no lleva min ni max;
// divergence lleva min y max; no_numeric_votes no lleva ninguno (FR-036 a FR-041). No hay promedio ni
// mediana (FR-042).
type RoundOutcome struct {
	// Tipo de resultado: consensus (Consenso), divergence (Divergencia) o no_numeric_votes (sin votos
	// numéricos).
	Kind domain.OutcomeKind `json:"kind" validate:"required" enums:"consensus,divergence,no_numeric_votes" example:"divergence"`
	// Valor del consenso, o null en los demás resultados.
	Value *int `json:"value" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"5"`
	// Mínimo de la divergencia, o null en los demás resultados.
	Min *OutcomeExtreme `json:"min" extensions:"x-nullable"`
	// Máximo de la divergencia, o null en los demás resultados.
	Max *OutcomeExtreme `json:"max" extensions:"x-nullable"`
}

// EstimationRound es una ronda de votación tal como la ve quien pide.
type EstimationRound struct {
	// Número de la ronda, desde 1.
	Number int `json:"number" validate:"required" minimum:"1" example:"1"`
	// Estado de la ronda: voting (En votación) o revealed (Revelada).
	Status domain.RoundStatus `json:"status" validate:"required" enums:"voting,revealed" example:"voting"`
	// Fecha y hora de apertura de la ronda (RFC 3339).
	OpenedAt string `json:"openedAt" validate:"required" format:"date-time" example:"2026-10-20T14:30:00Z"`
	// Fecha y hora del revelado (RFC 3339), o null mientras está en votación.
	RevealedAt *string `json:"revealedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-20T14:35:00Z"`
	// Votos de la ronda; null mientras está en votación, para todos, incluido el autor: el propio
	// voto viaja en myVote (FR-023, FR-082).
	Votes []EstimationVote `json:"votes" validate:"required" extensions:"x-nullable"`
	// Resultado de la ronda; solo en rondas reveladas.
	Outcome *RoundOutcome `json:"outcome" validate:"required" extensions:"x-nullable"`
	// En la ronda vigente revelada de una sesión abierta, participantes conectados sin voto (FR-035).
	// Vacía en las demás rondas.
	NotVoted []UserRef `json:"notVoted" validate:"required"`
}

// EstimationSession es la sesión vista por quien pide: el cuerpo de las acciones, del GET de la
// sesión y de cada evento state del canal.
type EstimationSession struct {
	// Identificador de la sesión.
	ID string `json:"id" validate:"required" format:"uuid" example:"5d2b8f71-0c3a-4e9d-b6a4-2f1e7c9d0a85"`
	// Sube con cada hecho; el cliente descarta instantáneas de versión menor.
	Version int64 `json:"version" validate:"required" format:"int64" example:"1"`
	// Estado de la sesión: open (Abierta), finished (Finalizada) o canceled (Cancelada).
	Status domain.SessionStatus `json:"status" validate:"required" enums:"open,finished,canceled" example:"open"`
	// Historia estimada, sin sus Story Points vigentes (FR-075).
	Story EstimationStory `json:"story" validate:"required"`
	// Facilitador vigente de la sesión.
	Facilitator UserRef `json:"facilitator" validate:"required"`
	// Persona que abrió la sesión.
	OpenedBy UserRef `json:"openedBy" validate:"required"`
	// Fecha y hora de apertura (RFC 3339).
	OpenedAt string `json:"openedAt" validate:"required" format:"date-time" example:"2026-10-20T14:30:00Z"`
	// Persona que cerró la sesión; null mientras está abierta y cuando la cerró el sistema.
	ClosedBy *UserRef `json:"closedBy" validate:"required" extensions:"x-nullable"`
	// Fecha y hora de cierre (RFC 3339), o null mientras está abierta.
	ClosedAt *string `json:"closedAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-20T15:00:00Z"`
	// Story Points acordados, o null si la sesión no terminó con un valor registrado.
	AgreedStoryPoints *int `json:"agreedStoryPoints" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"8"`
	// Motivo de la cancelación del sistema, o null (FR-061, FR-063).
	CancelReason *domain.CancelReason `json:"cancelReason" validate:"required" enums:"inactivity,story_completed,project_finished" extensions:"x-nullable" example:"inactivity"`
	// Escala de cartas disponible (FR-012).
	Cards []domain.Card `json:"cards" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100,?" example:"0,1,2,3,5,8,13,20,40,100,?"`
	// Participantes de la sesión.
	Participants []EstimationParticipant `json:"participants" validate:"required"`
	// Todas las rondas, de la 1 a la vigente (FR-044, FR-078).
	Rounds []EstimationRound `json:"rounds" validate:"required"`
	// Carta de quien pide en la ronda vigente, igual en todas sus conexiones, o null (FR-024).
	MyVote *domain.Card `json:"myVote" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100,?" extensions:"x-nullable" example:"5"`
	// Si el facilitador está desconectado, momento a partir del cual otro participante puede tomar el
	// rol (FR-068); null si está conectado o la sesión está cerrada.
	FacilitatorAvailableAt *string `json:"facilitatorAvailableAt" validate:"required" format:"date-time" extensions:"x-nullable" example:"2026-10-20T14:32:00Z"`
}

// VoteRequest es el cuerpo del voto (contracts/openapi.yaml). Card llega como json.RawMessage para
// distinguir la carta ausente, nula o vacía, que es un intento de retirar el voto (RN-36), de un
// número JSON, que es una carta no válida (research.md sección 10).
type VoteRequest struct {
	// Carta jugada, siempre como texto; "?" expresa que la persona no sabe estimar (FR-020).
	Card json.RawMessage `json:"card" validate:"required" swaggertype:"string" enums:"0,1,2,3,5,8,13,20,40,100,?" example:"5"`
}

// FinalizeRequest es el cuerpo del registro de la estimación acordada (contracts/openapi.yaml).
// StoryPoints llega como json.RawMessage para rechazar con el mismo mensaje el valor ausente, nulo,
// textual o decimal (FR-050, research.md sección 12).
type FinalizeRequest struct {
	// Valor acordado de la escala numérica de Story Points (FR-049).
	StoryPoints json.RawMessage `json:"storyPoints" validate:"required" swaggertype:"integer" enums:"0,1,2,3,5,8,13,20,40,100" example:"8"`
}

// OpenSessionExistsResponse es el cuerpo del 409 al abrir una sesión sobre una historia que ya tiene
// una en curso, con la sesión a la que se puede unir (FR-006).
type OpenSessionExistsResponse struct {
	// Mensaje del rechazo.
	Error string `json:"error" validate:"required" example:"La historia ya tiene una sesión de estimación en curso."`
	// Sesión abierta a la que se puede unir (FR-006).
	SessionID string `json:"sessionId" validate:"required" format:"uuid" example:"5d2b8f71-0c3a-4e9d-b6a4-2f1e7c9d0a85"`
}

// SessionClosedResponse es el cuerpo del 409 sobre una sesión cerrada, con el resultado registrado
// (FR-018, FR-055, FR-060).
type SessionClosedResponse struct {
	// Mensaje del rechazo.
	Error string `json:"error" validate:"required" example:"La sesión de estimación ya está cerrada."`
	// Estado de la sesión: finished (Finalizada) o canceled (Cancelada).
	Status domain.SessionStatus `json:"status" validate:"required" enums:"finished,canceled" example:"finished"`
	// Story Points acordados, o null si la sesión no terminó con un valor registrado.
	AgreedStoryPoints *int `json:"agreedStoryPoints" validate:"required" enums:"0,1,2,3,5,8,13,20,40,100" extensions:"x-nullable" example:"8"`
	// Motivo de la cancelación del sistema, o null.
	CancelReason *domain.CancelReason `json:"cancelReason" validate:"required" enums:"inactivity,story_completed,project_finished" extensions:"x-nullable" example:"inactivity"`
}

// newOpenSessionExistsResponse arma el cuerpo del 409 de apertura con el mensaje recibido y la sesión
// en curso que informa el error de dominio.
func newOpenSessionExistsResponse(message string, sessionID uuid.UUID) OpenSessionExistsResponse {
	return OpenSessionExistsResponse{Error: message, SessionID: sessionID.String()}
}

// newSessionClosedResponse arma el cuerpo del 409 de sesión cerrada con el mensaje recibido y el
// estado, el valor acordado y el motivo que informa el error de dominio.
func newSessionClosedResponse(message string, closed *domain.SessionClosedError) SessionClosedResponse {
	return SessionClosedResponse{
		Error:             message,
		Status:            closed.Status,
		AgreedStoryPoints: closed.AgreedStoryPoints,
		CancelReason:      closed.CancelReason,
	}
}

// newEstimationSession arma EstimationSession del contrato a partir de la vista de dominio. Es el
// único lugar que garantiza la forma de la respuesta: los punteros nulos se serializan como null y
// las listas de cartas, participantes y rondas nunca quedan nulas, de modo que se serializan como [].
func newEstimationSession(view *domain.SessionView) EstimationSession {
	cards := make([]domain.Card, len(view.Cards))
	copy(cards, view.Cards)

	participants := make([]EstimationParticipant, 0, len(view.Participants))
	for _, p := range view.Participants {
		participants = append(participants, EstimationParticipant{
			User:          newUserRef(p.User),
			Connected:     p.Connected,
			IsFacilitator: p.IsFacilitator,
			HasVoted:      p.HasVoted,
		})
	}

	rounds := make([]EstimationRound, 0, len(view.Rounds))
	for _, r := range view.Rounds {
		rounds = append(rounds, newEstimationRound(r))
	}

	return EstimationSession{
		ID:                     view.ID.String(),
		Version:                view.Version,
		Status:                 view.Status,
		Story:                  newEstimationStory(view.Story),
		Facilitator:            newUserRef(view.Facilitator),
		OpenedBy:               newUserRef(view.OpenedBy),
		OpenedAt:               formatTime(view.OpenedAt),
		ClosedBy:               newOptionalUserRef(view.ClosedBy),
		ClosedAt:               formatOptionalTime(view.ClosedAt),
		AgreedStoryPoints:      view.AgreedStoryPoints,
		CancelReason:           view.CancelReason,
		Cards:                  cards,
		Participants:           participants,
		Rounds:                 rounds,
		MyVote:                 view.MyVote,
		FacilitatorAvailableAt: formatOptionalTime(view.FacilitatorAvailableAt),
	}
}

// newEstimationStory convierte la historia de la vista; una historia sin criterios sale con la lista
// vacía y no nil (US1-4).
func newEstimationStory(story domain.SessionStory) EstimationStory {
	criteria := make([]string, len(story.AcceptanceCriteria))
	copy(criteria, story.AcceptanceCriteria)
	return EstimationStory{
		ID:                 story.ID.String(),
		ProjectID:          story.ProjectID.String(),
		Title:              story.Title,
		Description:        story.Description,
		AcceptanceCriteria: criteria,
	}
}

// newEstimationRound convierte una ronda de la vista. Los votos y el resultado siguen nil mientras la
// vista los oculta, para que se serialicen como null (FR-023); notVoted siempre sale como lista.
func newEstimationRound(round domain.RoundView) EstimationRound {
	var votes []EstimationVote
	if round.Votes != nil {
		votes = make([]EstimationVote, 0, len(round.Votes))
		for _, v := range round.Votes {
			votes = append(votes, EstimationVote{User: newUserRef(v.User), Card: v.Card})
		}
	}

	notVoted := make([]UserRef, 0, len(round.NotVoted))
	for _, u := range round.NotVoted {
		notVoted = append(notVoted, newUserRef(u))
	}

	return EstimationRound{
		Number:     round.Number,
		Status:     round.Status,
		OpenedAt:   formatTime(round.OpenedAt),
		RevealedAt: formatOptionalTime(round.RevealedAt),
		Votes:      votes,
		Outcome:    newRoundOutcome(round.Outcome),
		NotVoted:   notVoted,
	}
}

// newRoundOutcome convierte el resultado de una ronda revelada, o devuelve nil para que una ronda en
// votación lo serialice como null. El valor y los extremos ausentes quedan nil y salen como null.
func newRoundOutcome(outcome *domain.RoundOutcome) *RoundOutcome {
	if outcome == nil {
		return nil
	}
	return &RoundOutcome{
		Kind:  outcome.Kind,
		Value: outcome.Value,
		Min:   newOutcomeExtreme(outcome.Min),
		Max:   newOutcomeExtreme(outcome.Max),
	}
}

// newOutcomeExtreme convierte un extremo de la divergencia conservando el orden de sus autores, o
// devuelve nil si el resultado no lo lleva; los autores siempre salen como lista, nunca como null.
func newOutcomeExtreme(extreme *domain.OutcomeExtreme) *OutcomeExtreme {
	if extreme == nil {
		return nil
	}
	authors := make([]UserRef, 0, len(extreme.Authors))
	for _, u := range extreme.Authors {
		authors = append(authors, newUserRef(u))
	}
	return &OutcomeExtreme{Value: extreme.Value, Authors: authors}
}

// newUserRef convierte una persona de la vista al esquema UserRef.
func newUserRef(user domain.UserRef) UserRef {
	return UserRef{ID: user.ID.String(), FullName: user.FullName}
}

// newOptionalUserRef convierte una persona opcional, o devuelve nil para que se serialice como null.
func newOptionalUserRef(user *domain.UserRef) *UserRef {
	if user == nil {
		return nil
	}
	ref := newUserRef(*user)
	return &ref
}

// formatTime convierte un momento a UTC con formato RFC 3339.
func formatTime(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

// formatOptionalTime convierte un momento opcional a UTC con formato RFC 3339, o devuelve nil para
// que se serialice como null.
func formatOptionalTime(moment *time.Time) *string {
	if moment == nil {
		return nil
	}
	formatted := formatTime(*moment)
	return &formatted
}

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionRepository declara solo las operaciones sobre las sesiones de estimación que usa esta
// feature (data-model.md sección 9; Principio III). Las escrituras sobre una sesión abierta, desde
// Connect hasta TakeFacilitator, bloquean la sesión con SELECT … FOR UPDATE dentro de su
// transacción, verifican EnsureOpen, reevalúan AutoCancelReason (si corresponde cancelan, confirman
// y devuelven SessionClosedError con el motivo), aplican las reglas puras sobre los datos releídos y,
// si la escritura es un hecho, fijan last_activity_at = now y suben version. Toda escritura recibe el
// instante now, que el servicio toma del reloj inyectado. Ninguna operación devuelve
// gorm.ErrRecordNotFound ni (nil, nil) para un recurso obligatorio: la sesión inexistente devuelve
// ErrSessionNotFound. Solo FindOpenForStory devuelve nil sin error, porque la sesión abierta es
// opcional.
type SessionRepository interface {
	// FindAccessible resuelve RequireEstimationAccess en una consulta; sin sesión o sin membresía
	// vigente del titular devuelve ErrSessionNotFound.
	FindAccessible(ctx context.Context, sessionID, userID uuid.UUID) (*SessionAccess, error)

	// Open bloquea la historia y crea la sesión con la ronda 1 y a quien abre como facilitador y
	// participante conectado; rechaza con OpenSessionExistsError, ErrStoryCompleted o
	// ErrProjectFinished.
	Open(ctx context.Context, storyID, openerID uuid.UUID, now time.Time) (*EstimationSession, error)

	// Connect da de alta o reconecta al participante y evalúa la revelación automática.
	Connect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// Disconnect marca al participante desconectado y evalúa la revelación automática.
	Disconnect(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// Vote registra o cambia la carta de un participante conectado en la ronda vigente en votación y
	// evalúa la revelación automática.
	Vote(ctx context.Context, sessionID, userID uuid.UUID, card Card, now time.Time) error

	// Reveal revela manualmente la ronda vigente; lo reserva al facilitador y exige CanRevealManually.
	Reveal(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// StartRound abre la ronda siguiente; lo reserva al facilitador y exige la vigente revelada.
	StartRound(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// Finalize bloquea la sesión y la historia, exige al facilitador y al menos una ronda revelada,
	// cierra la sesión con el valor acordado y escribe story_points en la historia.
	Finalize(ctx context.Context, sessionID, userID uuid.UUID, storyPoints int, now time.Time) error

	// Cancel cancela la sesión a pedido del facilitador; la historia no se toca.
	Cancel(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// TakeFacilitator transfiere el rol de facilitador a quien lo pide si FacilitatorTakeover lo
	// admite.
	TakeFacilitator(ctx context.Context, sessionID, userID uuid.UUID, now time.Time) error

	// AutoCancel bloquea la sesión, reevalúa AutoCancelReason y la cancela si corresponde; informa si
	// la canceló.
	AutoCancel(ctx context.Context, sessionID uuid.UUID, now time.Time) (bool, error)

	// ResetPresence marca desconectados a todos los participantes de las sesiones abiertas al
	// arrancar el servidor, sin que cuente como hecho.
	ResetPresence(ctx context.Context, now time.Time) error

	// ListOpenForSweep devuelve las sesiones abiertas con los datos que necesita el barrido.
	ListOpenForSweep(ctx context.Context) ([]OpenSessionStatus, error)

	// ListRevokedParticipants devuelve los participantes conectados que ya no son integrantes del
	// proyecto.
	ListRevokedParticipants(ctx context.Context) ([]ParticipantRef, error)

	// FindSnapshot devuelve la instantánea completa de la sesión, entrada de ViewFor.
	FindSnapshot(ctx context.Context, sessionID uuid.UUID) (*SessionSnapshot, error)

	// ListForStory devuelve las sesiones cerradas de la historia, de la más reciente a la más
	// antigua.
	ListForStory(ctx context.Context, storyID uuid.UUID) ([]SessionSummary, error)

	// FindOpenForStory devuelve la sesión abierta de la historia, o nil sin error si no tiene.
	FindOpenForStory(ctx context.Context, storyID uuid.UUID) (*OpenSessionSummary, error)
}

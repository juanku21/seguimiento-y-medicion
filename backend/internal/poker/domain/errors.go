package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errores de dominio del módulo de Planning Poker (data-model.md sección 9). Son centinelas que el
// servicio devuelve y que delivery compara con errors.Is para traducirlos a la respuesta HTTP del
// contrato; el texto visible para la persona lo define delivery, no estos mensajes.
// ErrSessionNotFound unifica la sesión inexistente y la ajena (404). ErrNotParticipant y
// ErrFacilitatorOnly cubren los permisos (403). Los demás son conflictos (409): abrir sobre una
// historia completada o en un proyecto finalizado, votar o revelar sobre una ronda revelada, iniciar
// una ronda con la vigente en votación, revelar manualmente sin votos y registrar el valor acordado
// sin ninguna ronda revelada.
var (
	ErrSessionNotFound  = errors.New("la sesión de estimación no existe")
	ErrNotParticipant   = errors.New("quien actúa no es participante conectado de la sesión")
	ErrFacilitatorOnly  = errors.New("la acción está reservada al facilitador de la sesión")
	ErrStoryCompleted   = errors.New("la historia de usuario está completada y no se puede estimar")
	ErrProjectFinished  = errors.New("el proyecto está finalizado y no admite sesiones de estimación")
	ErrRoundRevealed    = errors.New("la ronda ya fue revelada")
	ErrRoundNotRevealed = errors.New("la ronda vigente todavía está en votación")
	ErrNoVotesToReveal  = errors.New("la ronda no tiene votos para revelar")
	ErrNoRevealedRound  = errors.New("la sesión no tiene ninguna ronda revelada")
)

// SessionClosedError rechaza una acción o una incorporación sobre una sesión cerrada. Status indica
// si terminó finished o canceled; CancelReason solo se completa en las cancelaciones del sistema y
// AgreedStoryPoints solo cuando la sesión terminó con un valor registrado. Delivery lo reconoce con
// errors.As para armar la respuesta 409.
type SessionClosedError struct {
	Status            SessionStatus
	CancelReason      *CancelReason
	AgreedStoryPoints *int
}

// Error devuelve un mensaje fijo; el estado y el resultado se exponen en los campos y no en el texto.
func (e *SessionClosedError) Error() string {
	return "la sesión de estimación está cerrada"
}

// OpenSessionExistsError rechaza abrir una sesión sobre una historia que ya tiene una abierta,
// indicada en SessionID, para que delivery la nombre en la respuesta 409. Delivery lo reconoce con
// errors.As.
type OpenSessionExistsError struct {
	SessionID uuid.UUID
}

// Error devuelve un mensaje fijo; la sesión en curso se expone en SessionID y no en el texto.
func (e *OpenSessionExistsError) Error() string {
	return "la historia de usuario ya tiene una sesión de estimación abierta"
}

// FacilitatorUnavailableError rechaza tomar el rol de facilitador sin cumplir FR-068. Facilitator
// identifica al facilitador vigente con su identificador y su nombre completo, y RetryAfter indica
// cuánto falta para que el rol quede disponible: es cero si el facilitador está conectado. Delivery
// lo reconoce con errors.As para armar la respuesta 409.
type FacilitatorUnavailableError struct {
	Facilitator struct {
		ID       uuid.UUID
		FullName string
	}
	RetryAfter time.Duration
}

// Error devuelve un mensaje fijo; el facilitador y la espera se exponen en los campos y no en el
// texto.
func (e *FacilitatorUnavailableError) Error() string {
	return "el rol de facilitador todavía no está disponible"
}

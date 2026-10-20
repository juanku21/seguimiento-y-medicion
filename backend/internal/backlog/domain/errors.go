package domain

import "errors"

// Errores de dominio del módulo de backlog. Son centinelas que el servicio devuelve y que delivery
// compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el texto visible para la
// persona lo define delivery, no estos mensajes. ErrStoryNotFound unifica la historia inexistente,
// eliminada o de un proyecto del que quien pide no es integrante, para no revelar su existencia
// (FR-044, FR-047). ErrBacklogReadOnly rechaza toda escritura sobre el backlog de un proyecto
// finalizado (FR-048). ErrStoryCompleted impide modificar o reestimar una historia completada
// (FR-030). ErrCommittedStoryMustStayReady impide quitar los Story Points o el último criterio de
// aceptación a una historia comprometida en un sprint (FR-018, FR-022).
var (
	ErrStoryNotFound               = errors.New("la historia de usuario no existe")
	ErrBacklogReadOnly             = errors.New("el proyecto está finalizado y su backlog es de solo lectura")
	ErrStoryCompleted              = errors.New("la historia de usuario está completada y no se puede modificar")
	ErrCommittedStoryMustStayReady = errors.New("la historia comprometida en un sprint debe conservar sus story points y al menos un criterio de aceptación")
)

// DeletionBlocker es el código de un motivo que impide eliminar una historia; viaja como texto en el
// campo code de DeletionBlockedReason del contrato (data-model.md sección 7, FR-043).
type DeletionBlocker string

// Motivos de bloqueo de la eliminación, declarados en el orden en que se listan en el rechazo:
// historial de sprint, esfuerzo registrado, defectos asociados y sesiones de Planning Poker.
const (
	DeletionBlockerSprintHistory      DeletionBlocker = "sprint_history"
	DeletionBlockerEffort             DeletionBlocker = "effort"
	DeletionBlockerDefects            DeletionBlocker = "defects"
	DeletionBlockerEstimationSessions DeletionBlocker = "estimation_sessions"
)

// DeletionBlockedError rechaza la eliminación de una historia con historial asociado y lleva todos
// los motivos que aplican, para que delivery los liste en la respuesta 409 (FR-043). Delivery lo
// reconoce con errors.As.
type DeletionBlockedError struct {
	Reasons []DeletionBlocker
}

// Error devuelve un mensaje fijo; los motivos se exponen en Reasons y no en el texto.
func (e *DeletionBlockedError) Error() string {
	return "la historia de usuario tiene historial asociado y no se puede eliminar"
}

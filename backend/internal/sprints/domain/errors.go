package domain

import (
	"errors"

	"github.com/google/uuid"
)

// Errores de dominio del módulo de sprints (data-model.md sección 9). Son centinelas que el servicio
// devuelve y que delivery compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el
// texto visible para la persona lo define delivery, no estos mensajes. ErrSprintNotFound unifica el
// sprint inexistente, eliminado o de un proyecto ajeno, y la historia inexistente reutiliza
// ErrStoryNotFound de backlog. ErrSprintsReadOnly rechaza toda escritura sobre los sprints de un
// proyecto finalizado. Los demás cubren las transiciones del sprint (iniciar uno activo, modificar
// uno activo, cerrar uno planificado, escribir sobre uno cerrado, eliminar uno ya iniciado o con
// esfuerzo), la remoción de una historia completada y el avance de las historias dentro del sprint.
var (
	ErrSprintNotFound              = errors.New("el sprint no existe")
	ErrSprintsReadOnly             = errors.New("el proyecto está finalizado y sus sprints son de solo lectura")
	ErrInvalidSprintTransition     = errors.New("transición de estado del sprint no permitida")
	ErrSprintNotPlanned            = errors.New("el sprint no está planificado")
	ErrSprintNotActive             = errors.New("el sprint no está activo")
	ErrSprintClosed                = errors.New("el sprint está cerrado y no se puede modificar")
	ErrSprintAlreadyStarted        = errors.New("el sprint ya fue iniciado y no se puede eliminar")
	ErrSprintHasEffort             = errors.New("el sprint tiene esfuerzo registrado y no se puede eliminar")
	ErrCompletedStoryRemoval       = errors.New("la historia de usuario está completada y no se puede quitar del sprint")
	ErrProgressOutsideActiveSprint = errors.New("la historia de usuario no está en un sprint activo")
	ErrInvalidStoryTransition      = errors.New("transición de estado de la historia de usuario no permitida")
)

// ConflictingSprint identifica el sprint que provoca un conflicto (superposición, sprint activo o
// compromiso abierto de una historia) para que delivery lo nombre en la respuesta 409. Es también un
// tipo de lectura de data-model.md sección 8.
type ConflictingSprint struct {
	ID   uuid.UUID
	Name string
}

// SprintOverlapError rechaza crear o modificar un sprint cuyo período se superpone con el de otro
// sprint del proyecto, indicado en Sprint. Delivery lo reconoce con errors.As.
type SprintOverlapError struct {
	Sprint ConflictingSprint
}

// Error devuelve un mensaje fijo; el sprint en conflicto se expone en Sprint y no en el texto.
func (e *SprintOverlapError) Error() string {
	return "el período del sprint se superpone con el de otro sprint del proyecto"
}

// ActiveSprintExistsError rechaza iniciar un sprint cuando el proyecto ya tiene otro activo,
// indicado en Sprint. Delivery lo reconoce con errors.As.
type ActiveSprintExistsError struct {
	Sprint ConflictingSprint
}

// Error devuelve un mensaje fijo; el sprint activo se expone en Sprint y no en el texto.
func (e *ActiveSprintExistsError) Error() string {
	return "el proyecto ya tiene un sprint activo"
}

// AdmissionReason es el código del motivo por el que una historia no se admite en un sprint; viaja
// como texto en el cuerpo del 409 de asignación (data-model.md sección 9).
type AdmissionReason string

// Motivos de no admisión: la historia no está lista para planificar, ya está completada o ya está
// comprometida en otro sprint abierto.
const (
	AdmissionReasonNotReady           AdmissionReason = "not_ready"
	AdmissionReasonCompleted          AdmissionReason = "completed"
	AdmissionReasonCommittedElsewhere AdmissionReason = "committed_elsewhere"
)

// AdmissionProblem describe una historia no admitida en una asignación. Missing lista lo que le falta
// (story_points y/o acceptance_criteria) solo cuando Reason es not_ready, y Sprint indica el sprint
// en el que ya está comprometida solo cuando Reason es committed_elsewhere.
type AdmissionProblem struct {
	StoryID uuid.UUID
	Reason  AdmissionReason
	Missing []string
	Sprint  *ConflictingSprint
}

// StoriesNotAdmittedError rechaza en bloque una asignación con historias no admisibles y lleva un
// problema por cada una, para que delivery los liste en la respuesta 409. Delivery lo reconoce con
// errors.As.
type StoriesNotAdmittedError struct {
	Problems []AdmissionProblem
}

// Error devuelve un mensaje fijo; los problemas se exponen en Problems y no en el texto.
func (e *StoriesNotAdmittedError) Error() string {
	return "hay historias de usuario que no se pueden asignar al sprint"
}

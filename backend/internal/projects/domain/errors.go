package domain

import "errors"

// Errores de dominio del módulo de proyectos. Son centinelas que el servicio devuelve y que
// delivery compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el texto visible
// para la persona lo define delivery, no estos mensajes. ErrProjectNameTaken cubre el nombre ya usado
// por el mismo propietario, incluida la violación del índice único bajo concurrencia (FR-004).
// ErrProjectNotFound unifica el proyecto inexistente y el proyecto del que quien pide no es
// integrante, para no revelar su existencia (FR-049). ErrNotProjectOwner es la falta de permiso de
// un integrante que no es el propietario (FR-050). ErrProjectReadOnly rechaza cualquier cambio sobre
// un proyecto finalizado (FR-018). ErrUserNotRegistered, ErrAlreadyMember, ErrNotMember y
// ErrOwnerCannotLeave cubren el alta y la baja de integrantes (FR-021, FR-022, FR-026, FR-025).
// ErrInvalidStatusTransition rechaza toda transición fuera de la cadena permitida (FR-038) y
// ErrActiveSprintBlocksFinish impide finalizar mientras haya un sprint activo (FR-041).
var (
	ErrProjectNameTaken         = errors.New("ya existe un proyecto propio con ese nombre")
	ErrProjectNotFound          = errors.New("el proyecto no existe")
	ErrNotProjectOwner          = errors.New("solo el propietario del proyecto puede realizar esta acción")
	ErrProjectReadOnly          = errors.New("el proyecto está finalizado y es de solo lectura")
	ErrUserNotRegistered        = errors.New("no existe un usuario registrado con ese correo electrónico")
	ErrAlreadyMember            = errors.New("la persona ya forma parte del proyecto")
	ErrNotMember                = errors.New("la persona no forma parte del proyecto")
	ErrOwnerCannotLeave         = errors.New("el propietario no puede quitarse del proyecto")
	ErrInvalidStatusTransition  = errors.New("transición de estado no permitida")
	ErrActiveSprintBlocksFinish = errors.New("el proyecto tiene un sprint activo que hay que cerrar antes de finalizarlo")
)

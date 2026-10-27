package domain

import "errors"

// Errores de dominio del módulo de defectos (data-model.md sección 8). Son centinelas que el servicio
// devuelve y que delivery compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el
// texto visible para la persona lo define delivery, no estos mensajes. ErrDefectNotFound unifica el
// defecto inexistente o de un proyecto ajeno. ErrDefectsReadOnly rechaza toda escritura sobre los
// defectos de un proyecto finalizado y ErrNoDetectionSprint indica que no hay un sprint Activo o
// Cerrado que pueda ser de detección. ErrDefectDiscarded rechaza modificar un defecto Descartado y
// ErrLinksFrozen rechaza corregir la historia o el sprint de detección fuera del estado Abierto.
// ErrDetectionSprintPlanned y ErrResolutionSprintPlanned rechazan un sprint Planificado como sprint de
// detección o de resolución, ErrResolutionSprintRequired exige el sprint de resolución al resolver y
// ErrResolutionBeforeDetection rechaza una resolución que empieza antes que la detección.
// ErrInvalidFilter rechaza un filtro de la consulta que no corresponde al proyecto.
var (
	ErrDefectNotFound            = errors.New("el defecto no existe")
	ErrDefectsReadOnly           = errors.New("el proyecto está finalizado y sus defectos son de solo lectura")
	ErrNoDetectionSprint         = errors.New("no hay ningún sprint activo ni cerrado que pueda ser sprint de detección")
	ErrDefectDiscarded           = errors.New("el defecto está descartado y es de solo lectura")
	ErrLinksFrozen               = errors.New("la historia relacionada y el sprint de detección solo se corrigen con el defecto abierto")
	ErrDetectionSprintPlanned    = errors.New("un defecto solo se detecta en un sprint activo o cerrado")
	ErrResolutionSprintRequired  = errors.New("el sprint de resolución es obligatorio para resolver")
	ErrResolutionSprintPlanned   = errors.New("un defecto solo se resuelve en un sprint activo o cerrado")
	ErrResolutionBeforeDetection = errors.New("el sprint de resolución no puede empezar antes que el de detección")
	ErrInvalidFilter             = errors.New("el filtro no corresponde al proyecto")
)

// InvalidTransitionError rechaza una transición de estado no admitida desde Current. Lleva el estado
// pedido en Requested, para que delivery elija el mensaje del caso, y los estados admitidos en
// Allowed. Delivery lo reconoce con errors.As.
type InvalidTransitionError struct {
	Current   DefectStatus
	Requested DefectStatus
	Allowed   []DefectStatus
}

// Error devuelve un mensaje fijo; los estados se exponen en los campos y no en el texto.
func (e *InvalidTransitionError) Error() string {
	return "la transición de estado del defecto no está admitida"
}

// InvalidFilterError rechaza un filtro de la consulta de defectos e indica en Field el nombre del
// parámetro, para que delivery lo informe como campo del 400. Equivale a ErrInvalidFilter con
// errors.Is y delivery lo reconoce con errors.As para leer el campo.
type InvalidFilterError struct {
	Field string
}

// Error devuelve un mensaje fijo; el parámetro rechazado se expone en Field y no en el texto.
func (e *InvalidFilterError) Error() string {
	return ErrInvalidFilter.Error()
}

// Is hace que errors.Is(err, ErrInvalidFilter) reconozca este tipo.
func (e *InvalidFilterError) Is(target error) bool {
	return target == ErrInvalidFilter
}

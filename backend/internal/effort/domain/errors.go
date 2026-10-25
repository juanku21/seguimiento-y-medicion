package domain

import (
	"errors"
	"time"
)

// Errores de dominio del módulo de esfuerzo (data-model.md sección 7). Son centinelas que el servicio
// devuelve y que delivery compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el
// texto visible para la persona lo define delivery, no estos mensajes. ErrEntryNotFound unifica el
// registro inexistente, eliminado o de un proyecto del que quien pide no es integrante.
// ErrEffortReadOnly rechaza toda escritura sobre el esfuerzo de un proyecto finalizado.
// ErrSprintFinalized rechaza el alta cuando el último sprint de la historia está cerrado, vencido su
// plazo y con la historia completada; ErrEffortFinalized rechaza modificar o eliminar un registro de
// un sprint cerrado con el plazo vencido. ErrNotEntryAuthor reserva la modificación y la eliminación
// a quien registró el esfuerzo, ErrEntryDateInFuture rechaza una fecha posterior a hoy y
// ErrInvalidFilter rechaza un filtro de la consulta que no corresponde al proyecto.
var (
	ErrEntryNotFound     = errors.New("el registro de esfuerzo no existe")
	ErrEffortReadOnly    = errors.New("el proyecto está finalizado y su esfuerzo es de solo lectura")
	ErrSprintFinalized   = errors.New("el sprint de la historia de usuario está cerrado y su esfuerzo es definitivo")
	ErrEffortFinalized   = errors.New("el esfuerzo pertenece a un sprint cerrado y es definitivo")
	ErrNotEntryAuthor    = errors.New("solo quien registró el esfuerzo puede modificarlo o eliminarlo")
	ErrEntryDateInFuture = errors.New("la fecha del esfuerzo es posterior a hoy")
	ErrInvalidFilter     = errors.New("el filtro no corresponde al proyecto")
)

// InvalidFilterError rechaza un filtro de la consulta de esfuerzo e indica en Field el nombre del
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

// DailyCapExceededError rechaza un registro que llevaría a la persona por encima del tope diario de
// horas y lleva en AvailableHundredths las centésimas de hora que todavía puede registrar ese día.
// Delivery lo reconoce con errors.As.
type DailyCapExceededError struct {
	AvailableHundredths int
}

// Error devuelve un mensaje fijo; las horas disponibles se exponen en AvailableHundredths.
func (e *DailyCapExceededError) Error() string {
	return "el registro supera el tope diario de horas"
}

// EntryDateBeforeProjectStartError rechaza una fecha anterior al inicio del proyecto y lleva en
// MinDate la fecha mínima admitida. Delivery lo reconoce con errors.As.
type EntryDateBeforeProjectStartError struct {
	MinDate time.Time
}

// Error devuelve un mensaje fijo; la fecha mínima se expone en MinDate y no en el texto.
func (e *EntryDateBeforeProjectStartError) Error() string {
	return "la fecha del esfuerzo es anterior al inicio del proyecto"
}

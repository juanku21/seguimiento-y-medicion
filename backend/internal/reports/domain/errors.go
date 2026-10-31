package domain

import "errors"

// Errores del dominio de reportes; la capa de entrega los traduce a respuestas HTTP.
var (
	// ErrSprintNotFound indica que el sprint no existe o pertenece a otro proyecto.
	ErrSprintNotFound = errors.New("el sprint no existe o es de otro proyecto")
	// ErrSprintNotStarted indica que el sprint está planificado y no tiene nada que reportar.
	ErrSprintNotStarted = errors.New("el sprint todavía no arrancó")
)

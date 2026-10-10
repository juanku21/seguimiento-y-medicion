package domain

import "errors"

// Errores de dominio del módulo de autenticación. Son centinelas que el servicio devuelve y que
// delivery compara con errors.Is para traducirlos a la respuesta HTTP del contrato; el texto visible
// para la persona lo define delivery, no estos mensajes. ErrEmailUnavailable cubre el correo
// normalizado ya registrado, incluida la violación del índice único bajo concurrencia (FR-005,
// FR-007). ErrInvalidCredentials es único para correo inexistente y contraseña incorrecta, de modo
// que no revele qué correos existen (FR-014). ErrInvalidSession unifica la sesión ausente, vencida,
// cerrada o adulterada en una sola respuesta (FR-015).
var (
	ErrEmailUnavailable   = errors.New("correo electrónico no disponible")
	ErrInvalidCredentials = errors.New("credenciales inválidas")
	ErrInvalidSession     = errors.New("sesión inválida")
)

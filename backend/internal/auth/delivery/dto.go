package delivery

import "time"

// RegisterRequest es el cuerpo de POST /api/v1/auth/register (contracts/openapi.yaml). Las
// etiquetas validate y de longitud solo documentan el contrato en Swagger: la validación real la
// hacen las funciones de validation.go, que devuelven los mensajes por campo (FR-008).
type RegisterRequest struct {
	// Nombre completo, entre 2 y 100 caracteres tras recortar los espacios extremos (FR-002).
	FullName string `json:"fullName" validate:"required" minLength:"2" maxLength:"100" example:"Ana Pérez"`
	// Correo electrónico de hasta 254 caracteres; se normaliza a minúsculas sin espacios extremos (FR-003, FR-004).
	Email string `json:"email" validate:"required" format:"email" maxLength:"254" example:"ana@example.com"`
	// Entre 8 y 72 caracteres y como máximo 72 bytes, con al menos una letra y un número (FR-006).
	Password string `json:"password" validate:"required" format:"password" minLength:"8" maxLength:"72" example:"Proyecto2026"`
}

// UserProfile son los datos visibles de una cuenta. No incluye la contraseña ni ninguna
// representación derivada de ella (FR-011, SC-005).
type UserProfile struct {
	// Identificador propio, estable e inmutable de la cuenta (FR-027).
	ID string `json:"id" validate:"required" format:"uuid" example:"3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04"`
	// Nombre completo de la persona.
	FullName string `json:"fullName" validate:"required" example:"Ana Pérez"`
	// Correo normalizado (FR-004).
	Email string `json:"email" validate:"required" format:"email" example:"ana@example.com"`
}

// ErrorResponse es la respuesta de error general, con un mensaje en español dirigido a la persona.
type ErrorResponse struct {
	// Mensaje en español dirigido a la persona (Principio VIII).
	Error string `json:"error" validate:"required" example:"Se requiere iniciar sesión."`
}

// FieldError describe un campo inválido y qué hay que corregir. El mensaje nunca repite el valor
// enviado para la contraseña (FR-011).
type FieldError struct {
	// Nombre del campo inválido, tal como se envió en la solicitud.
	Field string `json:"field" validate:"required" example:"password"`
	// Qué hay que corregir y por qué.
	Message string `json:"message" validate:"required" example:"La contraseña debe tener entre 8 y 72 caracteres."`
}

// ValidationErrorResponse es el rechazo por entrada inválida, con al menos un detalle por campo
// (FR-008).
type ValidationErrorResponse struct {
	// Mensaje general del rechazo.
	Error string `json:"error" validate:"required" example:"Hay campos inválidos en la solicitud."`
	// Detalle de cada campo inválido; nunca está vacío.
	Fields []FieldError `json:"fields" validate:"required" minItems:"1"`
}

// LoginRequest es el cuerpo de POST /api/v1/auth/login (contracts/openapi.yaml). El correo se
// normaliza igual que en el registro antes de buscar la cuenta (FR-004, FR-012).
type LoginRequest struct {
	// Correo electrónico de la cuenta; se recortan los espacios extremos y se pasa a minúsculas.
	Email string `json:"email" validate:"required" format:"email" maxLength:"254" example:"ana@example.com"`
	// Contraseña de la cuenta; nunca se repite en ninguna respuesta (FR-011).
	Password string `json:"password" validate:"required" format:"password" example:"Proyecto2026"`
}

// LoginResponse es la sesión emitida por un inicio de sesión exitoso (FR-013).
type LoginResponse struct {
	// JWT de la sesión emitida. Se envía en `Authorization: Bearer <token>`.
	Token string `json:"token" validate:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIzZjFhOWM2NCJ9.firma"`
	// Momento de vencimiento de la sesión, 8 horas después de la emisión (FR-018).
	ExpiresAt time.Time `json:"expiresAt" validate:"required" format:"date-time" example:"2026-10-05T22:30:00Z"`
}

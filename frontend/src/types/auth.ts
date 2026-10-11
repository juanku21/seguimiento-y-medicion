// Tipos de las peticiones y respuestas del registro, según contracts/openapi.yaml (spec 001).

// Cuerpo de POST /api/v1/auth/register.
export type RegisterRequest = {
  fullName: string;
  email: string;
  password: string;
};

// Datos visibles de una cuenta; nunca incluye la contraseña (FR-011).
export type UserProfile = {
  id: string;
  fullName: string;
  email: string;
};

// Respuesta de error general, con el mensaje en español.
export type ErrorResponse = {
  error: string;
};

// Detalle de un campo inválido.
export type FieldError = {
  field: string;
  message: string;
};

// Rechazo por entrada inválida, con el detalle por campo (FR-008).
export type ValidationErrorResponse = {
  error: string;
  fields: FieldError[];
};

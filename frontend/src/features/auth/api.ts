import { isRecord, postJson, type ApiResult } from "@/lib/api-client";
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  UserProfile,
} from "@/types/auth";

// Reconoce el perfil que devuelve el registro exitoso (201), sin confiar en el cuerpo recibido.
function isUserProfile(value: unknown): value is UserProfile {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.fullName === "string" &&
    typeof value.email === "string"
  );
}

// Registra una cuenta con POST /api/v1/auth/register. No guarda nada en el navegador: el registro
// no emite sesión (FR-009).
export function registerUser(
  request: RegisterRequest,
): Promise<ApiResult<UserProfile>> {
  return postJson("/api/v1/auth/register", request, isUserProfile);
}

// Reconoce la sesión que devuelve el inicio de sesión exitoso (200), sin confiar en el cuerpo
// recibido.
function isLoginResponse(value: unknown): value is LoginResponse {
  return (
    isRecord(value) &&
    typeof value.token === "string" &&
    typeof value.expiresAt === "string"
  );
}

// Inicia sesión con POST /api/v1/auth/login. Guardar el token es responsabilidad de quien llama,
// a través del contexto de sesión.
export function loginUser(
  request: LoginRequest,
): Promise<ApiResult<LoginResponse>> {
  return postJson("/api/v1/auth/login", request, isLoginResponse);
}

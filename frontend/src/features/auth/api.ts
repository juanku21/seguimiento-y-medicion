import {
  getJson,
  isRecord,
  postEmpty,
  postJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  UserProfile,
} from "@/types/auth";

// Reconoce el perfil que devuelven el registro exitoso (201) y la consulta del propio perfil (200),
// sin confiar en el cuerpo recibido.
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

// Consulta el perfil del titular de la sesión con GET /api/v1/users/me. No envía ningún
// identificador de cuenta: el backend lo resuelve desde el token (FR-022, FR-023). Ante un 401 el
// cliente HTTP descarta la sesión de la pestaña (FR-016).
export function getOwnProfile(): Promise<ApiResult<UserProfile>> {
  return getJson("/api/v1/users/me", isUserProfile);
}

// Cierra la sesión de la pestaña con POST /api/v1/auth/logout. Solo envía el token propio en la
// cabecera, sin identificadores de cuenta ni de otras sesiones, así que no afecta a otros
// dispositivos (FR-017). Ante un 401 el cliente HTTP descarta la sesión de la pestaña (FR-016).
export function logoutUser(): Promise<ApiResult<undefined>> {
  return postEmpty("/api/v1/auth/logout");
}

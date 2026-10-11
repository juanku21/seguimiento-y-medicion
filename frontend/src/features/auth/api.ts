import { isRecord, postJson, type ApiResult } from "@/lib/api-client";
import type { RegisterRequest, UserProfile } from "@/types/auth";

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

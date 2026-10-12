import { API_BASE_URL } from "@/config/env";
import { readToken, rejectSession } from "@/lib/session-storage";
import type { FieldError } from "@/types/auth";

// Error de una llamada a la API ya traducido a un mensaje en español. fields trae el detalle por
// campo cuando el backend lo informa (ValidationErrorResponse) y queda vacío en cualquier otro caso.
export type ApiError = {
  status: number;
  message: string;
  fields: FieldError[];
};

// Resultado de una llamada: los datos de la respuesta exitosa o el error, sin lanzar excepciones.
export type ApiResult<T> =
  { ok: true; data: T } | { ok: false; error: ApiError };

const NETWORK_ERROR_MESSAGE =
  "No se pudo conectar con el servidor. Intentá de nuevo más tarde.";
const UNEXPECTED_ERROR_MESSAGE =
  "El servidor devolvió una respuesta inesperada. Intentá de nuevo más tarde.";

// Indica si un valor desconocido es un objeto con propiedades, para poder leerlas sin any.
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

// Arma un error sin detalle por campo, para los casos en que se muestra un mensaje genérico.
function genericError(status: number, message: string): ApiError {
  return { status, message, fields: [] };
}

// Reconoce un elemento del detalle por campo de ValidationErrorResponse.
function isFieldError(value: unknown): value is FieldError {
  return (
    isRecord(value) &&
    typeof value.field === "string" &&
    typeof value.message === "string"
  );
}

// Lee el cuerpo como JSON; si está vacío o no es JSON válido devuelve undefined en lugar de fallar.
async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

// Traduce un cuerpo de error a ApiError. Distingue las dos formas del contrato: con "fields" es un
// ValidationErrorResponse y sin él un ErrorResponse; si no coincide con ninguna, usa un mensaje
// genérico para no mostrar nada técnico.
function toApiError(status: number, body: unknown): ApiError {
  if (!isRecord(body) || typeof body.error !== "string") {
    return genericError(status, UNEXPECTED_ERROR_MESSAGE);
  }
  const fields = Array.isArray(body.fields)
    ? body.fields.filter(isFieldError)
    : [];
  return { status, message: body.error, fields };
}

// Envía body como JSON con POST a la ruta indicada de la API usando fetch nativo. Los fallos de red,
// las respuestas no exitosas y los cuerpos que no cumplen isData se devuelven como error.
export async function postJson<T>(
  path: string,
  body: unknown,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    return { ok: false, error: genericError(0, NETWORK_ERROR_MESSAGE) };
  }

  const data = await readJson(response);
  if (!response.ok) {
    return { ok: false, error: toApiError(response.status, data) };
  }
  if (!isData(data)) {
    return {
      ok: false,
      error: genericError(response.status, UNEXPECTED_ERROR_MESSAGE),
    };
  }
  return { ok: true, data };
}

// Envía un GET autenticado a la ruta indicada de la API con el token de la pestaña en la cabecera
// Authorization (research.md, sección 9). Ante un 401 borra la sesión y deja la marca del rechazo,
// para que la guardia lleve a /login con el aviso de volver a iniciar sesión (FR-016).
export async function getJson<T>(
  path: string,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  const token = readToken();
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: "GET",
      headers: token === null ? {} : { Authorization: `Bearer ${token}` },
    });
  } catch {
    return { ok: false, error: genericError(0, NETWORK_ERROR_MESSAGE) };
  }

  const data = await readJson(response);
  if (response.status === 401) {
    rejectSession();
  }
  if (!response.ok) {
    return { ok: false, error: toApiError(response.status, data) };
  }
  if (!isData(data)) {
    return {
      ok: false,
      error: genericError(response.status, UNEXPECTED_ERROR_MESSAGE),
    };
  }
  return { ok: true, data };
}

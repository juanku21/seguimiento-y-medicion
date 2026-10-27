import { API_BASE_URL } from "@/config/env";
import { readToken, rejectSession } from "@/lib/session-storage";
import type { FieldError } from "@/types/auth";

// Error de una llamada a la API ya traducido a un mensaje en español. fields trae el detalle por
// campo cuando el backend lo informa (ValidationErrorResponse) y queda vacío en cualquier otro caso.
// reasons trae los mensajes de los motivos, en el orden recibido, solo cuando el cuerpo los informa
// (DeletionBlockedResponse de la spec 003). stories trae el motivo de cada historia rechazada solo
// cuando el cuerpo los informa (StoriesNotAdmittedResponse de la spec 004). sessionId trae la sesión
// de estimación en curso solo cuando el cuerpo la informa (OpenSessionExistsResponse de la spec 005).
// code y currentStatus traen el motivo del conflicto y el estado vigente del recurso solo cuando el
// cuerpo los informa (DefectConflictResponse de la spec 007).
export type ApiError = {
  status: number;
  message: string;
  fields: FieldError[];
  reasons?: string[];
  stories?: StoryProblem[];
  sessionId?: string;
  code?: string;
  currentStatus?: string;
};

// Motivo por el que el backend rechazó una historia concreta, identificada por su storyId.
export type StoryProblem = {
  storyId: string;
  message: string;
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

// Reconoce un motivo de rechazo con su mensaje, como DeletionBlockedReason de la spec 003.
function isReason(value: unknown): value is { message: string } {
  return isRecord(value) && typeof value.message === "string";
}

// Reconoce el motivo de una historia rechazada, como AdmissionProblem de la spec 004.
function isStoryProblem(value: unknown): value is StoryProblem {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
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

// Traduce un cuerpo de error a ApiError. Distingue las formas del contrato: con "fields" es un
// ValidationErrorResponse, con "reasons" un DeletionBlockedResponse, con "stories" un
// StoriesNotAdmittedResponse, con "sessionId" un OpenSessionExistsResponse, con "code" y
// "currentStatus" un DefectConflictResponse y sin ninguno un ErrorResponse; si no coincide con
// ninguna, usa un mensaje genérico para no mostrar nada técnico.
function toApiError(status: number, body: unknown): ApiError {
  if (!isRecord(body) || typeof body.error !== "string") {
    return genericError(status, UNEXPECTED_ERROR_MESSAGE);
  }
  const fields = Array.isArray(body.fields)
    ? body.fields.filter(isFieldError)
    : [];
  const error: ApiError = { status, message: body.error, fields };
  if (Array.isArray(body.reasons)) {
    error.reasons = body.reasons
      .filter(isReason)
      .map((reason) => reason.message);
  }
  if (Array.isArray(body.stories)) {
    error.stories = body.stories
      .filter(isStoryProblem)
      .map(({ storyId, message }) => ({ storyId, message }));
  }
  if (typeof body.sessionId === "string") {
    error.sessionId = body.sessionId;
  }
  if (typeof body.code === "string") {
    error.code = body.code;
  }
  if (typeof body.currentStatus === "string") {
    error.currentStatus = body.currentStatus;
  }
  return error;
}

// Envía la petición a la ruta indicada de la API usando fetch nativo y traduce la respuesta a
// ApiResult. Los fallos de red, las respuestas no exitosas y los cuerpos que no cumplen isData se
// devuelven como error.
async function request<T>(
  path: string,
  init: RequestInit,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, init);
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

// Envía body como JSON con POST a la ruta indicada de la API. No toca la sesión ante un 401,
// porque en el inicio de sesión ese código significa credenciales inválidas.
export function postJson<T>(
  path: string,
  body: unknown,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return request(
    path,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
    isData,
  );
}

// Arma la cabecera Authorization con el token de la pestaña (research.md, sección 9), o ninguna
// cabecera si no hay sesión.
function authorizationHeaders(): Record<string, string> {
  const token = readToken();
  return token === null ? {} : { Authorization: `Bearer ${token}` };
}

// Envía una petición autenticada con el token de la pestaña en la cabecera Authorization; si recibe
// body, lo envía como JSON. Ante un 401 borra la sesión y deja
// la marca del rechazo, para que la guardia lleve a /login con el aviso de volver a iniciar sesión
// (FR-016).
async function authorizedRequest<T>(
  path: string,
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
  isData: (value: unknown) => value is T,
  body?: unknown,
): Promise<ApiResult<T>> {
  const headers = authorizationHeaders();
  const init: RequestInit = { method, headers };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  const result = await request(path, init, isData);
  if (!result.ok && result.error.status === 401) {
    rejectSession();
  }
  return result;
}

// Envía un GET autenticado a la ruta indicada de la API.
export function getJson<T>(
  path: string,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return authorizedRequest(path, "GET", isData);
}

// Reconoce una respuesta sin cuerpo, como el 204 del cierre de sesión o de una baja.
function isEmpty(value: unknown): value is undefined {
  return value === undefined;
}

// Envía un POST autenticado y sin cuerpo a la ruta indicada de la API, para las acciones cuya
// respuesta exitosa no trae contenido.
export function postEmpty(path: string): Promise<ApiResult<undefined>> {
  return authorizedRequest(path, "POST", isEmpty);
}

// Envía body como JSON con un POST autenticado a la ruta indicada de la API.
export function postAuthorizedJson<T>(
  path: string,
  body: unknown,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return authorizedRequest(path, "POST", isData, body);
}

// Envía body como JSON con un PUT autenticado a la ruta indicada de la API, para las
// modificaciones que reemplazan los datos editables de un recurso.
export function putAuthorizedJson<T>(
  path: string,
  body: unknown,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return authorizedRequest(path, "PUT", isData, body);
}

// Envía body como JSON con un PATCH autenticado a la ruta indicada de la API, para los cambios
// parciales de un recurso, como el avance de estado del proyecto.
export function patchAuthorizedJson<T>(
  path: string,
  body: unknown,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return authorizedRequest(path, "PATCH", isData, body);
}

// Envía un DELETE autenticado a la ruta indicada de la API, para las bajas cuya respuesta exitosa
// (204) no trae contenido.
export function deleteEmpty(path: string): Promise<ApiResult<undefined>> {
  return authorizedRequest(path, "DELETE", isEmpty);
}

// Envía un DELETE autenticado a la ruta indicada de la API, para las bajas cuya respuesta exitosa
// trae el recurso actualizado, como la baja de historias de un sprint.
export function deleteAuthorizedJson<T>(
  path: string,
  isData: (value: unknown) => value is T,
): Promise<ApiResult<T>> {
  return authorizedRequest(path, "DELETE", isData);
}

// Abre con un GET autenticado un canal text/event-stream de la API y devuelve su cuerpo sin leerlo,
// para que lo consuma el lector de eventos. signal permite abortar la petición. Las respuestas no
// exitosas se traducen a ApiError como en las demás llamadas y, ante un 401, se descarta la sesión
// (FR-016 de la 001); un fallo de red o una petición abortada llegan como error de red.
export async function openAuthorizedStream(
  path: string,
  signal: AbortSignal,
): Promise<ApiResult<ReadableStream<Uint8Array>>> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: "GET",
      headers: { ...authorizationHeaders(), Accept: "text/event-stream" },
      signal,
    });
  } catch {
    return { ok: false, error: genericError(0, NETWORK_ERROR_MESSAGE) };
  }

  if (!response.ok) {
    const error = toApiError(response.status, await readJson(response));
    if (error.status === 401) {
      rejectSession();
    }
    return { ok: false, error };
  }
  if (response.body === null) {
    return {
      ok: false,
      error: genericError(response.status, UNEXPECTED_ERROR_MESSAGE),
    };
  }
  return { ok: true, data: response.body };
}

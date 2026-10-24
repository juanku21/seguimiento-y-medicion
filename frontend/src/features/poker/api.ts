import {
  isRecord,
  openAuthorizedStream,
  postAuthorizedJson,
  putAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type { Card, EstimationSession } from "@/types/poker";

// Reconoce la sesión de estimación (EstimationSession) que devuelven la apertura (201) y cada evento
// state del canal, sin confiar en el cuerpo recibido. El cierre, el valor acordado, el motivo de
// cancelación, el voto propio y el momento en que se libera el rol de facilitador pueden llegar en
// null.
export function isEstimationSession(
  value: unknown,
): value is EstimationSession {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.version === "number" &&
    typeof value.status === "string" &&
    isRecord(value.story) &&
    typeof value.story.id === "string" &&
    typeof value.story.title === "string" &&
    isRecord(value.facilitator) &&
    typeof value.facilitator.id === "string" &&
    isRecord(value.openedBy) &&
    typeof value.openedAt === "string" &&
    Array.isArray(value.cards) &&
    Array.isArray(value.participants) &&
    Array.isArray(value.rounds)
  );
}

// Arma la ruta de una sesión de estimación codificando el identificador, para que un valor con "/"
// o "../" no altere la ruta pedida al backend.
function sessionPath(sessionId: string): string {
  return `/api/v1/estimation-sessions/${encodeURIComponent(sessionId)}`;
}

// Abre una sesión de Planning Poker sobre la historia con POST
// /api/v1/stories/{storyId}/estimation-sessions, sin cuerpo y con la sesión de la pestaña (005/US1).
// Las reglas de la apertura las resuelve el backend: el 409 de sesión en curso llega con el
// identificador de esa sesión en sessionId para unirse (FR-006), los de historia Completada y
// proyecto Finalizado solo con su mensaje, y ante un 401 el cliente HTTP descarta la sesión.
export function openEstimationSession(
  storyId: string,
): Promise<ApiResult<EstimationSession>> {
  return postAuthorizedJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/estimation-sessions`,
    undefined,
    isEstimationSession,
  );
}

// Abre el canal GET /api/v1/estimation-sessions/{sessionId}/events, que une a quien pide como
// participante conectado y emite la sesión ante cada cambio (FR-011, FR-073). Devuelve el cuerpo
// text/event-stream sin leer; signal permite abortar la petición al salir de la sala.
export function streamEstimationSession(
  sessionId: string,
  signal: AbortSignal,
): Promise<ApiResult<ReadableStream<Uint8Array>>> {
  return openAuthorizedStream(`${sessionPath(sessionId)}/events`, signal);
}

// Emite o cambia el voto propio en la ronda vigente con PUT
// /api/v1/estimation-sessions/{sessionId}/vote y cuerpo {"card": "<carta>"} (005/US2). Devuelve la
// sesión vista por quien votó (200). Las reglas las resuelve el backend: la carta no admitida llega
// como 400 con el detalle en fields, quien no participa como 403, la sesión inexistente como 404 y
// la sesión cerrada o la ronda ya revelada como 409; ante un 401 el cliente HTTP descarta la sesión.
export function castEstimationVote(
  sessionId: string,
  card: Card,
): Promise<ApiResult<EstimationSession>> {
  return putAuthorizedJson(
    `${sessionPath(sessionId)}/vote`,
    { card },
    isEstimationSession,
  );
}

// Revela la ronda vigente con POST /api/v1/estimation-sessions/{sessionId}/reveal, sin cuerpo
// (005/US3). Devuelve la sesión vista por el facilitador (200). Las reglas las resuelve el backend:
// quien no es el facilitador recibe 403, la sesión inexistente 404 y la ronda sin votos, ya
// revelada o la sesión cerrada 409; ante un 401 el cliente HTTP descarta la sesión.
export function revealEstimationRound(
  sessionId: string,
): Promise<ApiResult<EstimationSession>> {
  return postAuthorizedJson(
    `${sessionPath(sessionId)}/reveal`,
    undefined,
    isEstimationSession,
  );
}

// Inicia una ronda nueva con POST /api/v1/estimation-sessions/{sessionId}/rounds, sin cuerpo
// (005/US4). Devuelve la sesión vista por el facilitador con la ronda siguiente en votación (201).
// Las reglas las resuelve el backend: quien no es el facilitador recibe 403, la sesión inexistente
// 404 y la ronda vigente todavía en votación o la sesión cerrada 409; ante un 401 el cliente HTTP
// descarta la sesión.
export function startEstimationRound(
  sessionId: string,
): Promise<ApiResult<EstimationSession>> {
  return postAuthorizedJson(
    `${sessionPath(sessionId)}/rounds`,
    undefined,
    isEstimationSession,
  );
}

import {
  getJson,
  isRecord,
  postAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  EffortEntry,
  EffortEntryInput,
  MemberEffort,
  StoryEffort,
} from "@/types/effort";

// Reconoce el registro de esfuerzo (EffortEntry) que devuelve el alta exitosa (201), sin confiar en
// el cuerpo recibido. El sprint puede llegar en null cuando la historia no está en ninguno.
function isEffortEntry(value: unknown): value is EffortEntry {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.entryDate === "string" &&
    typeof value.activity === "string" &&
    typeof value.hours === "number" &&
    isRecord(value.author) &&
    typeof value.author.id === "string" &&
    typeof value.author.fullName === "string" &&
    isRecord(value.story) &&
    typeof value.story.id === "string" &&
    (value.sprint === null || isRecord(value.sprint)) &&
    typeof value.definitive === "boolean" &&
    typeof value.createdAt === "string" &&
    typeof value.updatedAt === "string"
  );
}

// Arma la ruta de los registros de esfuerzo de una historia codificando el identificador, para que
// un valor con "/" o "../" no altere la ruta pedida al backend.
function storyEffortEntriesPath(storyId: string): string {
  return `/api/v1/stories/${encodeURIComponent(storyId)}/effort-entries`;
}

// Registra esfuerzo sobre una historia con POST /api/v1/stories/{storyId}/effort-entries, usando la
// sesión de la pestaña (006/US1). Solo viajan fecha, actividad y horas: el autor lo toma el backend
// de la sesión (FR-002). Las reglas que dependen de datos las resuelve el backend: el 400 llega con
// el detalle por campo, el 404 y el 409 (tope diario, sprint definitivo, proyecto de solo lectura)
// con su mensaje, y ante un 401 el cliente HTTP descarta la sesión.
export function createEffortEntry(
  storyId: string,
  input: EffortEntryInput,
): Promise<ApiResult<EffortEntry>> {
  const body: EffortEntryInput = {
    entryDate: input.entryDate,
    activity: input.activity,
    hours: input.hours,
  };
  return postAuthorizedJson(
    storyEffortEntriesPath(storyId),
    body,
    isEffortEntry,
  );
}

// Reconoce una línea del desglose por persona (MemberEffort) con su nombre y sus horas.
function isMemberEffort(value: unknown): value is MemberEffort {
  return (
    isRecord(value) &&
    isRecord(value.user) &&
    typeof value.user.id === "string" &&
    typeof value.user.fullName === "string" &&
    typeof value.hours === "number"
  );
}

// Reconoce el esfuerzo de una historia (StoryEffort) que devuelve la consulta exitosa (200), sin
// confiar en el cuerpo recibido: total numérico y listas cuyos elementos cumplen su forma.
function isStoryEffort(value: unknown): value is StoryEffort {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.totalHours === "number" &&
    Array.isArray(value.byMember) &&
    value.byMember.every(isMemberEffort) &&
    Array.isArray(value.entries) &&
    value.entries.every(isEffortEntry)
  );
}

// Consulta el esfuerzo de una historia con GET /api/v1/stories/{storyId}/effort, usando la sesión de
// la pestaña (006/US2): total, desglose por persona y registros ya ordenados por el backend
// (FR-030 a FR-035). Una historia sin registros llega con total 0 y listas vacías; el 404 llega con
// el mensaje del backend y ante un 401 el cliente HTTP descarta la sesión.
export function getStoryEffort(
  storyId: string,
): Promise<ApiResult<StoryEffort>> {
  return getJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/effort`,
    isStoryEffort,
  );
}

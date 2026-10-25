import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import type { EffortEntry, EffortEntryInput } from "@/types/effort";

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

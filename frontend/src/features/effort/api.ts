import {
  deleteEmpty,
  getJson,
  isRecord,
  patchAuthorizedJson,
  postAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  EffortComparisonRow,
  EffortEntry,
  EffortEntryInput,
  EffortEntryPatch,
  MemberEffort,
  ProjectEffort,
  ProjectEffortComparison,
  ProjectEffortFilter,
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

// Arma la ruta de un registro de esfuerzo codificando el identificador, para que un valor con "/" o
// "../" no altere la ruta pedida al backend.
function effortEntryPath(entryId: string): string {
  return `/api/v1/effort-entries/${encodeURIComponent(entryId)}`;
}

// Corrige un registro propio con PATCH /api/v1/effort-entries/{entryId}, usando la sesión de la
// pestaña (006/US3). Solo viajan los campos que cambiaron, con las horas como número; autor,
// historia y sprint nunca se envían (FR-019). El 403 (registro ajeno), el 404 y los 409 (tope
// diario, registro definitivo, proyecto de solo lectura) llegan con el mensaje del backend, y ante
// un 401 el cliente HTTP descarta la sesión.
export function updateEffortEntry(
  entryId: string,
  patch: EffortEntryPatch,
): Promise<ApiResult<EffortEntry>> {
  return patchAuthorizedJson(effortEntryPath(entryId), patch, isEffortEntry);
}

// Elimina un registro propio con DELETE /api/v1/effort-entries/{entryId}, usando la sesión de la
// pestaña (006/US3). La respuesta exitosa (204) no trae contenido; los rechazos llegan con el
// mensaje del backend como en la corrección.
export function deleteEffortEntry(
  entryId: string,
): Promise<ApiResult<undefined>> {
  return deleteEmpty(effortEntryPath(entryId));
}

// Reconoce el esfuerzo del proyecto filtrado (ProjectEffort) que devuelve la consulta exitosa (200),
// sin confiar en el cuerpo recibido: registros con su forma, total numérico y motivo del vacío.
function isProjectEffort(value: unknown): value is ProjectEffort {
  return (
    isRecord(value) &&
    Array.isArray(value.entries) &&
    value.entries.every(isEffortEntry) &&
    typeof value.totalHours === "number" &&
    (value.emptyReason === null ||
      value.emptyReason === "no_effort" ||
      value.emptyReason === "no_matches")
  );
}

// Consulta el esfuerzo del proyecto con GET /api/v1/projects/{projectId}/effort-entries, usando la
// sesión de la pestaña (006/US4). Cada filtro elegido viaja como parámetro y los que valen "" no
// viajan; sin filtros no se envía consulta. El filtrado, el orden, el total y el motivo del vacío
// los resuelve el backend (FR-036 a FR-045): un filtro inválido llega como 400 con el detalle por
// campo y ante un 401 el cliente HTTP descarta la sesión.
export function listProjectEffort(
  projectId: string,
  filter: ProjectEffortFilter,
): Promise<ApiResult<ProjectEffort>> {
  const query = new URLSearchParams();
  (["userId", "storyId", "sprintId", "from", "to"] as const).forEach((key) => {
    if (filter[key] !== "") {
      query.append(key, filter[key]);
    }
  });
  const encoded = query.toString();
  const search = encoded === "" ? "" : `?${encoded}`;
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/effort-entries${search}`,
    isProjectEffort,
  );
}

// Reconoce un número o null, la forma de los valores que pueden ser "no calculable".
function isNumberOrNull(value: unknown): value is number | null {
  return value === null || typeof value === "number";
}

// Reconoce una fila de la comparación entre estimado y real (EffortComparisonRow), sin confiar en el
// cuerpo recibido: las horas reales siempre son un número y el resto admite null (FR-050, FR-051).
function isEffortComparisonRow(value: unknown): value is EffortComparisonRow {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.title === "string" &&
    isNumberOrNull(value.storyPoints) &&
    isNumberOrNull(value.estimatedHours) &&
    typeof value.actualHours === "number" &&
    isNumberOrNull(value.deviationHours) &&
    isNumberOrNull(value.deviationPercent)
  );
}

// Reconoce la comparación del proyecto (ProjectEffortComparison): el factor vigente y una fila por
// historia con su forma.
function isProjectEffortComparison(
  value: unknown,
): value is ProjectEffortComparison {
  return (
    isRecord(value) &&
    typeof value.hoursPerStoryPoint === "number" &&
    Array.isArray(value.stories) &&
    value.stories.every(isEffortComparisonRow)
  );
}

// Consulta la comparación entre estimado y real de una historia con
// GET /api/v1/stories/{storyId}/effort-comparison, usando la sesión de la pestaña (006/US5). El
// cálculo lo resuelve el backend con el factor vigente (FR-046 a FR-054); el 404 llega con el
// mensaje del backend y ante un 401 el cliente HTTP descarta la sesión.
export function getStoryEffortComparison(
  storyId: string,
): Promise<ApiResult<EffortComparisonRow>> {
  return getJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/effort-comparison`,
    isEffortComparisonRow,
  );
}

// Consulta la comparación entre estimado y real de todas las historias del proyecto con
// GET /api/v1/projects/{projectId}/effort-comparison, usando la sesión de la pestaña (006/US5). Las
// historias llegan en el orden del backlog, incluidas las que no tienen registros (FR-052).
export function getProjectEffortComparison(
  projectId: string,
): Promise<ApiResult<ProjectEffortComparison>> {
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/effort-comparison`,
    isProjectEffortComparison,
  );
}

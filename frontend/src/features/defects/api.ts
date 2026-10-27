import {
  getJson,
  isRecord,
  postAuthorizedJson,
  putAuthorizedJson,
  type ApiError,
  type ApiResult,
} from "@/lib/api-client";
import { listSprints } from "@/features/sprints/api";
import type {
  Defect,
  DefectFilterValues,
  DefectInput,
  DefectPatch,
  DefectStatus,
  DefectStatusChange,
  ProjectDefects,
  StoryDefects,
} from "@/types/defect";
import type { SprintListResponse } from "@/types/sprint";

// Reconoce el defecto (Defect) que devuelven el alta (201), la consulta y el cambio de estado (200),
// sin confiar en el cuerpo recibido. El sprint de resolución llega en null mientras el defecto no está resuelto.
function isDefect(value: unknown): value is Defect {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.projectId === "string" &&
    typeof value.description === "string" &&
    typeof value.severity === "string" &&
    typeof value.status === "string" &&
    isRecord(value.story) &&
    typeof value.story.id === "string" &&
    isRecord(value.detectedSprint) &&
    typeof value.detectedSprint.id === "string" &&
    (value.resolvedSprint === null || isRecord(value.resolvedSprint)) &&
    isRecord(value.createdBy) &&
    typeof value.createdBy.id === "string" &&
    typeof value.createdAt === "string" &&
    typeof value.updatedAt === "string"
  );
}

// Registra un defecto con POST /api/v1/projects/{projectId}/defects, usando la sesión de la pestaña
// (007/US1). Solo viajan descripción, severidad, historia y sprint de detección: el autor lo toma el
// backend de la sesión (FR-056). Las reglas que dependen de datos las resuelve el backend: el 400
// llega con el detalle por campo, el 404 y el 409 (proyecto de solo lectura, sin sprint de
// detección) con su mensaje y ante un 401 el cliente HTTP descarta la sesión.
export function createDefect(
  projectId: string,
  input: DefectInput,
): Promise<ApiResult<Defect>> {
  const body: DefectInput = {
    description: input.description,
    severity: input.severity,
    storyId: input.storyId,
    detectedSprintId: input.detectedSprintId,
  };
  return postAuthorizedJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/defects`,
    body,
    isDefect,
  );
}

// Lista los sprints donde puede detectarse un defecto (Activos y Cerrados) con la consulta de la
// spec 004, GET /api/v1/projects/{projectId}/sprints?status=active&status=closed. Un sprint
// Planificado nunca se ofrece (FR-004).
export function listDetectionSprints(
  projectId: string,
): Promise<ApiResult<SprintListResponse>> {
  return listSprints(projectId, ["active", "closed"]);
}

// Arma la ruta de un defecto codificando el identificador, para que un valor con "/" o "../" no
// altere la ruta pedida al backend.
function defectPath(defectId: string): string {
  return `/api/v1/defects/${encodeURIComponent(defectId)}`;
}

// Consulta el detalle de un defecto con GET /api/v1/defects/{defectId}, usando la sesión de la
// pestaña (007/US2). El defecto inexistente o ajeno llega como 404 con su mensaje y ante un 401 el
// cliente HTTP descarta la sesión.
export function getDefect(defectId: string): Promise<ApiResult<Defect>> {
  return getJson(defectPath(defectId), isDefect);
}

// Cambia el estado de un defecto con PUT /api/v1/defects/{defectId}/status (007/US2). Solo viaja el
// estado pedido y, si se envía, el sprint de resolución: quién actúa y cuándo los toma el backend.
// Las reglas las resuelve el backend: el 400 llega con el detalle en resolvedSprintId, el 404 con su
// mensaje y el 409 con su code (invalid_transition trae además el estado vigente).
export function changeDefectStatus(
  defectId: string,
  change: DefectStatusChange,
): Promise<ApiResult<Defect>> {
  const body: DefectStatusChange = { status: change.status };
  if (change.resolvedSprintId !== undefined) {
    body.resolvedSprintId = change.resolvedSprintId;
  }
  return putAuthorizedJson(`${defectPath(defectId)}/status`, body, isDefect);
}

// Reconoce el listado de defectos (ProjectDefects y StoryDefects) que devuelve el 200: la lista
// siempre llega como arreglo y emptyReason es null cuando hay resultados.
function isDefectList(
  value: unknown,
): value is { defects: Defect[]; emptyReason: string | null } {
  return (
    isRecord(value) &&
    Array.isArray(value.defects) &&
    value.defects.every(isDefect) &&
    (value.emptyReason === null || typeof value.emptyReason === "string")
  );
}

// Reconoce el listado de defectos de un proyecto, cuyo motivo de vacío puede ser no_defects o
// no_matches.
function isProjectDefects(value: unknown): value is ProjectDefects {
  return (
    isDefectList(value) &&
    (value.emptyReason === null ||
      value.emptyReason === "no_defects" ||
      value.emptyReason === "no_matches")
  );
}

// Reconoce el listado de defectos de una historia, cuyo único motivo de vacío es no_defects.
function isStoryDefects(value: unknown): value is StoryDefects {
  return (
    isDefectList(value) &&
    (value.emptyReason === null || value.emptyReason === "no_defects")
  );
}

// Lista los defectos del proyecto con GET /api/v1/projects/{projectId}/defects, usando la sesión de
// la pestaña (007/US3). Cada valor elegido de estado o severidad viaja como parámetro repetido y
// los filtros de historia y sprint solo viajan si tienen valor. El orden, el filtrado y el motivo
// del vacío los resuelve el backend (FR-040 a FR-045).
export function listProjectDefects(
  projectId: string,
  filters: DefectFilterValues,
): Promise<ApiResult<ProjectDefects>> {
  const query = new URLSearchParams();
  filters.status.forEach((status) => query.append("status", status));
  filters.severity.forEach((severity) => query.append("severity", severity));
  if (filters.storyId !== "") {
    query.append("storyId", filters.storyId);
  }
  if (filters.sprintId !== "") {
    query.append("sprintId", filters.sprintId);
  }
  const encoded = query.toString();
  const search = encoded === "" ? "" : `?${encoded}`;
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/defects${search}`,
    isProjectDefects,
  );
}

// Lista los defectos de una historia con GET /api/v1/stories/{storyId}/defects, usando la sesión de
// la pestaña (007/US3). La historia inexistente o ajena llega como 404 con su mensaje.
export function listStoryDefects(
  storyId: string,
): Promise<ApiResult<StoryDefects>> {
  return getJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/defects`,
    isStoryDefects,
  );
}

const DEFECT_STATUSES: DefectStatus[] = [
  "open",
  "in_progress",
  "resolved",
  "discarded",
];

// Devuelve el estado vigente informado por un 409 invalid_transition (FR-018, RN-24), para refrescar
// el defecto sin otra consulta, o null si el error es de otro tipo o el estado no es uno conocido.
export function invalidTransitionStatus(error: ApiError): DefectStatus | null {
  if (error.status !== 409 || error.code !== "invalid_transition") {
    return null;
  }
  return (
    DEFECT_STATUSES.find((status) => status === error.currentStatus) ?? null
  );
}

// Modifica los datos de un defecto con PATCH /api/v1/defects/{defectId} (007/US4). Solo viajan los
// campos que cambian.
export function updateDefect(
  defectId: string,
  patch: DefectPatch,
): Promise<ApiResult<Defect>> {
  void defectId;
  void patch;
  throw new Error("not implemented");
}

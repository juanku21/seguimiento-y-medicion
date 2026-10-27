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

// Firma mínima (T040): lista los defectos del proyecto con los filtros, GET
// /api/v1/projects/{projectId}/defects. La implementa T045.
export function listProjectDefects(
  projectId: string,
  filters: DefectFilterValues,
): Promise<ApiResult<ProjectDefects>> {
  void projectId;
  void filters;
  throw new Error("not implemented");
}

// Firma mínima (T040): lista los defectos de una historia, GET /api/v1/stories/{storyId}/defects.
// La implementa T045.
export function listStoryDefects(
  storyId: string,
): Promise<ApiResult<StoryDefects>> {
  void storyId;
  throw new Error("not implemented");
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

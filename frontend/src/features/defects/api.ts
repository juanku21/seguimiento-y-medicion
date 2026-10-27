import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import { listSprints } from "@/features/sprints/api";
import type { Defect, DefectInput } from "@/types/defect";
import type { SprintListResponse } from "@/types/sprint";

// Reconoce el defecto (Defect) que devuelve el alta exitosa (201), sin confiar en el cuerpo
// recibido. El sprint de resolución llega en null mientras el defecto no está resuelto.
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

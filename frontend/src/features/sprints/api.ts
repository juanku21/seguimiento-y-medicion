import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import type { SprintDetail, SprintInput } from "@/types/sprint";

// Reconoce el detalle del sprint (SprintDetail) que devuelve la creación exitosa (201), sin confiar
// en el cuerpo recibido. Los campos de inicio, cierre, resultados y cronograma pueden llegar en null.
function isSprintDetail(value: unknown): value is SprintDetail {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.projectId === "string" &&
    typeof value.name === "string" &&
    typeof value.goal === "string" &&
    typeof value.status === "string" &&
    typeof value.startDate === "string" &&
    typeof value.endDate === "string" &&
    isRecord(value.createdBy) &&
    typeof value.createdBy.id === "string" &&
    typeof value.createdBy.fullName === "string" &&
    typeof value.createdAt === "string" &&
    typeof value.updatedAt === "string" &&
    Array.isArray(value.stories)
  );
}

// Arma la ruta de los sprints de un proyecto codificando el identificador, para que un valor con
// "/" o "../" no altere la ruta pedida al backend.
function projectSprintsPath(projectId: string): string {
  return `/api/v1/projects/${encodeURIComponent(projectId)}/sprints`;
}

// Crea un sprint Planificado con POST /api/v1/projects/{projectId}/sprints, usando la sesión de la
// pestaña (004/US1). Solo viajan nombre, Sprint Goal y fechas: el autor lo toma el backend de la
// sesión (FR-060). Las reglas las resuelve el backend: el 400 llega con el detalle por campo, el 404
// y el 409 con su mensaje (el de superposición ya nombra el sprint en conflicto) y ante un 401 el
// cliente HTTP descarta la sesión.
export function createSprint(
  projectId: string,
  input: SprintInput,
): Promise<ApiResult<SprintDetail>> {
  const body: SprintInput = {
    name: input.name,
    goal: input.goal,
    startDate: input.startDate,
    endDate: input.endDate,
  };
  return postAuthorizedJson(
    projectSprintsPath(projectId),
    body,
    isSprintDetail,
  );
}

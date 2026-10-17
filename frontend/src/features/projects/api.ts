import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import type { ProjectDetail, ProjectInput } from "@/types/project";

// Reconoce el estado completo del proyecto que devuelve la creación exitosa (201), sin confiar en
// el cuerpo recibido.
function isProjectDetail(value: unknown): value is ProjectDetail {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.name === "string" &&
    typeof value.status === "string" &&
    typeof value.startDate === "string" &&
    typeof value.plannedEndDate === "string" &&
    typeof value.hoursPerStoryPoint === "number" &&
    isRecord(value.owner) &&
    Array.isArray(value.members)
  );
}

// Crea un proyecto con POST /api/v1/projects usando la sesión de la pestaña. Quien crea queda como
// propietario e integrante (FR-001); ante un 401 el cliente HTTP descarta la sesión (FR-016).
export function createProject(
  input: ProjectInput,
): Promise<ApiResult<ProjectDetail>> {
  return postAuthorizedJson("/api/v1/projects", input, isProjectDetail);
}

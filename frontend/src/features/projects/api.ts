import {
  getJson,
  isRecord,
  postAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  ProjectDetail,
  ProjectInput,
  ProjectListItem,
  ProjectListResponse,
} from "@/types/project";

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

// Reconoce un elemento del listado (ProjectListItem) con los campos que muestra la vista.
function isProjectListItem(value: unknown): value is ProjectListItem {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.name === "string" &&
    typeof value.status === "string" &&
    typeof value.startDate === "string" &&
    typeof value.plannedEndDate === "string" &&
    typeof value.memberCount === "number" &&
    typeof value.isOwner === "boolean"
  );
}

// Reconoce la respuesta del listado (200): la lista de proyectos y el mensaje opcional del estado
// vacío, que puede faltar o llegar en null.
function isProjectListResponse(value: unknown): value is ProjectListResponse {
  return (
    isRecord(value) &&
    Array.isArray(value.projects) &&
    value.projects.every(isProjectListItem) &&
    (value.message === undefined ||
      value.message === null ||
      typeof value.message === "string")
  );
}

// Crea un proyecto con POST /api/v1/projects usando la sesión de la pestaña. Quien crea queda como
// propietario e integrante (FR-001); ante un 401 el cliente HTTP descarta la sesión (FR-016).
export function createProject(
  input: ProjectInput,
): Promise<ApiResult<ProjectDetail>> {
  return postAuthorizedJson("/api/v1/projects", input, isProjectDetail);
}

// Lista los proyectos de los que participa quien consulta con GET /api/v1/projects, usando la
// sesión de la pestaña. El filtrado por membresía y el orden los resuelve el backend (FR-032,
// FR-033); ante un 401 el cliente HTTP descarta la sesión (FR-016).
export function listProjects(): Promise<ApiResult<ProjectListResponse>> {
  return getJson("/api/v1/projects", isProjectListResponse);
}

import {
  deleteEmpty,
  getJson,
  isRecord,
  patchAuthorizedJson,
  postAuthorizedJson,
  putAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  BacklogSummary,
  ProjectDetail,
  ProjectInput,
  ProjectListItem,
  ProjectListResponse,
  ProjectMemberView,
  ProjectStatus,
} from "@/types/project";

// Reconoce el resumen del backlog (BacklogSummary) con los contadores y los totales que muestra la
// vista de estado.
function isBacklogSummary(value: unknown): value is BacklogSummary {
  return (
    isRecord(value) &&
    isRecord(value.countsByState) &&
    typeof value.estimatedStoryPoints === "number" &&
    typeof value.estimatedHours === "number"
  );
}

// Reconoce el estado completo del proyecto que devuelven la creación exitosa (201) y la vista de
// estado (200), sin confiar en el cuerpo recibido.
function isProjectDetail(value: unknown): value is ProjectDetail {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.name === "string" &&
    typeof value.status === "string" &&
    typeof value.startDate === "string" &&
    typeof value.plannedEndDate === "string" &&
    typeof value.hoursPerStoryPoint === "number" &&
    typeof value.readOnly === "boolean" &&
    typeof value.hasActiveSprint === "boolean" &&
    isRecord(value.owner) &&
    Array.isArray(value.members) &&
    isBacklogSummary(value.backlog)
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

// Reconoce el integrante que devuelve el alta exitosa (201); el correo es opcional porque solo
// llega cuando consulta el propietario (FR-030).
function isProjectMemberView(value: unknown): value is ProjectMemberView {
  return (
    isRecord(value) &&
    typeof value.userId === "string" &&
    typeof value.fullName === "string" &&
    (value.email === undefined || typeof value.email === "string") &&
    typeof value.isOwner === "boolean" &&
    typeof value.joinedAt === "string"
  );
}

// Crea un proyecto con POST /api/v1/projects usando la sesión de la pestaña. Quien crea queda como
// propietario e integrante (FR-001); ante un 401 el cliente HTTP descarta la sesión (FR-016).
export function createProject(
  input: ProjectInput,
): Promise<ApiResult<ProjectDetail>> {
  return postAuthorizedJson("/api/v1/projects", input, isProjectDetail);
}

// Modifica los datos editables del proyecto con PUT /api/v1/projects/{projectId}, usando la sesión
// de la pestaña. Se envían solo los cinco campos de ProjectInput: el estado y la fecha de
// finalización real no son editables (FR-012, FR-016); los rechazos (400, 403, 404, 409) llegan con
// el mensaje del backend y ante un 401 el cliente HTTP descarta la sesión. El identificador se
// codifica para que no altere la ruta pedida.
export function updateProject(
  projectId: string,
  input: ProjectInput,
): Promise<ApiResult<ProjectDetail>> {
  return putAuthorizedJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}`,
    input,
    isProjectDetail,
  );
}

// Lista los proyectos de los que participa quien consulta con GET /api/v1/projects, usando la
// sesión de la pestaña. El filtrado por membresía y el orden los resuelve el backend (FR-032,
// FR-033); ante un 401 el cliente HTTP descarta la sesión (FR-016).
export function listProjects(): Promise<ApiResult<ProjectListResponse>> {
  return getJson("/api/v1/projects", isProjectListResponse);
}

// Agrega como integrante a la persona con el correo indicado con POST
// /api/v1/projects/{projectId}/members, usando la sesión de la pestaña. El correo se envía tal como
// se escribió: la normalización y las reglas de negocio las resuelve el backend (FR-020 a FR-022),
// y sus errores llegan con el mensaje del backend para mostrarlo. El identificador del proyecto se
// codifica para que no altere la ruta pedida.
export function addProjectMember(
  projectId: string,
  email: string,
): Promise<ApiResult<ProjectMemberView>> {
  return postAuthorizedJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/members`,
    { email },
    isProjectMemberView,
  );
}

// Quita al integrante indicado con DELETE /api/v1/projects/{projectId}/members/{userId}, usando la
// sesión de la pestaña. Ambos identificadores se codifican para que no alteren la ruta pedida. La
// baja exitosa (204) no trae contenido; los rechazos (403, 404, 409) llegan con el mensaje del
// backend.
export function removeProjectMember(
  projectId: string,
  userId: string,
): Promise<ApiResult<undefined>> {
  return deleteEmpty(
    `/api/v1/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`,
  );
}

// Pide el estado completo del proyecto con GET /api/v1/projects/{projectId}, usando la sesión de la
// pestaña (FR-042). Un proyecto del que no se es integrante llega como 404 sin datos (FR-049); ante
// un 401 el cliente HTTP descarta la sesión (FR-016). El identificador se codifica para que no
// altere la ruta pedida.
export function getProjectDetail(
  projectId: string,
): Promise<ApiResult<ProjectDetail>> {
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}`,
    isProjectDetail,
  );
}

// Hace avanzar el proyecto al estado indicado con PATCH /api/v1/projects/{projectId}/status, usando
// la sesión de la pestaña (002/US6). Las reglas de la máquina de estados las resuelve el backend
// (FR-037 a FR-040): sus rechazos (400, 403, 404, 409) llegan con el mensaje recibido y ante un 401
// el cliente HTTP descarta la sesión (FR-016). El identificador se codifica para que no altere la
// ruta pedida.
export function changeProjectStatus(
  projectId: string,
  status: ProjectStatus,
): Promise<ApiResult<ProjectDetail>> {
  return patchAuthorizedJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/status`,
    { status },
    isProjectDetail,
  );
}

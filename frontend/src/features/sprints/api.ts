import {
  deleteAuthorizedJson,
  getJson,
  isRecord,
  postAuthorizedJson,
  putAuthorizedJson,
  type ApiResult,
} from "@/lib/api-client";
import type {
  SprintDetail,
  SprintInput,
  StoryProgressResponse,
} from "@/types/sprint";
import type { StoryStatus } from "@/types/story";

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

// Arma la ruta de un sprint codificando el identificador, por el mismo motivo que la de los sprints
// del proyecto.
function sprintPath(sprintId: string): string {
  return `/api/v1/sprints/${encodeURIComponent(sprintId)}`;
}

// Consulta el detalle de un sprint con GET /api/v1/sprints/{sprintId}, usando la sesión de la
// pestaña. El sprint inexistente o ajeno llega como 404 con su mensaje y ante un 401 el cliente
// HTTP descarta la sesión.
export function getSprint(sprintId: string): Promise<ApiResult<SprintDetail>> {
  return getJson(sprintPath(sprintId), isSprintDetail);
}

// Compromete un lote de historias en el sprint con POST /api/v1/sprints/{sprintId}/stories
// (004/US2). La admisión y la atomicidad las resuelve el backend: el 409 de historias no admitidas
// llega con el motivo de cada una en stories, los demás rechazos solo con su mensaje.
export function assignSprintStories(
  sprintId: string,
  storyIds: string[],
): Promise<ApiResult<SprintDetail>> {
  return postAuthorizedJson(
    `${sprintPath(sprintId)}/stories`,
    { storyIds },
    isSprintDetail,
  );
}

// Quita historias del sprint con DELETE /api/v1/sprints/{sprintId}/stories, enviando cada
// identificador como parámetro storyId repetido (004/US2). Con el 200 llega el detalle actualizado;
// las reglas (historia Completada, sprint Cerrado) las resuelve el backend.
export function removeSprintStories(
  sprintId: string,
  storyIds: string[],
): Promise<ApiResult<SprintDetail>> {
  const query = new URLSearchParams();
  storyIds.forEach((storyId) => query.append("storyId", storyId));
  return deleteAuthorizedJson(
    `${sprintPath(sprintId)}/stories?${query.toString()}`,
    isSprintDetail,
  );
}

// Inicia un sprint Planificado con POST /api/v1/sprints/{sprintId}/start, sin cuerpo y con la
// sesión de la pestaña (004/US3). Las reglas del inicio (otro sprint Activo, transición inválida,
// sprint Cerrado, proyecto Finalizado) las resuelve el backend y llegan como rechazo con su mensaje;
// con el 200 llega el detalle Activo con su momento de inicio real. El body undefined hace que el
// cliente HTTP no envíe cuerpo ni Content-Type, y el detalle recibido se valida con isSprintDetail.
export function startSprint(
  sprintId: string,
): Promise<ApiResult<SprintDetail>> {
  return postAuthorizedJson(
    `${sprintPath(sprintId)}/start`,
    undefined,
    isSprintDetail,
  );
}

// Reconoce el avance registrado (StoryProgressResponse) que devuelve el 200, sin confiar en el cuerpo
// recibido: el estado debe ser uno de los conocidos y completedAt llega como texto o en null.
function isStoryProgressResponse(
  value: unknown,
): value is StoryProgressResponse {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.sprintId === "string" &&
    (value.status === "pending" ||
      value.status === "in_progress" ||
      value.status === "completed") &&
    (value.completedAt === null || typeof value.completedAt === "string")
  );
}

// Registra el avance de una historia del sprint Activo con PUT /api/v1/stories/{storyId}/status y
// el cuerpo { status }, usando la sesión de la pestaña (004/US4). Las reglas del avance (transiciones
// admitidas, sprint Activo, sprint Cerrado) las resuelve el backend y llegan como rechazo con su
// mensaje; con el 200 llega el estado registrado y su completedAt.
export function changeStoryStatus(
  storyId: string,
  status: StoryStatus,
): Promise<ApiResult<StoryProgressResponse>> {
  return putAuthorizedJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/status`,
    { status },
    isStoryProgressResponse,
  );
}

// Cierra un sprint Activo con POST /api/v1/sprints/{sprintId}/close, sin cuerpo y con la sesión de
// la pestaña (004/US5). La instantánea, el destino de cada historia y las reglas del cierre (sprint
// no Activo, sprint ya Cerrado, proyecto Finalizado) las resuelve el backend y llegan como rechazo
// con su mensaje; con el 200 llega el detalle Cerrado con results, closedBy y el factor copiado.
export function closeSprint(
  sprintId: string,
): Promise<ApiResult<SprintDetail>> {
  return postAuthorizedJson(
    `${sprintPath(sprintId)}/close`,
    undefined,
    isSprintDetail,
  );
}

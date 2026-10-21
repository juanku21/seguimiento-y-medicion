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
  AppliedFilters,
  StoryDetail,
  StoryInput,
  StoryListItem,
  StoryListResponse,
  StoryPatch,
  StoryPoints,
} from "@/types/story";

// Reconoce el detalle de la historia (StoryDetail) que devuelven la creación exitosa (201) y la
// consulta del detalle (200), sin confiar en el cuerpo recibido. Los Story Points y la descripción
// pueden llegar en null.
function isStoryDetail(value: unknown): value is StoryDetail {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.projectId === "string" &&
    typeof value.title === "string" &&
    (value.description === null || typeof value.description === "string") &&
    typeof value.priority === "string" &&
    typeof value.status === "string" &&
    (value.storyPoints === null || typeof value.storyPoints === "number") &&
    Array.isArray(value.acceptanceCriteria) &&
    isRecord(value.readiness) &&
    typeof value.readiness.ready === "boolean" &&
    Array.isArray(value.readiness.missing) &&
    isRecord(value.createdBy) &&
    typeof value.createdAt === "string" &&
    typeof value.updatedAt === "string" &&
    typeof value.everAssignedToSprint === "boolean"
  );
}

// Registra una historia en el backlog con POST /api/v1/projects/{projectId}/stories, usando la
// sesión de la pestaña (003/US1). Las reglas de negocio las resuelve el backend: los rechazos (400
// con el detalle por campo, 404 y 409) llegan con su mensaje y ante un 401 el cliente HTTP descarta
// la sesión (FR-016 de la 001). El identificador del proyecto se codifica para que no altere la
// ruta pedida.
export function createStory(
  projectId: string,
  input: StoryInput,
): Promise<ApiResult<StoryDetail>> {
  return postAuthorizedJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/stories`,
    input,
    isStoryDetail,
  );
}

// Consulta el detalle de una historia con GET /api/v1/stories/{storyId}, usando la sesión de la
// pestaña (003/US3). El identificador se codifica para que no altere la ruta pedida. La historia
// ajena y la inexistente llegan con el mismo 404 y su mensaje, sin datos de la historia (FR-047), y
// ante un 401 el cliente HTTP descarta la sesión.
export function getStory(storyId: string): Promise<ApiResult<StoryDetail>> {
  return getJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}`,
    isStoryDetail,
  );
}

// Estima una historia con PUT /api/v1/stories/{storyId}/story-points, usando la sesión de la
// pestaña (003/US4). null la devuelve a "sin estimar" y el 0 viaja como 0 (FR-015). Las reglas
// (escala, historia Completada, comprometida y proyecto Finalizado) las resuelve el backend: los
// rechazos (400 con el detalle por campo, 404 y 409) llegan con su mensaje y ante un 401 el cliente
// HTTP descarta la sesión.
export function estimateStory(
  storyId: string,
  storyPoints: StoryPoints,
): Promise<ApiResult<StoryDetail>> {
  return putAuthorizedJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}/story-points`,
    { storyPoints },
    isStoryDetail,
  );
}

// Refina una historia con PATCH /api/v1/stories/{storyId}, usando la sesión de la pestaña
// (003/US5). El cuerpo trae solo los campos cambiados, para no pisar los que otro integrante haya
// modificado mientras tanto (FR-025). Las reglas (historia Completada, comprometida sin criterios y
// proyecto Finalizado) las resuelve el backend: los rechazos (400 con el detalle por campo, 404 y
// 409) llegan con su mensaje y ante un 401 el cliente HTTP descarta la sesión.
export function updateStory(
  storyId: string,
  patch: StoryPatch,
): Promise<ApiResult<StoryDetail>> {
  return patchAuthorizedJson(
    `/api/v1/stories/${encodeURIComponent(storyId)}`,
    patch,
    isStoryDetail,
  );
}

// Elimina una historia con DELETE /api/v1/stories/{storyId}, usando la sesión de la pestaña
// (003/US6). La eliminación es definitiva y el 204 no trae contenido. Las reglas (historial
// asociado y proyecto Finalizado) las resuelve el backend: el 409 por historial llega con cada
// motivo en reasons, el de solo lectura y el 404 solo con su mensaje, y ante un 401 el cliente HTTP
// descarta la sesión.
export function deleteStory(storyId: string): Promise<ApiResult<undefined>> {
  return deleteEmpty(`/api/v1/stories/${encodeURIComponent(storyId)}`);
}

// Reconoce un elemento del listado del backlog (StoryListItem); los Story Points pueden llegar en
// null cuando la historia no está estimada.
function isStoryListItem(value: unknown): value is StoryListItem {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.title === "string" &&
    typeof value.priority === "string" &&
    typeof value.status === "string" &&
    (value.storyPoints === null || typeof value.storyPoints === "number") &&
    typeof value.acceptanceCriteriaCount === "number" &&
    typeof value.readyToPlan === "boolean"
  );
}

// Reconoce la respuesta del listado (StoryListResponse): las historias, los filtros aplicados y el
// motivo del vacío, sin confiar en el cuerpo recibido.
function isStoryListResponse(value: unknown): value is StoryListResponse {
  return (
    isRecord(value) &&
    Array.isArray(value.stories) &&
    value.stories.every(isStoryListItem) &&
    isRecord(value.appliedFilters) &&
    Array.isArray(value.appliedFilters.status) &&
    Array.isArray(value.appliedFilters.priority) &&
    (value.emptyReason === null || typeof value.emptyReason === "string")
  );
}

// Pide el backlog del proyecto con GET /api/v1/projects/{projectId}/stories, usando la sesión de la
// pestaña (003/US2). Cada valor elegido de un filtro viaja como parámetro repetido
// (?status=pending&status=in_progress); sin filtros no se envía consulta. El orden, el filtrado y el
// motivo del vacío los resuelve el backend (FR-032 a FR-037). El identificador del proyecto se
// codifica para que no altere la ruta pedida.
export function listStories(
  projectId: string,
  filters: AppliedFilters,
): Promise<ApiResult<StoryListResponse>> {
  const query = new URLSearchParams();
  filters.status.forEach((status) => query.append("status", status));
  filters.priority.forEach((priority) => query.append("priority", priority));
  const encoded = query.toString();
  const search = encoded === "" ? "" : `?${encoded}`;
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/stories${search}`,
    isStoryListResponse,
  );
}

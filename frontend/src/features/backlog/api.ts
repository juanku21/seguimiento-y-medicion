import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import type { StoryDetail, StoryInput } from "@/types/story";

// Reconoce el detalle de la historia (StoryDetail) que devuelve la creación exitosa (201), sin
// confiar en el cuerpo recibido. Los Story Points y la descripción pueden llegar en null.
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
// la sesión (FR-016 de la 001).
export function createStory(
  projectId: string,
  input: StoryInput,
): Promise<ApiResult<StoryDetail>> {
  return postAuthorizedJson(
    `/api/v1/projects/${projectId}/stories`,
    input,
    isStoryDetail,
  );
}

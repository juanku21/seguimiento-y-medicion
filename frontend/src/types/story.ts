// Tipos de las peticiones y respuestas de las historias del backlog, según contracts/openapi.yaml
// (spec 003). Los errores (ErrorResponse, ValidationErrorResponse) se reutilizan de @/types/auth.
import type { STORY_POINTS_SCALE } from "@/utils/story-labels";

// Prioridad de la historia (FR-004).
export type StoryPriority = "high" | "medium" | "low";

// Estado de la historia; solo lo cambia la feature de sprints (FR-026, FR-027).
export type StoryStatus = "pending" | "in_progress" | "completed";

// Valor de la escala de Story Points; null significa "sin estimar" y el 0 es válido (FR-014, FR-015).
export type StoryPoints = (typeof STORY_POINTS_SCALE)[number] | null;

// Cuerpo de POST /api/v1/projects/{projectId}/stories (StoryInput).
export type StoryInput = {
  title: string;
  description?: string | null;
  priority: StoryPriority;
  storyPoints?: StoryPoints;
  acceptanceCriteria?: string[];
};

// Condición que falta para que la historia quede lista para planificar.
export type ReadinessMissing = "story_points" | "acceptance_criteria";

// Condición de lista para planificar y qué falta cuando no se cumple (Readiness, FR-028, FR-029).
export type Readiness = {
  ready: boolean;
  missing: ReadinessMissing[];
};

// Autor de la historia (AuthorRef, FR-050).
export type AuthorRef = {
  id: string;
  fullName: string;
};

// Detalle completo de una historia, respuesta de las escrituras (StoryDetail, FR-039).
export type StoryDetail = {
  id: string;
  projectId: string;
  title: string;
  description: string | null;
  priority: StoryPriority;
  status: StoryStatus;
  storyPoints: StoryPoints;
  acceptanceCriteria: string[];
  readiness: Readiness;
  createdBy: AuthorRef;
  createdAt: string;
  updatedAt: string;
  everAssignedToSprint: boolean;
};

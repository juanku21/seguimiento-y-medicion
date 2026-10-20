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

// Elemento del listado del backlog (StoryListItem, FR-033).
export type StoryListItem = {
  id: string;
  title: string;
  priority: StoryPriority;
  status: StoryStatus;
  storyPoints: StoryPoints;
  acceptanceCriteriaCount: number;
  readyToPlan: boolean;
};

// Filtros del listado; cada uno admite varios valores y vacío significa sin filtrar
// (AppliedFilters, FR-034, FR-036).
export type AppliedFilters = {
  status: StoryStatus[];
  priority: StoryPriority[];
};

// Motivo de un listado vacío: null si hay historias, backlog_empty si el proyecto no tiene ninguna
// y no_matches si ninguna cumple los filtros (FR-035, FR-036).
export type StoryListEmptyReason = "backlog_empty" | "no_matches" | null;

// Respuesta de GET /api/v1/projects/{projectId}/stories (StoryListResponse).
export type StoryListResponse = {
  stories: StoryListItem[];
  appliedFilters: AppliedFilters;
  emptyReason: StoryListEmptyReason;
};

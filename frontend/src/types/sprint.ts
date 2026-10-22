// Tipos de las peticiones y respuestas de los sprints, según contracts/openapi.yaml (spec 004).
// Los errores (ErrorResponse, ValidationErrorResponse) se reutilizan de @/types/auth y la prioridad,
// el estado y los Story Points de la historia, de @/types/story.
import type {
  AuthorRef,
  StoryPoints,
  StoryPriority,
  StoryStatus,
} from "@/types/story";

// Estado del sprint en su ciclo de vida (RN-02).
export type SprintStatus = "planned" | "active" | "closed";

// Resultado de la historia en un sprint cerrado; mientras el sprint está abierto llega null.
export type StoryOutcome = "completed" | "not_completed";

// Cuerpo de la creación de un sprint (SprintInput, FR-001 a FR-004); las fechas van en AAAA-MM-DD.
export type SprintInput = {
  name: string;
  goal: string;
  startDate: string;
  endDate: string;
};

// Referencia a un usuario (UserRef); tiene la misma forma que el autor de la historia de la 003.
export type UserRef = AuthorRef;

// Identificación mínima de un sprint, usada en los mensajes de conflicto (SprintRef).
export type SprintRef = {
  id: string;
  name: string;
};

// Totales congelados en la instantánea de un sprint cerrado (SprintResults, FR-043, FR-051).
export type SprintResults = {
  committedStoryPoints: number;
  plannedStoryPoints: number;
  completedStoryPoints: number;
  plannedStoryCount: number;
  completedStoryCount: number;
};

// Días restantes o vencidos del sprint Activo; exactamente uno de los dos es distinto de null
// (SprintSchedule, FR-053, RN-23).
export type SprintSchedule = {
  remainingDays: number | null;
  overdueDays: number | null;
};

// Historia comprometida en el sprint (SprintStoryItem); siempre tiene Story Points estimados.
export type SprintStoryItem = {
  storyId: string;
  title: string;
  priority: StoryPriority;
  storyPoints: NonNullable<StoryPoints>;
  status: StoryStatus;
  completedAt: string | null;
  outcome: StoryOutcome | null;
};

// Estado completo del sprint, respuesta de la creación (SprintDetail). Los campos de inicio, cierre,
// resultados y cronograma llegan en null mientras no correspondan a su estado.
export type SprintDetail = {
  id: string;
  projectId: string;
  name: string;
  goal: string;
  status: SprintStatus;
  startDate: string;
  endDate: string;
  startedAt: string | null;
  closedAt: string | null;
  createdBy: UserRef;
  closedBy: UserRef | null;
  createdAt: string;
  updatedAt: string;
  stories: SprintStoryItem[];
  results: SprintResults | null;
  hoursPerStoryPoint: number | null;
  schedule: SprintSchedule | null;
};

// Rechazo 409 por conflicto con otro sprint del proyecto, con el sprint en conflicto
// (SprintConflictResponse).
export type SprintConflictResponse = {
  error: string;
  sprint: SprintRef;
};

// Resultado del registro de avance de una historia del sprint Activo (StoryProgressResponse,
// FR-033 a FR-035); completedAt llega en null salvo cuando la historia queda Completada.
export type StoryProgressResponse = {
  storyId: string;
  sprintId: string;
  status: StoryStatus;
  completedAt: string | null;
};

// Elemento del listado de sprints del proyecto (SprintListItem, FR-050, FR-051). results trae los
// totales de la instantánea solo en un sprint cerrado; en los demás llega en null.
export type SprintListItem = {
  id: string;
  name: string;
  goal: string;
  status: SprintStatus;
  startDate: string;
  endDate: string;
  startedAt: string | null;
  closedAt: string | null;
  storyCount: number;
  results: SprintResults | null;
};

// Motivo del listado vacío: el proyecto sin sprints (FR-054) o sprints que no cumplen el filtro.
export type SprintListEmptyReason = "no_sprints" | "no_matches";

// Respuesta del listado de sprints (SprintListResponse); sprints y appliedFilters.status nunca
// llegan en null y emptyReason es null cuando hay resultados.
export type SprintListResponse = {
  sprints: SprintListItem[];
  appliedFilters: { status: SprintStatus[] };
  emptyReason: SprintListEmptyReason | null;
};

// Sprint en el que estuvo una historia y el resultado que tuvo en él (StorySprintHistoryItem,
// FR-055); outcome llega en null mientras el sprint está abierto.
export type StorySprintHistoryItem = {
  sprintId: string;
  sprintName: string;
  sprintStatus: SprintStatus;
  startDate: string;
  endDate: string;
  outcome: StoryOutcome | null;
  completedAt: string | null;
};

// Respuesta del historial de sprints de una historia (StorySprintHistoryResponse).
export type StorySprintHistoryResponse = {
  sprints: StorySprintHistoryItem[];
};

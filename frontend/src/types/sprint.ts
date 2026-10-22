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

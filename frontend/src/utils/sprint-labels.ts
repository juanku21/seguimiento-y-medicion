import type { SprintStatus, StoryOutcome } from "@/types/sprint";

// Etiquetas en español del estado del sprint; el backend envía los valores en inglés
// (RN-02, Principio VIII).
export const SPRINT_STATUS_LABELS: Record<SprintStatus, string> = {
  planned: "Planificado",
  active: "Activo",
  closed: "Cerrado",
};

// Etiquetas en español del resultado de la historia en un sprint cerrado (Principio VIII).
export const STORY_OUTCOME_LABELS: Record<StoryOutcome, string> = {
  completed: "Completada",
  not_completed: "No completada",
};

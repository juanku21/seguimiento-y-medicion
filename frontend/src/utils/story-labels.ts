import type { StoryPriority, StoryStatus } from "@/types/story";

// Etiquetas en español de la prioridad de la historia; el backend envía los valores en inglés
// (FR-004, Principio VIII).
export const STORY_PRIORITY_LABELS: Record<StoryPriority, string> = {
  high: "Alta",
  medium: "Media",
  low: "Baja",
};

// Etiquetas en español del estado de la historia (FR-026, Principio VIII).
export const STORY_STATUS_LABELS: Record<StoryStatus, string> = {
  pending: "Pendiente",
  in_progress: "En progreso",
  completed: "Completada",
};

// Etiqueta en español de la condición de lista para planificar (FR-028).
export function readinessLabel(ready: boolean): string {
  return ready ? "Lista para planificar" : "No lista para planificar";
}

// Escala de Story Points admitida, espejo de domain.StoryPointsScale del backend
// (data-model.md sección 5, FR-014). Se declara una sola vez y de ella deriva el tipo StoryPoints.
export const STORY_POINTS_SCALE = [0, 1, 2, 3, 5, 8, 13, 20, 40, 100] as const;

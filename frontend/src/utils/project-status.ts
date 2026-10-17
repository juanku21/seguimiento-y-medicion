import type { ProjectStatus } from "@/types/project";

// Etiquetas en español de los estados del proyecto; el backend envía los valores en inglés
// (FR-036, Principio VIII).
export const PROJECT_STATUS_LABELS: Record<ProjectStatus, string> = {
  planned: "Planificado",
  in_progress: "En curso",
  finished: "Finalizado",
};

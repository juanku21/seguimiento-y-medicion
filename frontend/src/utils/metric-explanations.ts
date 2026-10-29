// Explicaciones de las métricas "no calculable" (FR-014; research.md sección 6).

// Métricas del dashboard que pueden llegar como no calculables.
export type MetricKind =
  | "velocity"
  | "storyPoints"
  | "storyCompletionPercent"
  | "hours"
  | "deviationHours"
  | "deviationPercent"
  | "defects";

// Devuelve el texto fijo que nombra el denominador ausente de cada métrica;
// cualquier otra métrica usa un texto genérico para que ningún caso quede sin explicación.
export function explainNotCalculable(kind: MetricKind): string {
  switch (kind) {
    case "velocity":
      return "Todavía no hay sprints cerrados.";
    case "deviationPercent":
      return "Las horas estimadas son 0.";
    case "storyCompletionPercent":
      return "No hay historias planificadas.";
    default:
      return "No se puede calcular con los datos actuales.";
  }
}

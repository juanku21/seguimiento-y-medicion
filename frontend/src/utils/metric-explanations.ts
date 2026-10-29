// Firmas mínimas: la implementación pertenece a la tarea T014.

export type MetricKind =
  | "velocity"
  | "storyPoints"
  | "storyCompletionPercent"
  | "hours"
  | "deviationHours"
  | "deviationPercent"
  | "defects";

export function explainNotCalculable(kind: MetricKind): string {
  void kind;
  throw new Error("not implemented");
}

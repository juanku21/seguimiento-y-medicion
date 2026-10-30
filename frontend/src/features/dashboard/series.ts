import type { SeriesPoint } from "@/types/metrics";

// Única selección que hace el dashboard: la velocidad se mide sobre sprints
// terminados (US3-2), así que se omiten los puntos del sprint activo. No ordena,
// no suma ni completa puntos: devuelve los mismos objetos en el orden recibido.
export function closedSprintPoints(points: SeriesPoint[]): SeriesPoint[] {
  return points.filter((point) => point.sprintStatus !== "active");
}

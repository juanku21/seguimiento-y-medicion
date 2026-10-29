import type { ApiResult } from "@/lib/api-client";
import type {
  ActiveSprintMetricsResponse,
  Metric,
  ProjectMetricsResponse,
  SprintMetric,
} from "@/types/metrics";

// Reconoce un valor calculado coherente entre su estado y su número.
export function isMetric(value: unknown): value is Metric {
  return false;
}

// Reconoce una métrica de sprint con finalidad válida o null.
export function isSprintMetric(value: unknown): value is SprintMetric {
  return false;
}

// Reconoce la respuesta de las métricas del sprint activo.
export function isActiveSprintMetricsResponse(
  value: unknown,
): value is ActiveSprintMetricsResponse {
  return false;
}

// Reconoce la respuesta de las métricas del proyecto.
export function isProjectMetricsResponse(
  value: unknown,
): value is ProjectMetricsResponse {
  return false;
}

// Consulta las métricas del proyecto.
export function getProjectMetrics(
  projectId: string,
): Promise<ApiResult<ProjectMetricsResponse>> {
  return Promise.reject(new Error("not implemented"));
}

// Consulta las métricas del sprint activo.
export function getActiveSprintMetrics(
  projectId: string,
): Promise<ApiResult<ActiveSprintMetricsResponse>> {
  return Promise.reject(new Error("not implemented"));
}

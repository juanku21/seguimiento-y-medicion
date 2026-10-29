// Este módulo solo lee (RN-02): todas sus consultas usan getJson y ninguna escribe.
import { getJson, isRecord, type ApiResult } from "@/lib/api-client";
import type {
  ActiveSprintMetricsResponse,
  Metric,
  MetricsWarning,
  ProjectMetricsResponse,
  SprintMetric,
} from "@/types/metrics";

const VALUE_STATUSES = ["available", "not_calculable", "not_applicable"];
const FINALITIES = ["definitive", "provisional", "partial"];
// El acumulado nunca es provisional: se excluye de las finalidades válidas.
const CUMULATIVE_FINALITIES = FINALITIES.filter((f) => f !== "provisional");
const WARNING_KINDS = ["planning", "data_inconsistency"];
const SEVERITIES = ["critical", "high", "medium", "low"];
const METRIC_KEYS = [
  "plannedStoryPoints",
  "completedStoryPoints",
  "estimatedHours",
  "actualHours",
  "deviationHours",
  "deviationPercent",
  "storyCompletionPercent",
];

// Indica si el valor es una cadena incluida en la lista dada.
function isOneOf(value: unknown, options: string[]): boolean {
  return typeof value === "string" && options.includes(value);
}

// Reconoce un valor calculado coherente entre su estado y su número.
export function isMetric(value: unknown): value is Metric {
  if (!isRecord(value) || !isOneOf(value.status, VALUE_STATUSES)) {
    return false;
  }
  return value.status === "available"
    ? typeof value.value === "number"
    : value.value === null;
}

// Reconoce una métrica de sprint con finalidad válida o null.
export function isSprintMetric(value: unknown): value is SprintMetric {
  if (!isRecord(value)) {
    return false;
  }
  const finality = value.finality;
  return (
    isMetric(value) && (finality === null || isOneOf(finality, FINALITIES))
  );
}

// Reconoce la respuesta de las métricas del sprint activo.
export function isActiveSprintMetricsResponse(
  value: unknown,
): value is ActiveSprintMetricsResponse {
  if (!isRecord(value) || typeof value.activeSprint !== "boolean") {
    return false;
  }
  if (!value.activeSprint) {
    return value.metrics === null;
  }
  const metrics = value.metrics;
  return (
    isRecord(metrics) &&
    METRIC_KEYS.every((key) => isSprintMetric(metrics[key]))
  );
}

// Reconoce una advertencia con tipo, código y mensaje.
function isWarning(value: unknown): value is MetricsWarning {
  return (
    isRecord(value) &&
    isOneOf(value.kind, WARNING_KINDS) &&
    typeof value.code === "string" &&
    typeof value.message === "string"
  );
}

// Reconoce el acumulado del proyecto: finalidad definitiva o parcial y sus siete métricas.
function isCumulative(value: unknown): boolean {
  return (
    isRecord(value) &&
    isOneOf(value.finality, CUMULATIVE_FINALITIES) &&
    METRIC_KEYS.every((key) => isMetric(value[key]))
  );
}

// Reconoce los defectos abiertos: total y cantidad por cada severidad.
function isOpenDefects(value: unknown): boolean {
  if (!isRecord(value) || typeof value.total !== "number") {
    return false;
  }
  const bySeverity = value.bySeverity;
  return (
    isRecord(bySeverity) &&
    SEVERITIES.every((severity) => typeof bySeverity[severity] === "number")
  );
}

// Reconoce la respuesta de las métricas del proyecto.
export function isProjectMetricsResponse(
  value: unknown,
): value is ProjectMetricsResponse {
  return (
    isRecord(value) &&
    isRecord(value.velocity) &&
    isMetric(value.velocity.value) &&
    isCumulative(value.cumulative) &&
    isRecord(value.defects) &&
    isOpenDefects(value.defects.open) &&
    Array.isArray(value.warnings) &&
    value.warnings.every(isWarning)
  );
}

// Consulta las métricas del proyecto.
export function getProjectMetrics(
  projectId: string,
): Promise<ApiResult<ProjectMetricsResponse>> {
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/metrics`,
    isProjectMetricsResponse,
  );
}

// Consulta las métricas del sprint activo.
export function getActiveSprintMetrics(
  projectId: string,
): Promise<ApiResult<ActiveSprintMetricsResponse>> {
  return getJson(
    `/api/v1/projects/${encodeURIComponent(projectId)}/metrics/active-sprint`,
    isActiveSprintMetricsResponse,
  );
}

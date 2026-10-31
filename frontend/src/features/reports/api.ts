// Acceso a la API de reportes y reconocedores de sus respuestas (Principio VII).
import {
  isProjectMetricsResponse,
  isSprintMetric,
} from "@/features/dashboard/api";
import { isRecord, postAuthorizedJson, type ApiResult } from "@/lib/api-client";
import type { Report } from "@/types/report";

// Valores válidos de cada enumeración del contrato.
const SCOPES = ["sprint", "project"];
const SECTION_STATES = ["complete", "empty", "unavailable"];
const GROUP_STATES = ["complete", "empty"];
const PROJECT_STATUSES = ["planned", "in_progress", "finished"];
const REPORT_SPRINT_STATUSES = ["active", "closed"];
const METRICS_SPRINT_STATUSES = ["planned", "active", "closed"];
const PRIORITIES = ["high", "medium", "low"];
const STORY_RESULTS = ["pending", "in_progress", "completed", "not_completed"];
const POKER_STATES = ["agreed", "no_agreement", "none"];
const SEVERITIES = ["critical", "high", "medium", "low"];
const DEFECT_STATUSES = ["open", "in_progress", "resolved"];
// Claves de las nueve métricas de un sprint.
const SPRINT_METRIC_KEYS = [
  "plannedStoryPoints",
  "completedStoryPoints",
  "estimatedHours",
  "actualHours",
  "deviationHours",
  "deviationPercent",
  "storyCompletionPercent",
  "defectsDetected",
  "defectsResolved",
];

// Indica si el valor es una cadena incluida en la lista dada.
function isOneOf(value: unknown, options: string[]): boolean {
  return typeof value === "string" && options.includes(value);
}

// Indica si el valor es una cadena o null.
function isStringOrNull(value: unknown): boolean {
  return value === null || typeof value === "string";
}

// Indica si el valor es un número o null.
function isNumberOrNull(value: unknown): boolean {
  return value === null || typeof value === "number";
}

// Indica si el valor es un arreglo cuyos elementos cumplen el reconocedor.
function isArrayOf(
  value: unknown,
  isItem: (item: unknown) => boolean,
): boolean {
  return Array.isArray(value) && value.every(isItem);
}

// Indica si el valor es un objeto con id y nombre de tipo texto.
function isIdName(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.name === "string"
  );
}

// Reconoce el sprint del reporte, que solo puede ser Activo o Cerrado.
function isReportSprint(value: unknown): boolean {
  return (
    isIdName(value) &&
    isRecord(value) &&
    isOneOf(value.status, REPORT_SPRINT_STATUSES) &&
    typeof value.startDate === "string" &&
    typeof value.endDate === "string" &&
    isStringOrNull(value.closedAt)
  );
}

// Reconoce un sprint del reporte o null.
function isReportSprintOrNull(value: unknown): boolean {
  return value === null || isReportSprint(value);
}

// Reconoce el período cubierto por el reporte.
function isPeriod(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.startDate === "string" &&
    typeof value.endDate === "string"
  );
}

// Reconoce el encabezado del reporte.
function isHeader(value: unknown): boolean {
  if (!isRecord(value)) {
    return false;
  }
  const generatedBy = value.generatedBy;
  return (
    typeof value.projectId === "string" &&
    typeof value.projectName === "string" &&
    isOneOf(value.projectStatus, PROJECT_STATUSES) &&
    isReportSprintOrNull(value.sprint) &&
    (value.period === null || isPeriod(value.period)) &&
    typeof value.generatedAt === "string" &&
    isRecord(generatedBy) &&
    typeof generatedBy.id === "string" &&
    typeof generatedBy.fullName === "string"
  );
}

// Reconoce una sección con estado válido y una lista que es null solo con `unavailable`.
function isListSection(
  value: unknown,
  listKey: string,
  isItem: (item: unknown) => boolean,
): boolean {
  if (!isRecord(value) || !isOneOf(value.state, SECTION_STATES)) {
    return false;
  }
  return value.state === "unavailable"
    ? value[listKey] === null
    : isArrayOf(value[listKey], isItem);
}

// Reconoce un grupo con sprint, estado válido y la lista indicada.
function isGroup(
  value: unknown,
  listKey: string,
  isItem: (item: unknown) => boolean,
  isSprint: (sprint: unknown) => boolean,
): boolean {
  return (
    isRecord(value) &&
    isSprint(value.sprint) &&
    isOneOf(value.state, GROUP_STATES) &&
    isArrayOf(value[listKey], isItem)
  );
}

// Reconoce una línea de la sección de historias.
function isStoryLine(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.title === "string" &&
    isOneOf(value.priority, PRIORITIES) &&
    isNumberOrNull(value.storyPoints) &&
    isOneOf(value.status, STORY_RESULTS)
  );
}

// Reconoce el resumen de la sesión de poker.
function isPokerSummary(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.rounds === "number" &&
    typeof value.agreedStoryPoints === "number" &&
    typeof value.finishedAt === "string"
  );
}

// Reconoce una línea de la sección de estimaciones.
function isEstimationLine(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.title === "string" &&
    isNumberOrNull(value.storyPoints) &&
    isOneOf(value.poker, POKER_STATES) &&
    (value.session === null || isPokerSummary(value.session))
  );
}

// Reconoce las horas registradas en una historia.
function isStoryEffort(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.storyId === "string" &&
    typeof value.title === "string" &&
    typeof value.hours === "number"
  );
}

// Reconoce el esfuerzo de un sprint, por historia.
function isEffortGroup(value: unknown): boolean {
  return (
    isGroup(value, "stories", isStoryEffort, isReportSprint) &&
    isRecord(value) &&
    typeof value.totalHours === "number"
  );
}

// Reconoce las horas registradas por integrante.
function isMemberEffort(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.userId === "string" &&
    typeof value.fullName === "string" &&
    typeof value.hours === "number"
  );
}

// Reconoce la sección de esfuerzo; sus tres datos son null solo con `unavailable`.
function isEffortSection(value: unknown): boolean {
  if (!isRecord(value) || !isOneOf(value.state, SECTION_STATES)) {
    return false;
  }
  if (value.state === "unavailable") {
    return (
      value.groups === null &&
      value.byMember === null &&
      value.totalHours === null
    );
  }
  return (
    isArrayOf(value.groups, isEffortGroup) &&
    isArrayOf(value.byMember, isMemberEffort) &&
    typeof value.totalHours === "number"
  );
}

// Reconoce las métricas de un sprint: referencia al sprint, nueve métricas y advertencias.
function isSprintMetrics(value: unknown): boolean {
  if (!isRecord(value)) {
    return false;
  }
  const sprint = value.sprint;
  return (
    isIdName(sprint) &&
    isRecord(sprint) &&
    isOneOf(sprint.status, METRICS_SPRINT_STATUSES) &&
    typeof sprint.startDate === "string" &&
    typeof sprint.endDate === "string" &&
    isStringOrNull(sprint.closedAt) &&
    SPRINT_METRIC_KEYS.every((key) => isSprintMetric(value[key])) &&
    Array.isArray(value.warnings)
  );
}

// Reconoce la sección de métricas: con datos, exactamente una de sprint o project.
function isMetricsSection(value: unknown): boolean {
  if (!isRecord(value) || !isOneOf(value.state, SECTION_STATES)) {
    return false;
  }
  if (value.state === "unavailable") {
    return value.sprint === null && value.project === null;
  }
  if (value.sprint !== null) {
    return value.project === null && isSprintMetrics(value.sprint);
  }
  return isProjectMetricsResponse(value.project);
}

// Reconoce una línea de la sección de defectos.
function isDefectLine(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.defectId === "string" &&
    typeof value.description === "string" &&
    isOneOf(value.severity, SEVERITIES) &&
    isOneOf(value.status, DEFECT_STATUSES) &&
    isIdTitle(value.story) &&
    isIdName(value.detectedSprint) &&
    (value.resolvedSprint === null || isIdName(value.resolvedSprint))
  );
}

// Indica si el valor es un objeto con id y título de tipo texto.
function isIdTitle(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.title === "string"
  );
}

// Reconoce la sección de defectos; defects y totals son null solo con `unavailable`.
function isDefectsSection(value: unknown): boolean {
  if (!isRecord(value) || !isOneOf(value.state, SECTION_STATES)) {
    return false;
  }
  if (value.state === "unavailable") {
    return value.defects === null && value.totals === null;
  }
  const totals = value.totals;
  return (
    isArrayOf(value.defects, isDefectLine) &&
    isRecord(totals) &&
    typeof totals.detected === "number" &&
    typeof totals.resolved === "number"
  );
}

// Reconoce un cuerpo de reporte válido: alcance, bandera de parcialidad y las seis secciones.
export function isReport(value: unknown): value is Report {
  return (
    isRecord(value) &&
    isOneOf(value.scope, SCOPES) &&
    typeof value.partial === "boolean" &&
    isHeader(value.header) &&
    isListSection(value.stories, "groups", (group) =>
      isGroup(group, "stories", isStoryLine, isReportSprintOrNull),
    ) &&
    isListSection(value.estimations, "groups", (group) =>
      isGroup(group, "items", isEstimationLine, isReportSprintOrNull),
    ) &&
    isEffortSection(value.effort) &&
    isMetricsSection(value.metrics) &&
    isDefectsSection(value.defects)
  );
}

// Genera el reporte de un sprint; el backend no lee el cuerpo, por eso se envía uno vacío.
export function generateSprintReport(
  projectId: string,
  sprintId: string,
): Promise<ApiResult<Report>> {
  return postAuthorizedJson(
    `/api/v1/projects/${projectId}/reports/sprints/${sprintId}`,
    {},
    isReport,
  );
}

// Genera el reporte del proyecto completo; igual que el de sprint, se envía un cuerpo vacío.
export function generateProjectReport(
  projectId: string,
): Promise<ApiResult<Report>> {
  return postAuthorizedJson(
    `/api/v1/projects/${projectId}/reports/project`,
    {},
    isReport,
  );
}

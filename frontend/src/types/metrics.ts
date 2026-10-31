// Tipos de las respuestas de cálculo de métricas, según specs/008-metrics-calculation/contracts/openapi.yaml; solo los campos que usa el dashboard.

// Estado de disponibilidad de un valor calculado.
export type ValueStatus = "available" | "not_calculable" | "not_applicable";

// Grado de cierre de una métrica de sprint.
export type Finality = "definitive" | "provisional" | "partial";

// Valor calculado; `value` es null si y solo si `status` no es `available`.
export type Metric = {
  status: ValueStatus;
  value: number | null;
};

// Métrica de sprint con su finalidad (null en un sprint Planificado).
export type SprintMetric = Metric & { finality: Finality | null };

// Métricas del sprint activo.
export type ActiveSprintMetrics = {
  plannedStoryPoints: SprintMetric;
  completedStoryPoints: SprintMetric;
  estimatedHours: SprintMetric;
  actualHours: SprintMetric;
  deviationHours: SprintMetric;
  deviationPercent: SprintMetric;
  storyCompletionPercent: SprintMetric;
};

// Respuesta de las métricas del sprint activo; metrics es null sin sprint activo.
export type ActiveSprintMetricsResponse = {
  activeSprint: boolean;
  metrics: ActiveSprintMetrics | null;
};

// Métricas acumuladas del proyecto.
export type Cumulative = {
  finality: "definitive" | "partial";
  plannedStoryPoints: Metric;
  completedStoryPoints: Metric;
  estimatedHours: Metric;
  actualHours: Metric;
  deviationHours: Metric;
  deviationPercent: Metric;
  storyCompletionPercent: Metric;
};

// Sprint al que se refieren las métricas.
export type SprintMetricsRef = {
  id: string;
  name: string;
  status: "planned" | "active" | "closed";
  startDate: string;
  endDate: string;
  closedAt: string | null;
};

// Métricas de un sprint, con las nueve métricas y sus advertencias.
export type SprintMetrics = ActiveSprintMetrics & {
  sprint: SprintMetricsRef;
  defectsDetected: SprintMetric;
  defectsResolved: SprintMetric;
  warnings: MetricsWarning[];
};

// Cantidad de defectos por severidad.
export type SeverityBreakdown = {
  critical: number;
  high: number;
  medium: number;
  low: number;
};

// Total de defectos y su distribución por severidad.
export type DefectTally = {
  total: number;
  bySeverity: SeverityBreakdown;
};

// Tipo de advertencia de las métricas.
export type WarningKind = "planning" | "data_inconsistency";

// Advertencia de las métricas; `code` es string porque el dashboard no decide nada según el código.
export type MetricsWarning = {
  kind: WarningKind;
  code: string;
  message: string;
};

// Respuesta de las métricas del proyecto.
export type ProjectMetricsResponse = {
  velocity: { value: Metric; sprintsConsidered: number };
  cumulative: Cumulative;
  effortOutsideSprintsHours: number;
  effortInPlannedSprintsHours: number;
  unestimatedBacklogStories: number;
  defects: { detected: DefectTally; resolved: DefectTally; open: DefectTally };
  warnings: MetricsWarning[];
};

// Punto de una serie por sprint.
export type SeriesPoint = {
  sprintId: string;
  sprintName: string;
  sprintStatus: "active" | "closed";
  status: ValueStatus;
  value: number | null;
  finality: Finality;
};

// Respuesta de las series del proyecto, una lista de puntos por indicador.
export type ProjectSeriesResponse = {
  plannedStoryPoints: SeriesPoint[];
  completedStoryPoints: SeriesPoint[];
  estimatedHours: SeriesPoint[];
  actualHours: SeriesPoint[];
  defectsDetected: SeriesPoint[];
  defectsResolved: SeriesPoint[];
};

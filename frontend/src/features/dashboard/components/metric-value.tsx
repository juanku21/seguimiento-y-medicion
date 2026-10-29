import type { ReactNode } from "react";
import type { Metric, SprintMetric } from "@/types/metrics";
import type { MetricKind } from "@/utils/metric-explanations";

// Propiedades del presentador de métricas.
interface MetricValueProps {
  metric: Metric | SprintMetric;
  kind: MetricKind;
  format: (value: number) => string;
}

// Firma mínima: la implementación pertenece a la tarea T016.
export function MetricValue({ metric, kind, format }: MetricValueProps): ReactNode {
  void metric;
  void kind;
  void format;
  throw new Error("not implemented");
}

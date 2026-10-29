import type { ReactNode } from "react";
import type { SeriesPoint } from "@/types/metrics";
import type { MetricKind } from "@/utils/metric-explanations";

// Una serie del gráfico: su nombre en la leyenda y sus puntos por sprint.
export interface SeriesInput {
  name: string;
  points: SeriesPoint[];
}

// Propiedades del gráfico común de barras agrupadas por sprint.
export interface SprintSeriesChartProps {
  title: string;
  yAxisLabel: string;
  kind: MetricKind;
  format: (value: number) => string;
  emptyMessage: string;
  errorMessage: string;
  series: [SeriesInput, SeriesInput];
}

// Firma mínima: la implementación llega en la tarea T024.
export function SprintSeriesChart(props: SprintSeriesChartProps): ReactNode {
  void props;
  return null;
}

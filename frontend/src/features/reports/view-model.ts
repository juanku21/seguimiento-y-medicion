// Modelo de presentación del reporte que consumen la pantalla y el PDF.
import type { Report } from "@/types/report";
import type { Finality, Metric } from "@/types/metrics";
import type { MetricKind } from "@/utils/metric-explanations";

// Tabla ya formateada; footer es null si la tabla no tiene pie.
export type TableView = {
  columns: string[];
  rows: string[][];
  footer: string[] | null;
};

// Bloque de una sección: subtítulo de grupo, mensaje y tabla.
export type BlockView = {
  heading: string | null;
  message: string | null;
  table: TableView | null;
};

// Sección del reporte, siempre en el orden fijo de seis.
export type SectionView = {
  key: "header" | "stories" | "estimations" | "effort" | "metrics" | "defects";
  title: string;
  state: "complete" | "empty" | "unavailable";
  message: string | null;
  blocks: BlockView[];
};

// Reporte listo para mostrar.
export type ReportView = {
  title: string;
  partialNote: string | null;
  header: { label: string; value: string }[];
  sections: SectionView[];
  fileName: string;
};

// Arma el modelo de presentación; firma mínima, sin lógica todavía.
export function buildReportView(report: Report, timeZone: string): ReportView {
  void report;
  void timeZone;
  throw new Error("not implemented");
}

// Texto de una métrica según su estado y su finalidad; firma mínima, sin lógica todavía.
export function metricText(
  metric: Metric & { finality?: Finality | null },
  kind: MetricKind,
  format: (value: number) => string,
): string {
  void metric;
  void kind;
  void format;
  throw new Error("not implemented");
}

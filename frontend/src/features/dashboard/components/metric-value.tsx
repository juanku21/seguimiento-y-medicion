import type { ReactNode } from "react";
import type { Metric, SprintMetric } from "@/types/metrics";
import {
  explainNotCalculable,
  type MetricKind,
} from "@/utils/metric-explanations";
import { FinalityBadge } from "./finality-badge";

// Propiedades del presentador de métricas.
interface MetricValueProps {
  metric: Metric | SprintMetric;
  kind: MetricKind;
  format: (value: number) => string;
}

// Único presentador de métricas: distingue valor disponible, no calculable y no aplicable.
export function MetricValue({
  metric,
  kind,
  format,
}: MetricValueProps): ReactNode {
  // Solo las métricas de sprint traen finalidad.
  const finality = "finality" in metric ? metric.finality : null;

  let content: ReactNode;
  if (metric.status === "available" && metric.value !== null) {
    content = format(metric.value);
  } else if (metric.status === "not_calculable") {
    content = (
      <>
        <span>No calculable</span>
        <small className="block text-xs text-gray-600">
          {explainNotCalculable(kind)}
        </small>
      </>
    );
  } else {
    content = "No aplica";
  }

  return (
    <>
      {content}
      <FinalityBadge finality={finality} />
    </>
  );
}

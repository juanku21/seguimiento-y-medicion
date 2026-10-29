import type { ReactNode } from "react";
import type { MetricsWarning } from "@/types/metrics";

// Firma mínima: la implementación llega en T035.
export function DashboardWarnings(props: {
  warnings: MetricsWarning[];
}): ReactNode {
  void props;
  throw new Error("not implemented");
}

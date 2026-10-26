import type { ReactNode } from "react";
import type { EffortComparisonRow } from "@/types/effort";

// Propiedades de la tabla de comparación del proyecto: las historias en el orden que las devuelve la
// API.
export type EffortComparisonTableProps = {
  stories: EffortComparisonRow[];
};

// Comparación entre estimado y real de todas las historias del proyecto (006/US5). Firma mínima de
// T065: la implementación corresponde a T070.
export function EffortComparisonTable(
  props: EffortComparisonTableProps,
): ReactNode {
  void props;
  throw new Error("not implemented");
}

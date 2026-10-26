import type { ReactNode } from "react";
import type { EffortComparisonRow } from "@/types/effort";

// Propiedades de la comparación de una historia: la fila tal como la devuelve la API.
export type EffortComparisonCardProps = {
  comparison: EffortComparisonRow;
};

// Comparación entre estimado y real de una historia (006/US5). Firma mínima de T065: la
// implementación corresponde a T070.
export function EffortComparisonCard(
  props: EffortComparisonCardProps,
): ReactNode {
  void props;
  throw new Error("not implemented");
}

import type { ReactNode } from "react";
import type { SprintDetail } from "@/types/sprint";

// Propiedades del resultado del sprint: el detalle tal como lo devuelve la API.
export type SprintResultsProps = {
  sprint: SprintDetail;
};

// Resultado congelado de un sprint cerrado (004/US5). Firma mínima para las pruebas de T065; la
// implementación corresponde a T070.
export function SprintResults(props: SprintResultsProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

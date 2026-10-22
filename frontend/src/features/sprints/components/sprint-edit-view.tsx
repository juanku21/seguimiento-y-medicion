import type { ReactNode } from "react";

// Propiedades de la vista de modificación: el identificador del sprint tomado de la ruta.
export type SprintEditViewProps = {
  sprintId: string;
};

// Vista de modificación de un sprint Planificado (004/US7). Firma mínima para las pruebas de T083;
// la implementación corresponde a T087.
export function SprintEditView(props: SprintEditViewProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

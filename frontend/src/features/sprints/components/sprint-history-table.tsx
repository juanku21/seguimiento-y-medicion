import type { ReactNode } from "react";

// Propiedades del historial de sprints cerrados: el proyecto cuyo historial se consulta.
export type SprintHistoryTableProps = {
  projectId: string;
};

// Historial de los sprints cerrados del proyecto (004/US6). Firma mínima para las pruebas de T075;
// la implementación corresponde a T080.
export function SprintHistoryTable(props: SprintHistoryTableProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

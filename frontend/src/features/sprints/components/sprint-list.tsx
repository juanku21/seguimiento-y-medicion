import type { ReactNode } from "react";

// Propiedades del listado de sprints: el proyecto cuyos sprints se consultan.
export type SprintListProps = {
  projectId: string;
};

// Listado de los sprints del proyecto (004/US6). Firma mínima para las pruebas de T075; la
// implementación corresponde a T080.
export function SprintList(props: SprintListProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

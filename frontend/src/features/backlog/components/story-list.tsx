import type { ReactNode } from "react";
import type { AppliedFilters } from "@/types/story";

// Propiedades del listado: el proyecto cuyo backlog se consulta y los filtros elegidos.
export type StoryListProps = {
  projectId: string;
  filters: AppliedFilters;
};

// Listado del backlog ordenado y filtrado (003/US2). Firma mínima de la tarea de pruebas T031; la
// implementación es T037.
export function StoryList({ projectId, filters }: StoryListProps): ReactNode {
  void projectId;
  void filters;
  throw new Error("not implemented");
}

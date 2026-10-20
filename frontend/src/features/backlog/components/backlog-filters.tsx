import type { ReactNode } from "react";
import type { AppliedFilters } from "@/types/story";

// Propiedades de los filtros: los valores elegidos y la acción que recibe la nueva selección.
export type BacklogFiltersProps = {
  value: AppliedFilters;
  onChange: (next: AppliedFilters) => void;
};

// Filtros del backlog por estado y por prioridad (003/US2). Firma mínima de la tarea de pruebas
// T031; la implementación es T037.
export function BacklogFilters({
  value,
  onChange,
}: BacklogFiltersProps): ReactNode {
  void value;
  void onChange;
  throw new Error("not implemented");
}

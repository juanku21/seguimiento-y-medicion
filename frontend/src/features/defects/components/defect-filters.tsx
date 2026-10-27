"use client";

import type { ReactElement } from "react";
import type { DefectFilterValues } from "@/types/defect";

// Propiedades de los filtros: el proyecto, los filtros elegidos y la acción que recibe la nueva
// selección completa.
export type DefectFiltersProps = {
  projectId: string;
  value: DefectFilterValues;
  onChange: (next: DefectFilterValues) => void;
};

// Firma mínima (T040): filtros del listado de defectos. La implementa T045.
export function DefectFilters(props: DefectFiltersProps): ReactElement {
  void props;
  throw new Error("not implemented");
}

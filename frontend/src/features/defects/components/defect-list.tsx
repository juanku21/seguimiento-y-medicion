"use client";

import type { ReactElement } from "react";
import type { DefectFilterValues } from "@/types/defect";

// Propiedades del listado: el proyecto consultado y los filtros elegidos.
export type DefectListProps = {
  projectId: string;
  filters: DefectFilterValues;
};

// Firma mínima (T040): listado de defectos del proyecto. La implementa T045.
export function DefectList(props: DefectListProps): ReactElement {
  void props;
  throw new Error("not implemented");
}

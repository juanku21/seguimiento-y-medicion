"use client";

import type { ReactNode } from "react";

// Propiedades de la vista del esfuerzo del proyecto: el proyecto tomado de la ruta.
export type ProjectEffortViewProps = {
  projectId: string;
};

// Esfuerzo del proyecto con filtros, lista y total (006/US4). Firma mínima de T057: la
// implementación corresponde a T062.
export function ProjectEffortView(props: ProjectEffortViewProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

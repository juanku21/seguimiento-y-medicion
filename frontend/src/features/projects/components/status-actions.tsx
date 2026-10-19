"use client";

import type { ReactNode } from "react";
import type { ProjectDetail } from "@/types/project";

// Propiedades de las acciones de estado: el proyecto tal como lo devuelve la API y la función que
// recibe el proyecto actualizado cuando el backend acepta la transición.
export type StatusActionsProps = {
  project: ProjectDetail;
  onStatusChange: (project: ProjectDetail) => void;
};

// Acciones para avanzar el estado del proyecto (002/US6). Firma mínima de T073: la implementación
// corresponde a T077.
export function StatusActions(props: StatusActionsProps): ReactNode {
  throw new Error("not implemented");
}

import type { ReactNode } from "react";
import type { ProjectDetail } from "@/types/project";

// Propiedades de la tarjeta de estado: el estado completo del proyecto tal como lo devuelve la API.
export type ProjectStatusCardProps = {
  project: ProjectDetail;
};

// Tarjeta con el estado del proyecto (002/US4). Firma mínima de la tarea de pruebas T057; la
// implementación es T060.
export function ProjectStatusCard({
  project,
}: ProjectStatusCardProps): ReactNode {
  void project;
  throw new Error("not implemented");
}

"use client";

import type { ProjectMemberView } from "@/types/project";

// Propiedades de la lista de integrantes: el proyecto, los integrantes tal como los devuelve la API
// y si quien consulta es el propietario, que es quien ve los correos y las acciones de
// administración (FR-030).
export type MemberListProps = {
  projectId: string;
  members: ProjectMemberView[];
  viewerIsOwner: boolean;
};

// Lista de integrantes del proyecto (002/US3). Firma mínima para las pruebas de T046: la
// implementación corresponde a T054.
export function MemberList(props: MemberListProps) {
  void props;
  return null;
}

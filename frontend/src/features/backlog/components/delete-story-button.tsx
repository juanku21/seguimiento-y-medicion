"use client";

import type { ReactNode } from "react";

// Propiedades del botón de eliminación: la historia a eliminar y el proyecto a cuyo backlog se
// regresa al eliminarla.
export type DeleteStoryButtonProps = {
  storyId: string;
  projectId: string;
};

// Botón de eliminación de una historia con confirmación (003/US6). Firma mínima para las pruebas de
// T068; la implementación corresponde a T072.
export function DeleteStoryButton(props: DeleteStoryButtonProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

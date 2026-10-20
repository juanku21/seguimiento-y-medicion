import type { ReactNode } from "react";

// Propiedades del formulario de historia: el proyecto en cuyo backlog se registra.
export type StoryFormProps = {
  projectId: string;
};

// Formulario de registro de una historia en el backlog (003/US1). Firma mínima de la tarea de
// pruebas T016; la implementación es T027.
export function StoryForm({ projectId }: StoryFormProps): ReactNode {
  void projectId;
  throw new Error("not implemented");
}

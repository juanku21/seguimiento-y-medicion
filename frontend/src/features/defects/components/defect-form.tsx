"use client";

import type { Defect } from "@/types/defect";

// Propiedades del formulario: el proyecto donde se registra y el aviso del alta exitosa.
export type DefectFormProps = {
  projectId: string;
  onCreated?: (defect: Defect) => void;
};

// Formulario de registro de un defecto (007/US1). La historia preseleccionada llega en el parámetro
// de consulta storyId.
export function DefectForm({ projectId, onCreated }: DefectFormProps) {
  void projectId;
  void onCreated;
  return null;
}

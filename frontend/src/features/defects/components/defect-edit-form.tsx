"use client";

import type { ReactElement } from "react";
import type { Defect } from "@/types/defect";

// Propiedades del formulario de corrección: el defecto vigente y el aviso de la corrección exitosa.
export type DefectEditFormProps = {
  defect: Defect;
  onUpdated?: (defect: Defect) => void;
};

// Formulario de corrección de los datos de un defecto (007/US4).
export function DefectEditForm(props: DefectEditFormProps): ReactElement {
  void props;
  throw new Error("not implemented");
}

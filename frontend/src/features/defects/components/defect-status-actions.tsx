"use client";

import type { Defect } from "@/types/defect";

// Propiedades de las acciones de estado: el defecto mostrado y onChanged, que recibe el defecto
// con su estado vigente para que la pantalla de detalle lo muestre.
export type DefectStatusActionsProps = {
  defect: Defect;
  onChanged: (defect: Defect) => void;
};

// Firma mínima de las acciones de estado del defecto (007/US2). Todavía no renderiza nada: las
// acciones de pasar a En progreso y resolver las implementa T037.
export function DefectStatusActions(props: DefectStatusActionsProps) {
  void props;
  return null;
}

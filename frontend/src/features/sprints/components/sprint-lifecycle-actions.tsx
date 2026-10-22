"use client";

import type { SprintDetail } from "@/types/sprint";

// Propiedades de las acciones del ciclo de vida: el sprint mostrado y onChanged, que recibe el
// SprintDetail devuelto por el backend para que la pantalla de detalle lo muestre.
export type SprintLifecycleActionsProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Firma mínima de las acciones del ciclo de vida del sprint (004/US3). Todavía no renderiza nada:
// la acción de iniciar la implementa T051.
export function SprintLifecycleActions(props: SprintLifecycleActionsProps) {
  void props;
  return null;
}

import type { ReactElement } from "react";
import type { RoundStatus } from "@/types/poker";

// Propiedades de las acciones del facilitador: si quien mira tiene el rol, el estado de la ronda
// vigente y la acción que revela la ronda.
export type FacilitatorActionsProps = {
  isFacilitator: boolean;
  roundStatus: RoundStatus;
  onReveal: () => void;
};

// Acciones reservadas al facilitador de la sesión (005/US3). Firma mínima para las pruebas de T049;
// la implementación llega en T055.
export function FacilitatorActions({
  isFacilitator,
  roundStatus,
  onReveal,
}: FacilitatorActionsProps): ReactElement | null {
  void isFacilitator;
  void roundStatus;
  void onReveal;
  throw new Error("not implemented");
}

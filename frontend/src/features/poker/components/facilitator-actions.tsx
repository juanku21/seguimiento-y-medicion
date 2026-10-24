import type { ReactElement } from "react";
import type { RoundStatus } from "@/types/poker";

// Propiedades de las acciones del facilitador: si quien mira tiene el rol, el estado de la ronda
// vigente, la acción que revela la ronda y la que inicia una ronda nueva (005/US4, T060).
export type FacilitatorActionsProps = {
  isFacilitator: boolean;
  roundStatus: RoundStatus;
  onReveal: () => void;
  onNewRound?: () => void;
};

// Acciones reservadas al facilitador de la sesión (005/US3). Es de presentación: solo al facilitador
// y con la ronda vigente en votación ofrece "Revelar", que pide la revelación manual con onReveal
// (FR-032, FR-034); con la ronda ya revelada no la ofrece (FR-026). Las reglas de la revelación las
// resuelve el backend.
export function FacilitatorActions({
  isFacilitator,
  roundStatus,
  onReveal,
}: FacilitatorActionsProps): ReactElement | null {
  if (!isFacilitator || roundStatus !== "voting") {
    return null;
  }
  return (
    <div className="flex flex-wrap gap-3">
      <button
        type="button"
        onClick={onReveal}
        className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300 dark:focus-visible:outline-zinc-100"
      >
        Revelar
      </button>
    </div>
  );
}

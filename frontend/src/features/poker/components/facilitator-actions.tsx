import { useState, type ReactElement } from "react";
import type { RoundStatus } from "@/types/poker";

// Propiedades de las acciones del facilitador: si quien mira tiene el rol, el estado de la ronda
// vigente, la acción que revela la ronda, la que inicia una ronda nueva (005/US4, T060) y la que
// cancela la sesión sin registrar estimación (005/US7, T080).
export type FacilitatorActionsProps = {
  isFacilitator: boolean;
  roundStatus: RoundStatus;
  onReveal: () => void;
  onNewRound?: () => void;
  onCancel?: () => void;
};

// Acciones reservadas al facilitador de la sesión (005/US3, 005/US4). Es de presentación y solo se
// muestra al facilitador: con la ronda vigente en votación ofrece "Revelar", que pide la revelación
// manual con onReveal (FR-032, FR-034), y con la ronda ya revelada ofrece "Nueva ronda", que pide la
// ronda siguiente con onNewRound (FR-026, FR-043, FR-046, FR-047). Con la ronda en votación o
// revelada ofrece además "Cancelar sesión" (005/US7), que primero pide confirmación y recién con
// "Confirmar cancelación" llama a onCancel; "Volver" cierra la pregunta sin cancelar (FR-058,
// FR-059). Las reglas las resuelve el backend.
export function FacilitatorActions({
  isFacilitator,
  roundStatus,
  onReveal,
  onNewRound,
  onCancel,
}: FacilitatorActionsProps): ReactElement | null {
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  if (!isFacilitator) {
    return null;
  }
  const buttonClassName =
    "rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300 dark:focus-visible:outline-zinc-100";
  const secondaryButtonClassName =
    "rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:border-zinc-700 dark:hover:bg-zinc-800 dark:focus-visible:outline-zinc-100";

  // Confirma la cancelación pedida: cierra la pregunta y pide la cancelación una sola vez.
  const handleConfirmCancel = () => {
    setConfirmingCancel(false);
    onCancel?.();
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-3">
        {roundStatus === "voting" && (
          <button type="button" onClick={onReveal} className={buttonClassName}>
            Revelar
          </button>
        )}
        {roundStatus === "revealed" && onNewRound !== undefined && (
          <button
            type="button"
            onClick={onNewRound}
            className={buttonClassName}
          >
            Nueva ronda
          </button>
        )}
        {onCancel !== undefined && !confirmingCancel && (
          <button
            type="button"
            onClick={() => setConfirmingCancel(true)}
            className={secondaryButtonClassName}
          >
            Cancelar sesión
          </button>
        )}
      </div>

      {/* Confirmación previa: la cancelación no se puede deshacer, así que se pregunta antes de
          pedirla. */}
      {onCancel !== undefined && confirmingCancel && (
        <div className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-4 dark:border-zinc-700">
          <p className="text-sm font-medium">
            ¿Cancelar la sesión sin registrar estimación?
          </p>
          <div className="flex flex-wrap gap-3">
            <button
              type="button"
              onClick={handleConfirmCancel}
              className={buttonClassName}
            >
              Confirmar cancelación
            </button>
            <button
              type="button"
              onClick={() => setConfirmingCancel(false)}
              className={secondaryButtonClassName}
            >
              Volver
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

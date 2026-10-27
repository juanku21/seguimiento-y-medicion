"use client";

import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import {
  changeDefectStatus,
  invalidTransitionStatus,
} from "@/features/defects/api";
import {
  primaryButtonClass,
  ResolveDefectForm,
} from "@/features/defects/components/resolve-defect-form";
import { allowedTransitions } from "@/features/defects/transitions";
import type { Defect, DefectStatusChange } from "@/types/defect";

// Propiedades de las acciones de estado: el defecto mostrado y onChanged, que recibe el defecto
// con su estado vigente para que la pantalla de detalle lo muestre.
export type DefectStatusActionsProps = {
  defect: Defect;
  onChanged: (defect: Defect) => void;
};

// Acciones de estado del defecto (007/US2 y US5). Las ofrece según las transiciones admitidas desde
// el estado vigente: "Pasar a En progreso" en un Abierto, "Resolver" en uno En progreso, que abre el
// formulario del sprint de resolución, y "Reabrir" en uno Resuelto. Con el 200 entrega el defecto recibido a la pantalla. El 400
// del sprint de resolución se muestra junto al selector y los demás rechazos como mensaje general;
// ante un 409 invalid_transition además entrega el defecto con el estado vigente informado, para que
// la pantalla ofrezca las acciones que corresponden (FR-018, RN-24).
export function DefectStatusActions({
  defect,
  onChanged,
}: DefectStatusActionsProps) {
  const [error, setError] = useState<string | null>(null);
  const [sprintError, setSprintError] = useState<string | undefined>();
  const [isResolving, setIsResolving] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const allowed = allowedTransitions(defect.status);

  // Envía el cambio de estado. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function submit(change: DefectStatusChange) {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    setSprintError(undefined);
    const result = await changeDefectStatus(defect.id, change);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setIsResolving(false);
      onChanged(result.data);
      return;
    }

    // El error del sprint de resolución va junto al selector; cualquier otro, como mensaje general.
    const field = result.error.fields.find(
      (item) => item.field === "resolvedSprintId",
    );
    if (field) {
      setSprintError(field.message);
      return;
    }
    setError(result.error.message);

    // Si el estado cambió mientras tanto, se refresca el defecto con el estado vigente.
    const currentStatus = invalidTransitionStatus(result.error);
    if (currentStatus !== null) {
      setIsResolving(false);
      onChanged({ ...defect, status: currentStatus });
    }
  }

  // Confirma la resolución; sin sprint elegido no se envía resolvedSprintId y decide el backend.
  function resolve(resolvedSprintId: string) {
    void submit(
      resolvedSprintId === ""
        ? { status: "resolved" }
        : { status: "resolved", resolvedSprintId },
    );
  }

  if (allowed.length === 0 && error === null) {
    return null;
  }

  return (
    <section className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}

      {/* Inicio del trabajo: solo en un defecto Abierto. */}
      {allowed.includes("in_progress") && (
        <div>
          <button
            type="button"
            onClick={() => void submit({ status: "in_progress" })}
            disabled={isSubmitting}
            className={primaryButtonClass}
          >
            Pasar a En progreso
          </button>
        </div>
      )}

      {/* Reapertura: solo en un defecto Resuelto; no pide datos, envía status open. */}
      {allowed.includes("open") && (
        <div>
          <button
            type="button"
            onClick={() => void submit({ status: "open" })}
            disabled={isSubmitting}
            className={primaryButtonClass}
          >
            Reabrir
          </button>
        </div>
      )}

      {/* Resolución: en un defecto En progreso, primero el botón y después el sprint de
          resolución, que no envía nada hasta que se confirma. */}
      {allowed.includes("resolved") && !isResolving && (
        <div>
          <button
            type="button"
            onClick={() => {
              setError(null);
              setIsResolving(true);
            }}
            className={primaryButtonClass}
          >
            Resolver
          </button>
        </div>
      )}
      {allowed.includes("resolved") && isResolving && (
        <ResolveDefectForm
          projectId={defect.projectId}
          error={sprintError}
          isSubmitting={isSubmitting}
          onConfirm={resolve}
          onCancel={() => {
            setSprintError(undefined);
            setIsResolving(false);
          }}
        />
      )}
    </section>
  );
}

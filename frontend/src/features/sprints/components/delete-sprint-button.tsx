"use client";

import { useId, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Alert } from "@/components/alert";
import { deleteSprint } from "@/features/sprints/api";
import type { SprintDetail } from "@/types/sprint";

// Propiedades del botón de eliminación: el sprint a eliminar, tal como lo devuelve la API.
export type DeleteSprintButtonProps = {
  sprint: SprintDetail;
};

// Clases del botón principal de la eliminación y del secundario de la confirmación.
const primaryButtonClass =
  "rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700 disabled:opacity-60";
const secondaryButtonClass =
  "rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-900 transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800";

// Botón de eliminación de un sprint Planificado (004/US7). Solo se ofrece en un sprint Planificado
// (FR-048) y, como la eliminación es definitiva, primero pide confirmación sin enviar nada. Con el
// 204 regresa al listado de sprints del proyecto reemplazando la entrada del historial, para que
// volver atrás no lleve al detalle de un sprint que ya no existe. Con un rechazo (sprint ya
// iniciado, esfuerzo registrado o sprint inexistente) muestra el mensaje del backend y vuelve a
// ofrecer la eliminación. Ante un 401 el cliente HTTP descarta la sesión.
export function DeleteSprintButton({ sprint }: DeleteSprintButtonProps) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  const submittingRef = useRef(false);
  const questionId = useId();

  // Envía la eliminación confirmada. La referencia descarta un segundo envío mientras el primero
  // sigue en curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function confirmDelete() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await deleteSprint(sprint.id);
    submittingRef.current = false;
    setIsSubmitting(false);
    setIsConfirming(false);

    if (result.ok) {
      router.replace(
        `/projects/${encodeURIComponent(sprint.projectId)}/sprints`,
      );
      return;
    }
    setError(result.error.message);
  }

  if (sprint.status !== "planned") {
    return null;
  }

  return (
    <section className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}

      {/* Primero el botón y después la confirmación, que no envía nada hasta que se confirma. */}
      {!isConfirming && (
        <div>
          <button
            type="button"
            onClick={() => {
              setError(null);
              setIsConfirming(true);
            }}
            className={primaryButtonClass}
          >
            Eliminar sprint
          </button>
        </div>
      )}
      {isConfirming && (
        <div
          aria-labelledby={questionId}
          className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
          role="group"
        >
          <p id={questionId} className="text-sm font-medium">
            ¿Eliminar este sprint?
          </p>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Sus historias vuelven al backlog. Esta acción no se puede deshacer.
          </p>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={confirmDelete}
              disabled={isSubmitting}
              className={primaryButtonClass}
            >
              Confirmar eliminación
            </button>
            <button
              type="button"
              onClick={() => setIsConfirming(false)}
              disabled={isSubmitting}
              className={secondaryButtonClass}
            >
              Cancelar
            </button>
          </div>
        </div>
      )}
    </section>
  );
}

"use client";

import { useId, useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { closeSprint, startSprint } from "@/features/sprints/api";
import type { ApiResult } from "@/lib/api-client";
import type { SprintDetail, SprintStoryItem } from "@/types/sprint";

// Propiedades de las acciones del ciclo de vida: el sprint mostrado y onChanged, que recibe el
// SprintDetail devuelto por el backend para que la pantalla de detalle lo muestre.
export type SprintLifecycleActionsProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Clases del botón principal de las acciones y del secundario de la confirmación.
const primaryButtonClass =
  "rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300";
const secondaryButtonClass =
  "rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-900 transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800";

// Arma el aviso de cuántas historias vuelven al backlog al cerrar: las que no están Completadas,
// en singular, plural o "Ninguna" si no vuelve ninguna.
function returningStoriesText(stories: SprintStoryItem[]): string {
  const count = stories.filter((story) => story.status !== "completed").length;
  if (count === 0) {
    return "Ninguna historia vuelve al backlog";
  }
  if (count === 1) {
    return "1 historia vuelve al backlog";
  }
  return `${count} historias vuelven al backlog`;
}

// Acciones del ciclo de vida del sprint (004/US3 y US5). En un sprint Planificado ofrece iniciarlo,
// sin depender de las historias ni de la fecha prevista (RN-02). En uno Activo ofrece cerrarlo,
// previa confirmación que informa cuántas historias vuelven al backlog. Un sprint Cerrado no ofrece
// ninguna acción. Con el 200 entrega el detalle a la pantalla y con un rechazo muestra el mensaje
// del backend. Ante un 401 el cliente HTTP descarta la sesión.
export function SprintLifecycleActions({
  sprint,
  onChanged,
}: SprintLifecycleActionsProps) {
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isConfirmingClose, setIsConfirmingClose] = useState(false);
  const submittingRef = useRef(false);
  const questionId = useId();

  // Envía la acción indicada. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function submit(action: () => Promise<ApiResult<SprintDetail>>) {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await action();
    submittingRef.current = false;
    setIsSubmitting(false);
    setIsConfirmingClose(false);

    if (result.ok) {
      onChanged(result.data);
      return;
    }
    setError(result.error.message);
  }

  if (sprint.status === "closed") {
    return null;
  }

  return (
    <section className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}

      {/* Inicio: solo en un sprint Planificado. */}
      {sprint.status === "planned" && (
        <div>
          <button
            type="button"
            onClick={() => submit(() => startSprint(sprint.id))}
            disabled={isSubmitting}
            className={primaryButtonClass}
          >
            Iniciar sprint
          </button>
        </div>
      )}

      {/* Cierre: en un sprint Activo, primero el botón y después la confirmación, que no envía
          nada hasta que se confirma. */}
      {sprint.status === "active" && !isConfirmingClose && (
        <div>
          <button
            type="button"
            onClick={() => {
              setError(null);
              setIsConfirmingClose(true);
            }}
            className={primaryButtonClass}
          >
            Cerrar sprint
          </button>
        </div>
      )}
      {sprint.status === "active" && isConfirmingClose && (
        <div
          aria-labelledby={questionId}
          className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
          role="group"
        >
          <p id={questionId} className="text-sm font-medium">
            ¿Cerrar este sprint?
          </p>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {returningStoriesText(sprint.stories)}
          </p>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => submit(() => closeSprint(sprint.id))}
              disabled={isSubmitting}
              className={primaryButtonClass}
            >
              Confirmar cierre
            </button>
            <button
              type="button"
              onClick={() => setIsConfirmingClose(false)}
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

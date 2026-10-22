"use client";

import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { startSprint } from "@/features/sprints/api";
import type { SprintDetail } from "@/types/sprint";

// Propiedades de las acciones del ciclo de vida: el sprint mostrado y onChanged, que recibe el
// SprintDetail devuelto por el backend para que la pantalla de detalle lo muestre.
export type SprintLifecycleActionsProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Acciones del ciclo de vida del sprint (004/US3). Solo en un sprint Planificado ofrece iniciarlo,
// sin depender de las historias ni de la fecha prevista (RN-02). Con el 200 entrega el detalle Activo
// a la pantalla y con un rechazo muestra el mensaje del backend. Ante un 401 el cliente HTTP
// descarta la sesión.
export function SprintLifecycleActions({
  sprint,
  onChanged,
}: SprintLifecycleActionsProps) {
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Envía el inicio del sprint. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleStart() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await startSprint(sprint.id);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      onChanged(result.data);
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
      <div>
        <button
          type="button"
          onClick={handleStart}
          disabled={isSubmitting}
          className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300"
        >
          Iniciar sprint
        </button>
      </div>
    </section>
  );
}

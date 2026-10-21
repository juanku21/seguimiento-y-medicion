"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { deleteStory } from "@/features/backlog/api";
import type { ApiError } from "@/lib/api-client";

// Propiedades del botón de eliminación: la historia a eliminar y el proyecto a cuyo backlog se
// regresa al eliminarla.
export type DeleteStoryButtonProps = {
  storyId: string;
  projectId: string;
};

// Botón de eliminación de una historia con confirmación (003/US6). El primer botón no envía nada:
// abre la confirmación, y solo "Confirmar eliminación" envía el DELETE. Con el 204 regresa al
// backlog con replace, para que volver atrás no lleve al detalle de una historia que ya no existe;
// con un rechazo muestra el mensaje y, si el backend los informa, cada motivo en el orden recibido,
// sin salir del detalle (FR-041 a FR-044, FR-048).
export function DeleteStoryButton({
  storyId,
  projectId,
}: DeleteStoryButtonProps) {
  const router = useRouter();
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const submittingRef = useRef(false);

  // Envía la eliminación confirmada. La referencia descarta un segundo envío mientras el primero
  // sigue en curso.
  async function handleConfirm() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setError(null);
    const result = await deleteStory(storyId);
    submittingRef.current = false;

    if (result.ok) {
      router.replace(`/projects/${projectId}/backlog`);
      return;
    }
    setConfirming(false);
    setError(result.error);
  }

  return (
    <section className="flex flex-col gap-3">
      {/* Rechazo de la eliminación: el mensaje general y, en el 409 por historial, la lista de
          motivos. */}
      {error && (
        <Alert>
          <p>{error.message}</p>
          {error.reasons && error.reasons.length > 0 && (
            <ul className="mt-2 flex list-disc flex-col gap-1 pl-5">
              {error.reasons.map((reason, index) => (
                <li key={index}>{reason}</li>
              ))}
            </ul>
          )}
        </Alert>
      )}

      {/* Confirmación previa: la eliminación es definitiva, así que se pregunta antes de enviar. */}
      {confirming ? (
        <div className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-4 dark:border-zinc-700">
          <p className="text-sm font-medium">¿Eliminar esta historia?</p>
          <Button onClick={handleConfirm}>Confirmar eliminación</Button>
          <button
            type="button"
            onClick={() => setConfirming(false)}
            className="inline-flex h-11 w-full items-center justify-center rounded-md border border-zinc-300 px-4 text-sm font-medium transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:border-zinc-700 dark:hover:bg-zinc-800"
          >
            Cancelar
          </button>
        </div>
      ) : (
        <Button onClick={() => setConfirming(true)}>Eliminar historia</Button>
      )}
    </section>
  );
}

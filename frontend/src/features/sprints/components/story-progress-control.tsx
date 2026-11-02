"use client";

import { buttonClassName } from "@/components/button";
import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { changeStoryStatus } from "@/features/sprints/api";
import type { SprintStoryItem, StoryProgressResponse } from "@/types/sprint";
import type { StoryStatus } from "@/types/story";

// Propiedades del control de avance: la historia comprometida en el sprint Activo y onChanged, que
// recibe el avance registrado por el backend.
export type StoryProgressControlProps = {
  story: SprintStoryItem;
  onChanged: (progress: StoryProgressResponse) => void;
};

// Transición admitida desde cada estado (RN-11) con el texto de su botón. No existe el salto de
// Pendiente a Completada ni la vuelta a Pendiente, así que la interfaz nunca los ofrece.
const NEXT_TRANSITION: Record<StoryStatus, { to: StoryStatus; label: string }> =
  {
    pending: { to: "in_progress", label: "Pasar a En progreso" },
    in_progress: { to: "completed", label: "Marcar como completada" },
    completed: { to: "in_progress", label: "Volver a En progreso" },
  };

// Control de avance de una historia del sprint Activo (004/US4). Ofrece un único botón con la
// transición admitida desde el estado actual y la envía sin confirmación. Con el 200 entrega el
// avance a la pantalla y con un rechazo muestra el mensaje del backend. Ante un 401 el cliente HTTP
// descarta la sesión.
export function StoryProgressControl({
  story,
  onChanged,
}: StoryProgressControlProps) {
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const transition = NEXT_TRANSITION[story.status];

  // Envía el avance. La referencia descarta un segundo envío mientras el primero sigue en curso, aun
  // antes de que el botón deshabilitado llegue a renderizarse.
  async function handleProgress() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await changeStoryStatus(story.storyId, transition.to);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      onChanged(result.data);
      return;
    }
    setError(result.error.message);
  }

  return (
    <div className="flex w-full flex-col gap-2 sm:w-auto sm:items-end">
      {error && <Alert>{error}</Alert>}
      <button
        type="button"
        onClick={handleProgress}
        disabled={isSubmitting}
        className={buttonClassName("secondary")}
      >
        {transition.label}
      </button>
    </div>
  );
}

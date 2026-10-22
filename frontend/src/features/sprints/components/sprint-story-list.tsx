"use client";

import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { removeSprintStories } from "@/features/sprints/api";
import { StoryProgressControl } from "@/features/sprints/components/story-progress-control";
import type { SprintDetail, StoryProgressResponse } from "@/types/sprint";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
  formatStoryPoints,
} from "@/utils/story-labels";

// Propiedades de la lista de historias del sprint: el sprint con sus historias y la función que
// recibe el detalle actualizado tras quitar historias o registrar su avance.
export type SprintStoryListProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Lista de las historias comprometidas en un sprint, con la acción de quitarlas (004/US2). Muestra
// cada historia en el orden recibido con su estado, sus Story Points y su prioridad. Solo en un
// sprint abierto ofrece quitar las historias que no están Completadas (RN-09, RN-21); con el 200
// entrega el detalle actualizado a la pantalla y con un rechazo muestra su mensaje. En un sprint
// Activo cada fila ofrece además el control de avance (004/US4). Ante un 401 el cliente HTTP descarta
// la sesión.
export function SprintStoryList({ sprint, onChanged }: SprintStoryListProps) {
  const [error, setError] = useState<string | null>(null);
  const submittingRef = useRef(false);
  const isOpen = sprint.status !== "closed";
  const isActive = sprint.status === "active";

  // Envía la baja de una historia. La referencia descarta un segundo envío mientras el primero sigue
  // en curso.
  async function handleRemove(storyId: string) {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setError(null);
    const result = await removeSprintStories(sprint.id, [storyId]);
    submittingRef.current = false;

    if (result.ok) {
      onChanged(result.data);
      return;
    }
    setError(result.error.message);
  }

  // Entrega a la pantalla el sprint con el estado y el completedAt de la historia avanzada
  // reemplazados por los que registró el backend; el resto del detalle se conserva.
  function handleProgress(progress: StoryProgressResponse) {
    onChanged({
      ...sprint,
      stories: sprint.stories.map((story) =>
        story.storyId === progress.storyId
          ? {
              ...story,
              status: progress.status,
              completedAt: progress.completedAt,
            }
          : story,
      ),
    });
  }

  // Sprint sin historias: se informa como estado normal, no como error.
  if (sprint.stories.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
        El sprint no tiene historias comprometidas.
      </p>
    );
  }

  return (
    <section className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}

      {/* Historias comprometidas, con el avance y la acción de quitar solo donde las reglas los admiten. */}
      <ul className="flex flex-col gap-2">
        {sprint.stories.map((story) => (
          <li
            key={story.storyId}
            className="flex items-center justify-between gap-4 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
          >
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium">{story.title}</p>
              <p className="text-xs text-zinc-600 dark:text-zinc-400">
                {STORY_STATUS_LABELS[story.status]} ·{" "}
                {formatStoryPoints(story.storyPoints, "SP")} · Prioridad{" "}
                {STORY_PRIORITY_LABELS[story.priority].toLowerCase()}
              </p>
            </div>
            <div className="flex shrink-0 items-start gap-2">
              {isActive && (
                <StoryProgressControl
                  story={story}
                  onChanged={handleProgress}
                />
              )}
              {isOpen && story.status !== "completed" && (
                <button
                  type="button"
                  onClick={() => handleRemove(story.storyId)}
                  className="shrink-0 rounded-md border border-zinc-300 px-3 py-2 text-sm font-medium transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 dark:border-zinc-700 dark:hover:bg-zinc-800"
                >
                  Quitar del sprint
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

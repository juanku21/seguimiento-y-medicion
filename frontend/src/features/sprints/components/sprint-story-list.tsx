"use client";

import { buttonClassName } from "@/components/button";
import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { removeSprintStories } from "@/features/sprints/api";
import { StoryProgressControl } from "@/features/sprints/components/story-progress-control";
import type {
  SprintDetail,
  SprintStoryItem,
  StoryProgressResponse,
} from "@/types/sprint";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
  formatStoryPoints,
} from "@/utils/story-labels";
import { STORY_OUTCOME_LABELS } from "@/utils/sprint-labels";

// Propiedades de la lista de historias del sprint: el sprint con sus historias y la función que
// recibe el detalle actualizado tras quitar historias o registrar su avance.
export type SprintStoryListProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Lista de las historias comprometidas en un sprint, con la acción de quitarlas (004/US2). Muestra
// cada historia en el orden recibido con su estado (en un sprint Cerrado, el resultado congelado al
// cierre, 004/US5), sus Story Points y su prioridad. Solo en un
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

  // Etiqueta de la historia: en un sprint Cerrado, el resultado congelado al cierre (FR-043); en un
  // sprint abierto, o si el cierre no registró resultado, su estado vigente.
  function storyLabel(story: SprintStoryItem): string {
    if (!isOpen && story.outcome) {
      return STORY_OUTCOME_LABELS[story.outcome];
    }
    return STORY_STATUS_LABELS[story.status];
  }

  // Sprint sin historias: se informa como estado normal, no como error.
  if (sprint.stories.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-border px-5 py-6 text-center text-sm text-foreground-muted">
        El sprint no tiene historias comprometidas.
      </p>
    );
  }

  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-lg font-semibold">Historias comprometidas</h3>
      {error && <Alert>{error}</Alert>}

      {/* Historias comprometidas, con el avance y la acción de quitar solo donde las reglas los admiten. */}
      <ul className="divide-y divide-border rounded-lg border border-border bg-surface">
        {sprint.stories.map((story) => (
          <li
            key={story.storyId}
            className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
          >
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium">{story.title}</p>
              <p className="text-xs text-foreground-muted">
                {storyLabel(story)},{" "}
                {formatStoryPoints(story.storyPoints, "SP")}, prioridad{" "}
                {STORY_PRIORITY_LABELS[story.priority].toLowerCase()}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap items-start gap-2">
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
                  className={buttonClassName("secondary")}
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

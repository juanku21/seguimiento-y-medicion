"use client";

import { useCallback, useId } from "react";
import { Alert } from "@/components/alert";
import { listStorySprints } from "@/features/sprints/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { formatDate } from "@/utils/format-date";
import {
  SPRINT_STATUS_LABELS,
  STORY_OUTCOME_LABELS,
} from "@/utils/sprint-labels";

// Propiedades del historial de sprints de una historia: la historia cuyo historial se consulta.
export type StorySprintHistoryProps = {
  storyId: string;
};

const MUTED_CLASS_NAME = "text-sm text-zinc-600 dark:text-zinc-400";

// Sprints en los que estuvo una historia y su resultado en cada uno (004/US6, FR-055). Pide el
// historial con un token vigente y lo muestra en el orden recibido; el resultado solo aparece en
// los sprints cerrados, porque en uno abierto todavía no lo hay. La historia inexistente o ajena
// llega como 404 y se muestra su mensaje; ante un 401 el cliente HTTP borra el token.
export function StorySprintHistory({ storyId }: StorySprintHistoryProps) {
  const headingId = useId();
  // Pide el historial con un token vigente y cada vez que cambia la historia.
  const load = useCallback(() => listStorySprints(storyId), [storyId]);
  const state = useAuthorizedQuery(load);

  // Contenido de la sección según el estado de la consulta.
  function renderContent() {
    if (state.status === "error") {
      return <Alert>{state.message}</Alert>;
    }
    if (state.status === "loading") {
      return (
        <p role="status" className={MUTED_CLASS_NAME}>
          Cargando sprints…
        </p>
      );
    }
    const { sprints } = state.data;
    if (sprints.length === 0) {
      return (
        <p className={MUTED_CLASS_NAME}>
          La historia todavía no estuvo en ningún sprint.
        </p>
      );
    }
    return (
      <ul className="flex flex-col gap-2">
        {sprints.map((sprint) => (
          <li
            key={sprint.sprintId}
            className="flex flex-col gap-1 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
          >
            <p className="text-sm font-medium">{sprint.sprintName}</p>
            <p className="text-xs text-zinc-600 dark:text-zinc-400">
              {SPRINT_STATUS_LABELS[sprint.sprintStatus]} ·{" "}
              {formatDate(sprint.startDate)} al {formatDate(sprint.endDate)}
              {sprint.outcome !== null &&
                ` · ${STORY_OUTCOME_LABELS[sprint.outcome]}`}
            </p>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold tracking-tight">
        Sprints en los que estuvo
      </h2>
      {renderContent()}
    </section>
  );
}

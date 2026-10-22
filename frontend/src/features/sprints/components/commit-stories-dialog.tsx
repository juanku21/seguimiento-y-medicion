"use client";

import { useEffect, useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { listStories } from "@/features/backlog/api";
import { assignSprintStories } from "@/features/sprints/api";
import { useRequireSession } from "@/hooks/use-session";
import type { ApiError } from "@/lib/api-client";
import type { SprintDetail } from "@/types/sprint";
import type { AppliedFilters, StoryListItem } from "@/types/story";
import { formatStoryPoints } from "@/utils/story-labels";

// Propiedades del diálogo de compromiso: el sprint abierto en el que se comprometen historias y la
// función que recibe el detalle actualizado que devuelve el backend.
export type CommitStoriesDialogProps = {
  sprint: SprintDetail;
  onCommitted: (sprint: SprintDetail) => void;
};

// Estado de la consulta del backlog: todavía sin respuesta, las historias recibidas o el mensaje de
// error.
type BacklogState =
  | { status: "loading" }
  | { status: "loaded"; stories: StoryListItem[] }
  | { status: "error"; message: string };

// Sin filtros, el listado de la 003 devuelve todo el backlog del proyecto.
const NO_FILTERS: AppliedFilters = { status: [], priority: [] };

// Diálogo para comprometer historias del backlog en un sprint (004/US2). Pide el backlog del
// proyecto y ofrece solo las historias listas para planificar, no Completadas y que no están ya en
// este sprint (RN-08). La admisión definitiva y la atomicidad del lote las resuelve el backend: con
// el 200 entrega el detalle actualizado a la pantalla y con un rechazo muestra el mensaje general y,
// en el 409 de historias no admitidas, el motivo de cada una junto a ella. Ante un 401 el cliente
// HTTP descarta la sesión.
export function CommitStoriesDialog({
  sprint,
  onCommitted,
}: CommitStoriesDialogProps) {
  const token = useRequireSession();
  const [backlog, setBacklog] = useState<BacklogState>({ status: "loading" });
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState<ApiError | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Pide el backlog del proyecto con un token vigente. La bandera descarta la respuesta si el
  // componente se desmontó o el pedido quedó viejo antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listStories(sprint.projectId, NO_FILTERS).then((result) => {
      if (!isCurrent) {
        return;
      }
      setBacklog(
        result.ok
          ? { status: "loaded", stories: result.data.stories }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, sprint.projectId]);

  // Marca o desmarca una historia del lote a comprometer.
  function toggle(storyId: string) {
    setSelected((current) =>
      current.includes(storyId)
        ? current.filter((id) => id !== storyId)
        : [...current, storyId],
    );
  }

  // Envía el lote elegido. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleCommit() {
    if (submittingRef.current || selected.length === 0) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await assignSprintStories(sprint.id, selected);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setSelected([]);
      onCommitted(result.data);
      return;
    }
    setError(result.error);
  }

  if (backlog.status === "error") {
    return <Alert>{backlog.message}</Alert>;
  }
  if (backlog.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando historias del backlog…
      </p>
    );
  }

  // Historias que se ofrecen (RN-08) y motivo de rechazo de cada una, si el backend lo informó.
  const committedIds = new Set(sprint.stories.map((story) => story.storyId));
  const offered = backlog.stories.filter(
    (story) =>
      story.readyToPlan &&
      story.status !== "completed" &&
      !committedIds.has(story.id),
  );
  const problems = new Map(
    (error?.stories ?? []).map((problem) => [problem.storyId, problem.message]),
  );

  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold tracking-tight">
        Comprometer historias del backlog
      </h2>
      {error && <Alert>{error.message}</Alert>}

      {offered.length === 0 ? (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          No hay historias listas para comprometer.
        </p>
      ) : (
        // Cada historia ofrecida es una casilla con su título como nombre; el motivo del rechazo
        // queda fuera de la etiqueta para no alterar ese nombre.
        <ul className="flex flex-col gap-2">
          {offered.map((story) => (
            <li
              key={story.id}
              className="flex flex-col gap-1 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
            >
              <div className="flex items-center justify-between gap-4">
                <label className="flex items-center gap-3 text-sm font-medium">
                  <input
                    type="checkbox"
                    checked={selected.includes(story.id)}
                    onChange={() => toggle(story.id)}
                    className="size-4"
                  />
                  <span>{story.title}</span>
                </label>
                <span className="shrink-0 text-xs text-zinc-600 dark:text-zinc-400">
                  {formatStoryPoints(story.storyPoints, "SP")}
                </span>
              </div>
              {problems.has(story.id) && (
                <p className="text-sm text-red-700 dark:text-red-300">
                  {problems.get(story.id)}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}

      <Button
        onClick={handleCommit}
        disabled={isSubmitting || selected.length === 0}
      >
        Comprometer historias
      </Button>
    </section>
  );
}

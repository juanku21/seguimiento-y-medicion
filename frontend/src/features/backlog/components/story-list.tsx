"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { listStories } from "@/features/backlog/api";
import { ReadinessBadge } from "@/features/backlog/components/readiness-badge";
import { useRequireSession } from "@/hooks/use-session";
import type { ApiError } from "@/lib/api-client";
import type {
  AppliedFilters,
  StoryListItem,
  StoryListResponse,
} from "@/types/story";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
} from "@/utils/story-labels";

// Propiedades del listado: el proyecto cuyo backlog se consulta y los filtros elegidos.
export type StoryListProps = {
  projectId: string;
  filters: AppliedFilters;
};

// Estado de la consulta: todavía sin respuesta, el listado recibido o el error devuelto.
type ListState =
  | { status: "loading" }
  | { status: "loaded"; data: StoryListResponse }
  | { status: "error"; error: ApiError };

const EMPTY_MESSAGE_CLASS_NAME =
  "rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400";

// Arma los Story Points de la historia; null es "Sin estimar" y el 0 es una estimación válida
// (FR-014, FR-015).
function formatStoryPoints(points: StoryListItem["storyPoints"]): string {
  return points === null ? "Sin estimar" : `${points} SP`;
}

// Arma la cantidad de criterios de aceptación en singular o plural.
function formatCriteriaCount(count: number): string {
  return count === 1 ? "1 criterio" : `${count} criterios`;
}

// Describe en español los filtros aplicados, para el mensaje de filtros sin coincidencias
// (FR-036).
function describeFilters(filters: AppliedFilters): string {
  const parts: string[] = [];
  if (filters.status.length > 0) {
    const labels = filters.status.map((value) => STORY_STATUS_LABELS[value]);
    parts.push(`Estado: ${labels.join(", ")}`);
  }
  if (filters.priority.length > 0) {
    const labels = filters.priority.map(
      (value) => STORY_PRIORITY_LABELS[value],
    );
    parts.push(`Prioridad: ${labels.join(", ")}`);
  }
  return parts.join("; ");
}

// Listado del backlog ordenado y filtrado (003/US2). Pide las historias del proyecto con los
// filtros elegidos y las muestra en el orden recibido: el orden, el filtrado y el motivo del vacío
// los resuelve el backend (FR-032 a FR-036). Los rechazos (400, 404) se muestran como error con su
// mensaje y el detalle por campo (FR-037, FR-046).
export function StoryList({ projectId, filters }: StoryListProps) {
  const token = useRequireSession();
  const [state, setState] = useState<ListState>({ status: "loading" });

  // Pide el listado con un token vigente y cada vez que cambian el proyecto o los filtros. La
  // bandera descarta la respuesta si llegó otra consulta o el componente se desmontó.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listStories(projectId, filters).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", data: result.data }
          : { status: "error", error: result.error },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, projectId, filters]);

  // Error de la consulta: el mensaje general y, si el backend lo informa, el detalle de cada campo.
  if (state.status === "error") {
    return (
      <Alert>
        <p>{state.error.message}</p>
        {state.error.fields.map((field) => (
          <p key={field.field}>{field.message}</p>
        ))}
      </Alert>
    );
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando historias…
      </p>
    );
  }

  // Vacíos: el backlog sin historias y los filtros sin coincidencias tienen mensajes distintos y
  // ninguno se presenta como error (FR-035, FR-036).
  const { stories, appliedFilters, emptyReason } = state.data;
  if (stories.length === 0) {
    return (
      <p className={EMPTY_MESSAGE_CLASS_NAME}>
        {emptyReason === "no_matches"
          ? `Ninguna historia coincide con los filtros (${describeFilters(appliedFilters)}).`
          : "El backlog todavía no tiene historias."}
      </p>
    );
  }

  // Una fila por historia, en el orden recibido, con prioridad y estado en español, Story Points,
  // cantidad de criterios y si está lista para planificar (FR-033). El título enlaza al detalle
  // de la historia (003/US3).
  return (
    <ul aria-label="Backlog" className="flex flex-col gap-3">
      {stories.map((story) => (
        <li
          key={story.id}
          className="flex flex-col gap-2 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-base font-medium break-words">
              <Link
                href={`/projects/${projectId}/backlog/${story.id}`}
                className="underline-offset-4 hover:underline"
              >
                {story.title}
              </Link>
            </h2>
            <ReadinessBadge ready={story.readyToPlan} />
          </div>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {STORY_PRIORITY_LABELS[story.priority]} ·{" "}
            {STORY_STATUS_LABELS[story.status]} ·{" "}
            {formatStoryPoints(story.storyPoints)} ·{" "}
            {formatCriteriaCount(story.acceptanceCriteriaCount)}
          </p>
        </li>
      ))}
    </ul>
  );
}

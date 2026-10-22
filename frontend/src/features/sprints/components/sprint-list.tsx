"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { listSprints } from "@/features/sprints/api";
import { useRequireSession } from "@/hooks/use-session";
import type { SprintListResponse } from "@/types/sprint";
import { formatDate } from "@/utils/format-date";
import { SPRINT_STATUS_LABELS } from "@/utils/sprint-labels";

// Propiedades del listado de sprints: el proyecto cuyos sprints se consultan.
export type SprintListProps = {
  projectId: string;
};

// Estado de la consulta: todavía sin respuesta, el listado recibido o el mensaje de error.
type ListState =
  | { status: "loading" }
  | { status: "loaded"; data: SprintListResponse }
  | { status: "error"; message: string };

// Arma la cantidad de historias en singular o plural.
function formatStoryCount(count: number): string {
  return count === 1 ? "1 historia" : `${count} historias`;
}

// Listado de los sprints del proyecto (004/US6). Pide todos los sprints, sin filtro de estado, y los
// muestra en el orden recibido: el orden y el motivo del vacío los resuelve el backend (FR-050,
// FR-052, FR-054). El proyecto inexistente o ajeno llega como 404 y se muestra solo su mensaje
// (FR-058); ante un 401 el cliente HTTP borra el token y la guardia lleva a /login.
export function SprintList({ projectId }: SprintListProps) {
  const token = useRequireSession();
  const [state, setState] = useState<ListState>({ status: "loading" });

  // Pide el listado con un token vigente y cada vez que cambia el proyecto. La bandera descarta la
  // respuesta si llegó otra consulta o el componente se desmontó.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listSprints(projectId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", data: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, projectId]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando sprints…
      </p>
    );
  }

  // Proyecto sin sprints: se informa como estado normal, no como error (FR-054).
  const { sprints } = state.data;
  if (sprints.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
        El proyecto todavía no tiene sprints.
      </p>
    );
  }

  // Una fila por sprint, en el orden recibido, con su estado en español, las fechas previstas, el
  // Sprint Goal y la cantidad de historias. El nombre enlaza al detalle del sprint.
  return (
    <ul aria-label="Sprints del proyecto" className="flex flex-col gap-3">
      {sprints.map((sprint) => (
        <li
          key={sprint.id}
          className="flex flex-col gap-1 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
        >
          <h2 className="text-base font-medium break-words">
            <Link
              href={`/projects/${projectId}/sprints/${sprint.id}`}
              className="underline-offset-4 hover:underline"
            >
              {sprint.name}
            </Link>
          </h2>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {SPRINT_STATUS_LABELS[sprint.status]} ·{" "}
            {formatDate(sprint.startDate)} al {formatDate(sprint.endDate)} ·{" "}
            {formatStoryCount(sprint.storyCount)}
          </p>
          <p className="text-sm break-words">{sprint.goal}</p>
        </li>
      ))}
    </ul>
  );
}

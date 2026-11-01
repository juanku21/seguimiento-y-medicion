"use client";

import { Badge, type BadgeTone } from "@/components/badge";
import type { SprintStatus } from "@/types/sprint";
import { LoadingState } from "@/components/loading-state";
import Link from "next/link";
import { useCallback } from "react";
import { Alert } from "@/components/alert";
import { listSprints } from "@/features/sprints/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { formatDate } from "@/utils/format-date";
import { SPRINT_STATUS_LABELS } from "@/utils/sprint-labels";

// Propiedades del listado de sprints: el proyecto cuyos sprints se consultan.
export type SprintListProps = {
  projectId: string;
};

// Arma la cantidad de historias en singular o plural.
function formatStoryCount(count: number): string {
  return count === 1 ? "1 historia" : `${count} historias`;
}

// Tono de cada estado: el texto de la insignia nombra el estado.
const STATUS_TONES: Record<SprintStatus, BadgeTone> = {
  planned: "neutral",
  active: "primary",
  closed: "success",
};

// Listado de los sprints del proyecto (004/US6). Pide todos los sprints, sin filtro de estado, y los
// muestra en el orden recibido: el orden y el motivo del vacío los resuelve el backend (FR-050,
// FR-052, FR-054). El proyecto inexistente o ajeno llega como 404 y se muestra solo su mensaje
// (FR-058); ante un 401 el cliente HTTP borra el token y la guardia lleva a /login.

export function SprintList({ projectId }: SprintListProps) {
  // Pide el listado con un token vigente y cada vez que cambia el proyecto.
  const load = useCallback(() => listSprints(projectId), [projectId]);
  const state = useAuthorizedQuery(load);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return <LoadingState>Cargando sprints…</LoadingState>;
  }

  // Proyecto sin sprints: se informa como estado normal, no como error (FR-054).
  const { sprints } = state.data;
  if (sprints.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-border px-5 py-6 text-center text-sm text-foreground-muted">
        El proyecto todavía no tiene sprints.
      </p>
    );
  }

  // Una fila por sprint, en el orden recibido, con su estado en español, las fechas previstas, el
  // Sprint Goal y la cantidad de historias. El nombre enlaza al detalle del sprint.
  return (
    <ul
      aria-label="Sprints del proyecto"
      className="divide-y divide-border rounded-lg border border-border bg-surface"
    >
      {sprints.map((sprint) => (
        <li
          key={sprint.id}
          className="flex flex-col gap-1.5 px-4 py-3 transition-colors hover:bg-surface-muted"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-base font-medium break-words">
              <Link
                href={`/projects/${projectId}/sprints/${sprint.id}`}
                className="underline-offset-4 hover:underline"
              >
                {sprint.name}
              </Link>
            </h2>
            <Badge tone={STATUS_TONES[sprint.status]}>
              {SPRINT_STATUS_LABELS[sprint.status]}
            </Badge>
          </div>
          <p className="text-sm text-foreground-muted">
            {formatDate(sprint.startDate)} al {formatDate(sprint.endDate)},{" "}
            {formatStoryCount(sprint.storyCount)}
          </p>
          <p className="text-sm break-words">{sprint.goal}</p>
        </li>
      ))}
    </ul>
  );
}

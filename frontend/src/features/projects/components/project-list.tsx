"use client";

import Link from "next/link";
import { LoadingState } from "@/components/loading-state";
import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { listProjects } from "@/features/projects/api";
import { useRequireSession } from "@/hooks/use-session";
import type { ProjectListResponse } from "@/types/project";
import { formatDate } from "@/utils/format-date";
import { PROJECT_STATUS_LABELS } from "@/utils/project-status";

// Estado de la consulta: todavía sin respuesta, el listado recibido o el mensaje de error.
type ListState =
  | { status: "loading" }
  | { status: "loaded"; data: ProjectListResponse }
  | { status: "error"; message: string };

// Arma la cantidad de integrantes en singular o plural.
function formatMemberCount(count: number): string {
  return count === 1 ? "1 integrante" : `${count} integrantes`;
}

// Listado de los proyectos de los que participa el titular de la sesión (002/US2). No recibe props:
// con la guardia de sesión obtiene el token vigente y pide el listado. El filtrado por membresía y
// el orden los resuelve el backend (FR-032, FR-033), así que se muestra lo recibido tal cual llega.
// Si la API rechaza la sesión, el cliente HTTP borra el token y la guardia lleva a /login (FR-016).
export function ProjectList() {
  const token = useRequireSession();
  const [state, setState] = useState<ListState>({ status: "loading" });

  // Pide el listado cada vez que hay un token vigente. La bandera descarta la respuesta si el
  // componente se desmontó o el token cambió antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listProjects().then((result) => {
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
  }, [token]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return <LoadingState>Cargando proyectos…</LoadingState>;
  }

  // Estado vacío: se muestra el mensaje que envía el backend, sin presentarlo como error (FR-034).
  const { projects, message } = state.data;
  if (projects.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-border px-5 py-6 text-center text-sm text-foreground-muted">
        {message}
      </p>
    );
  }

  // Un elemento por proyecto, en el orden recibido, con su estado en español, las fechas, la
  // cantidad de integrantes y la marca de propietario solo en los propios (FR-035, FR-036).
  return (
    <ul
      aria-label="Mis proyectos"
      className="grid gap-4 md:grid-cols-2 xl:grid-cols-3"
    >
      {projects.map((project) => (
        <li
          key={project.id}
          className="relative flex flex-col gap-4 rounded-lg border border-border bg-surface p-5 transition-colors hover:border-primary focus-within:border-primary"
        >
          <div className="flex items-start justify-between gap-3">
            <h2 className="text-lg font-semibold break-words">
              <Link
                href={`/projects/${project.id}`}
                className="rounded-sm after:absolute after:inset-0 after:rounded-lg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
              >
                {project.name}
              </Link>
            </h2>
            {project.isOwner && (
              <span className="shrink-0 rounded-full bg-primary-soft px-2.5 py-0.5 text-xs font-medium text-primary">
                Propietario
              </span>
            )}
          </div>
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <dt className="text-foreground-muted">Estado</dt>{" "}
              <dd className="font-medium">
                {PROJECT_STATUS_LABELS[project.status]}
              </dd>{" "}
            </div>
            <div>
              <dt className="text-foreground-muted">Equipo</dt>{" "}
              <dd className="font-medium">
                {formatMemberCount(project.memberCount)}
              </dd>{" "}
            </div>
            <div>
              <dt className="text-foreground-muted">Inicio</dt>{" "}
              <dd className="font-medium tabular-nums">
                <time dateTime={project.startDate}>
                  {formatDate(project.startDate)}
                </time>
              </dd>{" "}
            </div>
            <div>
              <dt className="text-foreground-muted">Fin previsto</dt>{" "}
              <dd className="font-medium tabular-nums">
                <time dateTime={project.plannedEndDate}>
                  {formatDate(project.plannedEndDate)}
                </time>
              </dd>{" "}
            </div>
          </dl>
        </li>
      ))}
    </ul>
  );
}

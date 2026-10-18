"use client";

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
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando proyectos…
      </p>
    );
  }

  // Estado vacío: se muestra el mensaje que envía el backend, sin presentarlo como error (FR-034).
  const { projects, message } = state.data;
  if (projects.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
        {message}
      </p>
    );
  }

  // Un elemento por proyecto, en el orden recibido, con su estado en español, las fechas, la
  // cantidad de integrantes y la marca de propietario solo en los propios (FR-035, FR-036).
  return (
    <ul aria-label="Mis proyectos" className="flex flex-col gap-3">
      {projects.map((project) => (
        <li
          key={project.id}
          className="flex flex-col gap-2 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-base font-medium break-words">
              {project.name}
            </h2>
            {project.isOwner && (
              <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                Propietario
              </span>
            )}
          </div>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            {PROJECT_STATUS_LABELS[project.status]} · Inicio:{" "}
            <time dateTime={project.startDate}>
              {formatDate(project.startDate)}
            </time>{" "}
            · Fin previsto:{" "}
            <time dateTime={project.plannedEndDate}>
              {formatDate(project.plannedEndDate)}
            </time>{" "}
            · {formatMemberCount(project.memberCount)}
          </p>
        </li>
      ))}
    </ul>
  );
}

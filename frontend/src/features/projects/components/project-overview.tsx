"use client";

import { buttonClassName } from "@/components/button";
import { LoadingState } from "@/components/loading-state";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getProjectDetail } from "@/features/projects/api";
import { MemberList } from "@/features/projects/components/member-list";
import { ProjectStatusCard } from "@/features/projects/components/project-status-card";
import { StatusActions } from "@/features/projects/components/status-actions";
import { useRequireSession } from "@/hooks/use-session";
import type { ProjectDetail } from "@/types/project";

// Propiedades de la vista del proyecto: el identificador tomado de la ruta.
export type ProjectOverviewProps = {
  projectId: string;
};

const SECTION_LINK_CLASS_NAME =
  "flex min-h-12 items-center justify-center rounded-lg border border-border bg-surface px-3 text-center text-sm font-medium transition-colors hover:border-primary hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus";

// Estado de la consulta: todavía sin respuesta, el estado del proyecto recibido o el mensaje de
// error.
type OverviewState =
  | { status: "loading" }
  | { status: "loaded"; project: ProjectDetail }
  | { status: "error"; message: string };

// Indica si quien consulta es el propietario. La vista de estado no informa la identidad de quien
// consulta, pero el backend incluye los correos de los integrantes solo cuando consulta el
// propietario (FR-030), y el propietario siempre es integrante, así que su respuesta trae al menos
// un correo. Solo decide qué acciones se ofrecen: los permisos los vuelve a validar el backend.
export function isViewerOwner(project: ProjectDetail): boolean {
  return project.members.some((member) => member.email !== undefined);
}

// Vista del proyecto en una sola pantalla (002/US4, SC-003): pide el estado una sola vez y reparte
// la respuesta entre la tarjeta de estado, los enlaces al dashboard (009), al backlog (003), a los sprints (004) y al
// esfuerzo (006), a los defectos (007), las acciones de cambio de estado (002/US6) y la lista de integrantes, que para el
// propietario ya incluye el formulario de alta (002/US3). El 404 llega con el mensaje del backend,
// que no revela si el proyecto existe (FR-049); ante un 401 el cliente HTTP borra el token y la
// guardia lleva a /login (FR-016).
export function ProjectOverview({ projectId }: ProjectOverviewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<OverviewState>({ status: "loading" });

  // Pide el estado cada vez que hay un token vigente o cambia el proyecto. La bandera descarta la
  // respuesta si el componente se desmontó o el pedido quedó viejo antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getProjectDetail(projectId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", project: result.data }
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
    return <LoadingState>Cargando proyecto…</LoadingState>;
  }

  // Estado y equipo del proyecto, ambos a partir de la misma respuesta. Solo el propietario ve
  // las acciones de administración, y ninguna se ofrece si el proyecto es de solo lectura: en ese
  // caso la tarjeta muestra la fecha de finalización real (FR-018, US4-2). El proyecto que devuelve
  // un cambio de estado reemplaza al vigente (002/US6). Los enlaces al backlog (003), a los
  // sprints (004) y al esfuerzo (006) y a los defectos (007) se ofrecen a todo integrante, propietario o no.
  const { project } = state;
  const viewerIsOwner = isViewerOwner(project);
  return (
    <div className="flex flex-col gap-8">
      <ProjectStatusCard project={project} />
      <nav
        aria-label="Secciones del proyecto"
        className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6"
      >
        <Link
          href={`/projects/${project.id}/dashboard`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Dashboard
        </Link>
        <Link
          href={`/projects/${project.id}/reports`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Reportes
        </Link>
        <Link
          href={`/projects/${project.id}/backlog`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Ver backlog
        </Link>
        <Link
          href={`/projects/${project.id}/sprints`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Ver sprints
        </Link>
        <Link
          href={`/projects/${project.id}/effort`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Esfuerzo
        </Link>
        <Link
          href={`/projects/${project.id}/defects`}
          className={SECTION_LINK_CLASS_NAME}
        >
          Defectos
        </Link>
      </nav>
      {viewerIsOwner && !project.readOnly && (
        <div className="flex flex-col gap-3 sm:flex-row">
          <Link
            href={`/projects/${project.id}/edit`}
            className={buttonClassName("secondary")}
          >
            Editar proyecto
          </Link>
          <StatusActions
            project={project}
            onStatusChange={(updated) =>
              setState({ status: "loaded", project: updated })
            }
          />
        </div>
      )}
      <section
        aria-labelledby="members-heading"
        className="flex flex-col gap-4"
      >
        <h2 id="members-heading" className="text-lg font-semibold">
          Integrantes
        </h2>
        <MemberList
          projectId={project.id}
          members={project.members}
          viewerIsOwner={viewerIsOwner}
          readOnly={project.readOnly}
        />
      </section>
    </div>
  );
}

"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getProjectDetail } from "@/features/projects/api";
import { ProjectForm } from "@/features/projects/components/project-form";
import { isViewerOwner } from "@/features/projects/components/project-overview";
import { useRequireSession } from "@/hooks/use-session";
import type { ProjectDetail } from "@/types/project";

// Mensaje para quien es integrante pero no propietario; repite el que el backend devuelve con el
// 403 de una acción de administración (FR-050).
const NOT_OWNER_MESSAGE =
  "Solo el propietario del proyecto puede realizar esta acción.";

// Propiedades de la vista de edición: el identificador tomado de la ruta.
export type ProjectEditViewProps = {
  projectId: string;
};

// Estado de la consulta: todavía sin respuesta, el proyecto vigente recibido o el mensaje de error.
type EditViewState =
  | { status: "loading" }
  | { status: "loaded"; project: ProjectDetail }
  | { status: "error"; message: string };

// Vista de modificación del proyecto (002/US5): pide el proyecto vigente y, solo si quien consulta
// es el propietario, monta el ProjectForm en modo edición con esos valores. El 404 llega con el
// mensaje del backend, que no revela si el proyecto existe (FR-049); ante un 401 el cliente HTTP
// borra el token y la guardia lleva a /login (FR-016). Ocultar el formulario no reemplaza el
// permiso: el backend igual rechaza con 403 la modificación de quien no es propietario.
export function ProjectEditView({ projectId }: ProjectEditViewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<EditViewState>({ status: "loading" });

  // Pide el proyecto cada vez que hay un token vigente o cambia el proyecto. La bandera descarta la
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
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando proyecto…
      </p>
    );
  }

  // Formulario de edición solo para el propietario; el resto de los integrantes ve el aviso.
  if (!isViewerOwner(state.project)) {
    return <Alert>{NOT_OWNER_MESSAGE}</Alert>;
  }
  return <ProjectForm project={state.project} />;
}

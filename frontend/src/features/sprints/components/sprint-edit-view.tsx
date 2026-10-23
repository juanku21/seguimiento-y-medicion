"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getSprint } from "@/features/sprints/api";
import { SprintForm } from "@/features/sprints/components/sprint-form";
import { useRequireSession } from "@/hooks/use-session";
import type { SprintDetail } from "@/types/sprint";

// Propiedades de la vista de modificación: el identificador del sprint tomado de la ruta.
export type SprintEditViewProps = {
  sprintId: string;
};

// Estado de la consulta: todavía sin respuesta, el sprint recibido o el mensaje de error.
type EditState =
  | { status: "loading" }
  | { status: "loaded"; sprint: SprintDetail }
  | { status: "error"; message: string };

// Motivos por los que un sprint ya iniciado no ofrece la modificación (FR-016), con los mismos
// textos que usa el backend al rechazarla.
const NOT_PLANNED_MESSAGE = "Solo se puede modificar un sprint Planificado.";
const CLOSED_MESSAGE = "El sprint está cerrado y no admite cambios.";

// Vista de modificación de un sprint (004/US7): pide el sprint con un token vigente y, solo si está
// Planificado, muestra el formulario precargado con sus valores. En un sprint Activo o Cerrado
// indica por qué no se puede modificar, y el sprint inexistente o eliminado llega como 404 y se
// muestra solo su mensaje. Ante un 401 el cliente HTTP borra el token y la guardia lleva a /login.
export function SprintEditView({ sprintId }: SprintEditViewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<EditState>({ status: "loading" });

  // Pide el sprint cada vez que hay un token vigente o cambia el sprint. La bandera descarta la
  // respuesta si el componente se desmontó o el pedido quedó viejo antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getSprint(sprintId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", sprint: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, sprintId]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando sprint…
      </p>
    );
  }

  // Solo el sprint Planificado se modifica; los demás muestran el motivo sin formulario.
  const { sprint } = state;
  if (sprint.status === "active") {
    return <Alert>{NOT_PLANNED_MESSAGE}</Alert>;
  }
  if (sprint.status === "closed") {
    return <Alert>{CLOSED_MESSAGE}</Alert>;
  }
  return <SprintForm sprint={sprint} />;
}

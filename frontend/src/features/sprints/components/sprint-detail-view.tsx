"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getSprint } from "@/features/sprints/api";
import { CommitStoriesDialog } from "@/features/sprints/components/commit-stories-dialog";
import { SprintLifecycleActions } from "@/features/sprints/components/sprint-lifecycle-actions";
import { SprintResults } from "@/features/sprints/components/sprint-results";
import { SprintSchedule } from "@/features/sprints/components/sprint-schedule";
import { SprintStoryList } from "@/features/sprints/components/sprint-story-list";
import { useRequireSession } from "@/hooks/use-session";
import type { SprintDetail } from "@/types/sprint";
import { formatDate } from "@/utils/format-date";
import { SPRINT_STATUS_LABELS } from "@/utils/sprint-labels";

// Propiedades de la vista de detalle: el identificador del sprint tomado de la ruta.
export type SprintDetailViewProps = {
  sprintId: string;
};

// Estado de la consulta: todavía sin respuesta, el sprint recibido o el mensaje de error.
type DetailState =
  | { status: "loading" }
  | { status: "loaded"; sprint: SprintDetail }
  | { status: "error"; message: string };

// Vista del detalle de un sprint (004/US2, US3, US5 y US6): pide el sprint con un token vigente y
// muestra sus datos con los días restantes del sprint Activo, las acciones del ciclo de vida, el
// resultado congelado si está cerrado, la lista de historias comprometidas y, mientras el sprint
// está abierto, el diálogo para comprometer más. Cada cambio reemplaza el sprint mostrado por el
// detalle que devuelve el backend.
// El sprint inexistente o ajeno llega como 404 y se muestra solo su mensaje; ante un 401 el cliente
// HTTP borra el token y la guardia lleva a /login.
export function SprintDetailView({ sprintId }: SprintDetailViewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<DetailState>({ status: "loading" });

  // Pide el detalle cada vez que hay un token vigente o cambia el sprint. La bandera descarta la
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

  const { sprint } = state;
  const showSprint = (updated: SprintDetail) =>
    setState({ status: "loaded", sprint: updated });

  return (
    <div className="flex flex-col gap-8">
      {/* Datos del sprint: nombre, estado, Sprint Goal, fechas previstas y, si está Activo, los
          días restantes o vencidos. */}
      <section className="flex flex-col gap-1">
        <h2 className="text-xl font-semibold tracking-tight">{sprint.name}</h2>
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          {SPRINT_STATUS_LABELS[sprint.status]} · {formatDate(sprint.startDate)}{" "}
          al {formatDate(sprint.endDate)}
        </p>
        <p className="text-sm">{sprint.goal}</p>
        <SprintSchedule sprint={sprint} />
      </section>

      <SprintLifecycleActions sprint={sprint} onChanged={showSprint} />

      <SprintResults sprint={sprint} />

      <SprintStoryList sprint={sprint} onChanged={showSprint} />

      {sprint.status !== "closed" && (
        <CommitStoriesDialog sprint={sprint} onCommitted={showSprint} />
      )}
    </div>
  );
}

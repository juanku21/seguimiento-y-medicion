"use client";

import { useCallback } from "react";
import { Alert } from "@/components/alert";
import { getSprint } from "@/features/sprints/api";
import { SprintSchedule } from "@/features/sprints/components/sprint-schedule";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { formatDate } from "@/utils/format-date";

// Propiedades del panel: el id del sprint activo del proyecto, o null si no hay ninguno.
interface ActiveSprintPanelProps {
  activeSprintId: string | null;
}

// Contenido con sprint: consulta el detalle con una carga estabilizada por el id del sprint y
// muestra la carga, el error o los datos. Los días restantes o vencidos los entrega la 004 tal cual,
// sin restar fechas en el cliente.
function ActiveSprintContent({ sprintId }: { sprintId: string }) {
  const load = useCallback(() => getSprint(sprintId), [sprintId]);
  const state = useAuthorizedQuery(load);

  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando sprint activo…
      </p>
    );
  }
  if (state.status === "error") {
    return <Alert>No se pudo cargar el sprint activo.</Alert>;
  }

  const sprint = state.data;
  return (
    <div className="flex flex-col gap-1">
      <h3 className="text-base font-medium">{sprint.name}</h3>
      <p className="text-sm">{sprint.goal}</p>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Inicio: {formatDate(sprint.startDate)}
      </p>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Fin: {formatDate(sprint.endDate)}
      </p>
      <SprintSchedule sprint={sprint} />
    </div>
  );
}

// Panel del sprint activo del dashboard (009/US1): una región con el nombre, el Sprint Goal, las
// fechas y los días restantes o vencidos. Sin sprint activo lo informa sin consultar a la API.
export function ActiveSprintPanel({ activeSprintId }: ActiveSprintPanelProps) {
  return (
    <section
      aria-labelledby="active-sprint-heading"
      className="flex flex-col gap-2"
    >
      <h2 id="active-sprint-heading" className="text-lg font-semibold">
        Sprint activo
      </h2>
      {activeSprintId === null ? (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          No hay ningún sprint en curso.
        </p>
      ) : (
        <ActiveSprintContent sprintId={activeSprintId} />
      )}
    </section>
  );
}

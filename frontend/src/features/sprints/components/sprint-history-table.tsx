"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { listSprints } from "@/features/sprints/api";
import { useRequireSession } from "@/hooks/use-session";
import type { SprintListResponse, SprintStatus } from "@/types/sprint";
import { formatDate, formatInstant } from "@/utils/format-date";

// Propiedades del historial de sprints cerrados: el proyecto cuyo historial se consulta.
export type SprintHistoryTableProps = {
  projectId: string;
};

// Estado de la consulta: todavía sin respuesta, el listado recibido o el mensaje de error.
type HistoryState =
  | { status: "loading" }
  | { status: "loaded"; data: SprintListResponse }
  | { status: "error"; message: string };

// Filtro del historial: solo los sprints cerrados. Es una constante del módulo para que el efecto no
// vuelva a consultar en cada render.
const CLOSED_ONLY: SprintStatus[] = ["closed"];

// Encabezados de las columnas del historial, en el orden en que se muestran.
const COLUMNS = [
  "Sprint",
  "Sprint Goal",
  "Inicio previsto",
  "Fin previsto",
  "Inicio real",
  "Cierre real",
  "Historias planificadas",
  "Historias completadas",
  "Story Points comprometidos al inicio",
  "Story Points planificados al cierre",
  "Story Points completados",
];

const CELL_CLASS_NAME = "px-3 py-2 align-top";

// Muestra la fecha de un instante real en la zona horaria del navegador, o una raya si falta.
function formatReal(instant: string | null, timeZone: string): string {
  return instant === null ? "—" : formatInstant(instant, timeZone);
}

// Historial de los sprints cerrados del proyecto (004/US6, FR-051). Pide solo los cerrados y los
// muestra en el orden recibido con sus cuatro fechas y los totales congelados al cierre, tal como
// los devuelve el backend. Los vacíos (proyecto sin sprints o sin cerrados) se informan como estado
// normal; un rechazo se muestra con su mensaje y ante un 401 el cliente HTTP borra el token.
export function SprintHistoryTable({ projectId }: SprintHistoryTableProps) {
  const token = useRequireSession();
  const [state, setState] = useState<HistoryState>({ status: "loading" });

  // Pide los sprints cerrados con un token vigente y cada vez que cambia el proyecto. La bandera
  // descarta la respuesta si llegó otra consulta o el componente se desmontó.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    listSprints(projectId, CLOSED_ONLY).then((result) => {
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
        Cargando historial…
      </p>
    );
  }

  // Vacíos: el proyecto sin sprints y el proyecto sin cerrados tienen mensajes distintos y ninguno
  // se presenta como error (FR-054).
  const { sprints, emptyReason } = state.data;
  if (sprints.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-zinc-300 px-5 py-6 text-center text-sm text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
        {emptyReason === "no_sprints"
          ? "El proyecto todavía no tiene sprints."
          : "Todavía no hay sprints cerrados."}
      </p>
    );
  }

  // Una fila por sprint cerrado, con las fechas previstas como vienen y las reales en la zona
  // horaria del navegador; los totales son los de la instantánea.
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return (
    <div className="overflow-x-auto rounded-md border border-zinc-300 dark:border-zinc-700">
      <table className="w-full text-left text-sm">
        <caption className="px-3 py-2 text-left font-medium">
          Historial de sprints cerrados
        </caption>
        <thead className="bg-zinc-50 text-xs text-zinc-600 dark:bg-zinc-900 dark:text-zinc-400">
          <tr>
            {COLUMNS.map((column) => (
              <th key={column} scope="col" className={CELL_CLASS_NAME}>
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="tabular-nums">
          {sprints.map((sprint) => (
            <tr
              key={sprint.id}
              className="border-t border-zinc-200 dark:border-zinc-800"
            >
              <td className={`${CELL_CLASS_NAME} font-medium`}>
                {sprint.name}
              </td>
              <td className={CELL_CLASS_NAME}>{sprint.goal}</td>
              <td className={CELL_CLASS_NAME}>
                {formatDate(sprint.startDate)}
              </td>
              <td className={CELL_CLASS_NAME}>{formatDate(sprint.endDate)}</td>
              <td className={CELL_CLASS_NAME}>
                {formatReal(sprint.startedAt, timeZone)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {formatReal(sprint.closedAt, timeZone)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {sprint.results?.plannedStoryCount ?? "—"}
              </td>
              <td className={CELL_CLASS_NAME}>
                {sprint.results?.completedStoryCount ?? "—"}
              </td>
              <td className={CELL_CLASS_NAME}>
                {sprint.results?.committedStoryPoints ?? "—"}
              </td>
              <td className={CELL_CLASS_NAME}>
                {sprint.results?.plannedStoryPoints ?? "—"}
              </td>
              <td className={CELL_CLASS_NAME}>
                {sprint.results?.completedStoryPoints ?? "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

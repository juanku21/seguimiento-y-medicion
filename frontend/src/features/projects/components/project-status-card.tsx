import type { ReactNode } from "react";
import type { ProjectDetail } from "@/types/project";
import { formatDate } from "@/utils/format-date";
import { PROJECT_STATUS_LABELS } from "@/utils/project-status";
import { STORY_STATUS_LABELS } from "@/utils/story-labels";

// Propiedades de la tarjeta de estado: el estado completo del proyecto tal como lo devuelve la API.
export type ProjectStatusCardProps = {
  project: ProjectDetail;
};

// Formato de números en español (coma decimal), para el factor y las horas estimadas.
const numberFormat = new Intl.NumberFormat("es-AR");

// Etiquetas de los estados del backlog indexables por la clave que envía el backend; una clave
// desconocida no tiene etiqueta y se muestra tal cual llega (003, FR-026).
const backlogStateLabels: Readonly<Record<string, string | undefined>> =
  STORY_STATUS_LABELS;

// Arma la cantidad de historias en singular o plural.
function formatStoryCount(count: number): string {
  return count === 1 ? "1 historia" : `${count} historias`;
}

// Fila de la lista de datos: un rótulo y su valor.
function DataRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs font-medium tracking-wide text-zinc-500 uppercase dark:text-zinc-400">
        {label}
      </dt>
      <dd className="text-sm text-zinc-900 dark:text-zinc-100">{children}</dd>
    </div>
  );
}

// Tarjeta con el estado del proyecto (002/US4). Es de presentación: recibe el ProjectDetail que la
// pantalla pide una sola vez y no hace pedidos. Los totales y el reparto del backlog los calcula el
// backend (FR-042 a FR-045), así que se muestran tal cual llegan; los estados del backlog se
// muestran con las etiquetas en español de la feature 003.
export function ProjectStatusCard({
  project,
}: ProjectStatusCardProps): ReactNode {
  const { backlog } = project;
  const stateEntries = Object.entries(backlog.countsByState);
  const totalStories = stateEntries.reduce((sum, [, count]) => sum + count, 0);

  return (
    <section
      aria-label="Estado del proyecto"
      className="flex flex-col gap-5 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700"
    >
      {/* Encabezado: nombre, estado en español, marca de solo lectura y descripción. */}
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold break-words">{project.name}</h2>
          <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
            {PROJECT_STATUS_LABELS[project.status]}
          </span>
          {project.readOnly && (
            <span className="rounded-full border border-zinc-300 px-2.5 py-0.5 text-xs font-medium text-zinc-600 dark:border-zinc-700 dark:text-zinc-400">
              Proyecto de solo lectura
            </span>
          )}
        </div>
        {project.description && (
          <p className="text-sm break-words text-zinc-600 dark:text-zinc-400">
            {project.description}
          </p>
        )}
      </header>

      {/* Datos generales: fechas, factor, propietario y sprint activo. Sin sprint activo se indica
          de forma explícita, sin presentarlo como error (FR-043). */}
      <dl className="grid gap-4 sm:grid-cols-2">
        <DataRow label="Inicio">
          <time dateTime={project.startDate}>
            {formatDate(project.startDate)}
          </time>
        </DataRow>
        <DataRow label="Finalización prevista">
          <time dateTime={project.plannedEndDate}>
            {formatDate(project.plannedEndDate)}
          </time>
        </DataRow>
        {project.actualEndDate && (
          <DataRow label="Finalización real">
            <time dateTime={project.actualEndDate}>
              {formatDate(project.actualEndDate)}
            </time>
          </DataRow>
        )}
        <DataRow label="Factor">
          {numberFormat.format(project.hoursPerStoryPoint)} horas por Story
          Point
        </DataRow>
        <DataRow label="Propietario">{project.owner.fullName}</DataRow>
        <DataRow label="Sprint activo">
          {project.hasActiveSprint && project.activeSprint
            ? project.activeSprint.name
            : "No hay sprint activo"}
        </DataRow>
      </dl>

      {/* Backlog: total de historias, reparto por estado y totales estimados. Un proyecto sin
          historias muestra los contadores en cero (FR-044). */}
      <div className="flex flex-col gap-3 border-t border-zinc-200 pt-4 dark:border-zinc-800">
        <h3 className="text-sm font-medium">
          Backlog: {formatStoryCount(totalStories)} en total
        </h3>
        {stateEntries.length > 0 && (
          <ul
            aria-label="Historias por estado"
            className="flex flex-wrap gap-2"
          >
            {stateEntries.map(([state, count]) => (
              <li
                key={state}
                className="rounded-md bg-zinc-100 px-3 py-1 text-sm text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
              >
                {backlogStateLabels[state] ?? state}:{" "}
                <span className="font-medium tabular-nums">{count}</span>
              </li>
            ))}
          </ul>
        )}
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          Estimado:{" "}
          <span className="font-medium text-zinc-900 tabular-nums dark:text-zinc-100">
            {backlog.estimatedStoryPoints} Story Points
          </span>{" "}
          ·{" "}
          <span className="font-medium text-zinc-900 tabular-nums dark:text-zinc-100">
            {numberFormat.format(backlog.estimatedHours)} horas
          </span>
        </p>
      </div>
    </section>
  );
}

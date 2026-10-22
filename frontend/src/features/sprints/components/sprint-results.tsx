import { useId, type ReactNode } from "react";
import type {
  SprintDetail,
  SprintResults as SprintResultsData,
} from "@/types/sprint";
import { formatInstant } from "@/utils/format-date";

// Propiedades del resultado del sprint: el detalle tal como lo devuelve la API.
export type SprintResultsProps = {
  sprint: SprintDetail;
};

// Formato de números en español (coma decimal), para el factor al cierre.
const numberFormat = new Intl.NumberFormat("es-AR");

// Arma la diferencia de alcance (planificados al cierre menos comprometidos al inicio) con signo:
// "+16" si se sumó alcance, "-5" si se quitó y "0" si no cambió.
function scopeDifference(results: SprintResultsData): string {
  const difference = results.plannedStoryPoints - results.committedStoryPoints;
  return difference > 0 ? `+${difference}` : String(difference);
}

// Fila de la lista de datos: un rótulo y su valor.
function DataRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs font-medium tracking-wide text-zinc-500 uppercase dark:text-zinc-400">
        {label}
      </dt>
      <dd className="text-sm text-zinc-900 tabular-nums dark:text-zinc-100">
        {children}
      </dd>
    </div>
  );
}

// Resultado congelado de un sprint cerrado (004/US5). Es de presentación y de solo lectura: muestra
// los totales de la instantánea tal como los calculó el backend al cerrar (FR-043, FR-045), la
// diferencia de alcance (SC-009), el factor copiado, el momento real de cierre en la zona horaria
// del navegador y quién cerró. Mientras el sprint no está cerrado no hay resultado y no muestra nada.
export function SprintResults({ sprint }: SprintResultsProps): ReactNode {
  const headingId = useId();
  const { results } = sprint;
  if (results === null) {
    return null;
  }
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

  return (
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-4 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700"
    >
      <h3 id={headingId} className="text-lg font-semibold tracking-tight">
        Resultado del sprint
      </h3>
      <dl className="grid gap-4 sm:grid-cols-2">
        <DataRow label="Story Points comprometidos al inicio">
          {results.committedStoryPoints}
        </DataRow>
        <DataRow label="Story Points planificados al cierre">
          {results.plannedStoryPoints}
        </DataRow>
        <DataRow label="Diferencia de alcance">
          {scopeDifference(results)}
        </DataRow>
        <DataRow label="Story Points completados">
          {results.completedStoryPoints}
        </DataRow>
        <DataRow label="Historias planificadas">
          {results.plannedStoryCount}
        </DataRow>
        <DataRow label="Historias completadas">
          {results.completedStoryCount}
        </DataRow>
        <DataRow label="Factor al cierre">
          {sprint.hoursPerStoryPoint === null
            ? "—"
            : `${numberFormat.format(sprint.hoursPerStoryPoint)} horas por Story Point`}
        </DataRow>
        <DataRow label="Cierre real">
          {sprint.closedAt === null ? (
            "—"
          ) : (
            <time dateTime={sprint.closedAt}>
              {formatInstant(sprint.closedAt, timeZone)}
            </time>
          )}
        </DataRow>
        <DataRow label="Cerrado por">
          {sprint.closedBy?.fullName ?? "—"}
        </DataRow>
      </dl>
    </section>
  );
}

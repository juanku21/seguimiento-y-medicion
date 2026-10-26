import type { ReactNode } from "react";
import type { EffortEntry } from "@/types/effort";
import { formatHours } from "@/utils/effort-format";
import { formatDate } from "@/utils/format-date";

// Propiedades de la lista de registros de esfuerzo de una historia, en el orden de la API.
export type EffortEntryListProps = {
  entries: EffortEntry[];
};

// Lista de registros de esfuerzo de una historia (006/US2), en el orden recibido (FR-035). Cada
// registro muestra fecha, autor, actividad, horas y sprint, o "Sin sprint" si no tiene. Sin
// registros muestra un aviso informativo, que no es un error (FR-034).
export function EffortEntryList({ entries }: EffortEntryListProps): ReactNode {
  if (entries.length === 0) {
    return (
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Todavía no hay esfuerzo registrado.
      </p>
    );
  }

  return (
    <ul aria-label="Registros de esfuerzo" className="flex flex-col gap-3">
      {entries.map((entry) => (
        <li
          key={entry.id}
          className="flex flex-col gap-1 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="tabular-nums">{formatDate(entry.entryDate)}</span>
            <span className="font-medium tabular-nums">
              {formatHours(entry.hours)}
            </span>
          </div>
          <span className="break-words">{entry.activity}</span>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-zinc-600 dark:text-zinc-400">
            <span>{entry.author.fullName}</span>
            <span>{entry.sprint ? entry.sprint.name : "Sin sprint"}</span>
          </div>
        </li>
      ))}
    </ul>
  );
}

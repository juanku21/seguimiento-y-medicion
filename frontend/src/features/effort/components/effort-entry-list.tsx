import type { ReactNode } from "react";
import { EffortEntryRow } from "@/features/effort/components/effort-entry-row";
import type { EffortEntry } from "@/types/effort";

// Propiedades de la lista de registros de esfuerzo de una historia, en el orden de la API.
// currentUserId identifica al titular de la sesión para ofrecer la corrección y la eliminación de
// sus registros, y onChanged avisa que una de ellas se completó (006/US3).
export type EffortEntryListProps = {
  entries: EffortEntry[];
  currentUserId?: string;
  onChanged?: () => void;
};

// Indica si quien tiene la sesión puede corregir o eliminar el registro: solo su autor y mientras no
// sea definitivo (FR-018, FR-020, FR-022). Sin sesión identificada no se ofrece ninguna acción.
function canModify(entry: EffortEntry, currentUserId?: string): boolean {
  return (
    currentUserId !== undefined &&
    entry.author.id === currentUserId &&
    !entry.definitive
  );
}

// Lista de registros de esfuerzo de una historia (006/US2), en el orden recibido (FR-035). Cada
// registro muestra fecha, autor, actividad, horas y sprint, o "Sin sprint" si no tiene, y los
// propios no definitivos se pueden corregir o eliminar desde su fila (006/US3). Sin registros
// muestra un aviso informativo, que no es un error (FR-034).
export function EffortEntryList({
  entries,
  currentUserId,
  onChanged,
}: EffortEntryListProps): ReactNode {
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
        <EffortEntryRow
          key={entry.id}
          entry={entry}
          canModify={canModify(entry, currentUserId)}
          onChanged={onChanged}
        />
      ))}
    </ul>
  );
}

import type { ReactNode } from "react";
import type { EffortComparisonRow } from "@/types/effort";
import {
  formatHours,
  formatPercent,
  formatSignedHours,
  formatStoryPoints,
} from "@/utils/effort-format";

// Propiedades de la tabla de comparación del proyecto: las historias en el orden que las devuelve la
// API.
export type EffortComparisonTableProps = {
  stories: EffortComparisonRow[];
};

const COLUMNS = [
  "Historia",
  "Story Points",
  "Horas estimadas",
  "Horas reales",
  "Diferencia",
  "Diferencia porcentual",
] as const;

const CELL_CLASS_NAME = "px-3 py-2 text-right tabular-nums";

// Comparación entre estimado y real de todas las historias del proyecto (006/US5), una fila por
// historia en el orden del backlog que resuelve el backend. Cada celda usa el mismo formato que
// EffortComparisonCard; un proyecto con todas sus historias sin estimar se muestra completo con
// "No calculable", sin aviso de error (FR-050, FR-051, FR-055).
export function EffortComparisonTable({
  stories,
}: EffortComparisonTableProps): ReactNode {
  return (
    <div className="overflow-x-auto">
      <table
        aria-label="Comparación estimado contra real por historia"
        className="w-full border-collapse text-sm"
      >
        <thead>
          <tr className="border-b border-zinc-300 dark:border-zinc-700">
            {COLUMNS.map((column, index) => (
              <th
                key={column}
                scope="col"
                className={`px-3 py-2 font-medium text-zinc-600 dark:text-zinc-400 ${index === 0 ? "text-left" : "text-right"}`}
              >
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {stories.map((story) => (
            <tr
              key={story.storyId}
              className="border-b border-zinc-200 dark:border-zinc-800"
            >
              <th
                scope="row"
                className="px-3 py-2 text-left font-normal break-words"
              >
                {story.title}
              </th>
              <td className={CELL_CLASS_NAME}>
                {formatStoryPoints(story.storyPoints)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {formatHours(story.estimatedHours)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {formatHours(story.actualHours)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {formatSignedHours(story.deviationHours)}
              </td>
              <td className={CELL_CLASS_NAME}>
                {formatPercent(story.deviationPercent)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

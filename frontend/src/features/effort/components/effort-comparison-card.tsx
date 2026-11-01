import type { ReactNode } from "react";
import type { EffortComparisonRow } from "@/types/effort";
import {
  formatHours,
  formatPercent,
  formatSignedHours,
  formatStoryPoints,
} from "@/utils/effort-format";

// Propiedades de la comparación de una historia: la fila tal como la devuelve la API.
export type EffortComparisonCardProps = {
  comparison: EffortComparisonRow;
};

// Comparación entre estimado y real de una historia (006/US5). Muestra los valores tal como los
// calcula el backend: "Sin estimar" distingue la historia sin Story Points de la de 0 (FR-055), la
// diferencia y el porcentaje llevan signo y todo null se muestra como "No calculable" (FR-050,
// FR-051).
export function EffortComparisonCard({
  comparison,
}: EffortComparisonCardProps): ReactNode {
  const items = [
    ["Story Points", formatStoryPoints(comparison.storyPoints)],
    ["Horas estimadas", formatHours(comparison.estimatedHours)],
    ["Horas reales", formatHours(comparison.actualHours)],
    ["Diferencia", formatSignedHours(comparison.deviationHours)],
    ["Diferencia porcentual", formatPercent(comparison.deviationPercent)],
  ] as const;

  return (
    <dl
      aria-label="Comparación estimado contra real"
      className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-lg border border-border bg-surface px-4 py-3 text-sm"
    >
      {items.map(([label, value]) => (
        <div key={label} className="contents">
          <dt className="text-foreground-muted">{label}</dt>
          <dd className="text-right font-medium tabular-nums">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

import { readinessLabel } from "@/utils/story-labels";

// Propiedades de la insignia: si la historia está lista para planificar.
export type ReadinessBadgeProps = {
  ready: boolean;
};

// Insignia con la condición de lista para planificar que comparten la fila del listado y la tarjeta
// de detalle; verde cuando está lista y gris cuando no.
export function ReadinessBadge({ ready }: ReadinessBadgeProps) {
  return (
    <span
      className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
        ready
          ? "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200"
          : "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
      }`}
    >
      {readinessLabel(ready)}
    </span>
  );
}

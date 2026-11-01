import type { ReactNode } from "react";
import type { Finality } from "@/types/metrics";

// Texto y estilo de cada marca; la finalidad definitiva no lleva marca.
const BADGES: Partial<Record<Finality, { label: string; className: string }>> =
  {
    partial: { label: "Parcial", className: "bg-warning-soft text-warning" },
    provisional: {
      label: "Provisoria",
      className: "bg-primary-soft text-primary",
    },
  };

// Marca visual de la finalidad de una métrica: nada para definitive o null.
export function FinalityBadge({
  finality,
}: {
  finality: Finality | null;
}): ReactNode {
  const badge = finality ? BADGES[finality] : undefined;
  if (!badge) {
    return null;
  }
  return (
    <span
      className={`rounded px-1.5 py-0.5 text-xs font-medium ${badge.className}`}
    >
      {badge.label}
    </span>
  );
}

import type { ReactNode } from "react";
import type { Finality } from "@/types/metrics";

// Firma mínima: la implementación pertenece a la tarea T016.
export function FinalityBadge({ finality }: { finality: Finality | null }): ReactNode {
  void finality;
  throw new Error("not implemented");
}

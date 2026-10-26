import type { ReactNode } from "react";
import type { EffortEntry } from "@/types/effort";

// Propiedades de la lista de registros de esfuerzo de una historia, en el orden de la API.
export type EffortEntryListProps = {
  entries: EffortEntry[];
};

// Lista de registros de esfuerzo de una historia (006/US2). Pendiente de implementar en T039.
export function EffortEntryList(props: EffortEntryListProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

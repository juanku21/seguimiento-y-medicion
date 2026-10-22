"use client";

import type { SprintDetail } from "@/types/sprint";

// Propiedades de la lista de historias del sprint: el sprint con sus historias y la función que
// recibe el detalle actualizado tras quitar historias.
export type SprintStoryListProps = {
  sprint: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
};

// Lista de las historias comprometidas en un sprint, con la acción de quitarlas (004/US2). Firma
// mínima de T030: todavía no renderiza nada; la implementación corresponde a T038.
export function SprintStoryList(props: SprintStoryListProps) {
  void props;
  return null;
}

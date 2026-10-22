"use client";

import type { SprintDetail } from "@/types/sprint";

// Propiedades del diálogo de compromiso: el sprint abierto en el que se comprometen historias y la
// función que recibe el detalle actualizado que devuelve el backend.
export type CommitStoriesDialogProps = {
  sprint: SprintDetail;
  onCommitted: (sprint: SprintDetail) => void;
};

// Diálogo para comprometer historias del backlog en un sprint (004/US2). Firma mínima de T030:
// todavía no renderiza nada; la implementación corresponde a T038.
export function CommitStoriesDialog(props: CommitStoriesDialogProps) {
  void props;
  return null;
}

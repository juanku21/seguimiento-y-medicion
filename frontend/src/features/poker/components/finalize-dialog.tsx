import type { ReactElement } from "react";
import type { ScaleStoryPoints } from "@/types/poker";

// Propiedades del diálogo de registro: la acción que registra el valor acordado elegido.
export type FinalizeDialogProps = {
  onFinalize: (storyPoints: ScaleStoryPoints) => void;
};

// Diálogo del facilitador para registrar la estimación acordada (005/US5). Firma mínima para las
// pruebas de T064; la implementación llega en T069.
export function FinalizeDialog({
  onFinalize,
}: FinalizeDialogProps): ReactElement {
  void onFinalize;
  throw new Error("not implemented");
}

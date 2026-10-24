import type { ReactElement } from "react";

// Propiedades del botón para tomar el rol de facilitador: el momento a partir del cual el rol queda
// disponible según la instantánea (null si el facilitador está conectado) y la acción que pide el
// rol.
export type TakeFacilitatorButtonProps = {
  facilitatorAvailableAt: string | null;
  onTake: () => void;
};

// Botón "Tomar el rol" de la sala de Planning Poker (005/US6). Firma mínima para las pruebas de
// T073; la implementación llega en T077.
export function TakeFacilitatorButton(
  props: TakeFacilitatorButtonProps,
): ReactElement | null {
  void props;
  throw new Error("not implemented");
}

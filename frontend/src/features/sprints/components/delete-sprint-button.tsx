import type { ReactNode } from "react";
import type { SprintDetail } from "@/types/sprint";

// Propiedades del botón de eliminación: el sprint a eliminar, tal como lo devuelve la API.
export type DeleteSprintButtonProps = {
  sprint: SprintDetail;
};

// Botón de eliminación de un sprint Planificado con confirmación (004/US7). Firma mínima para las
// pruebas de T083; la implementación corresponde a T087.
export function DeleteSprintButton(props: DeleteSprintButtonProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

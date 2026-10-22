import type { ReactNode } from "react";
import type { SprintDetail } from "@/types/sprint";

// Propiedades de los días restantes: el detalle del sprint tal como lo devuelve la API.
export type SprintScheduleProps = {
  sprint: SprintDetail;
};

// Días restantes o vencidos del sprint Activo (004/US6). Firma mínima para las pruebas de T075; la
// implementación corresponde a T080.
export function SprintSchedule(props: SprintScheduleProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

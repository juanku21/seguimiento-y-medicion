import type { ReactNode } from "react";
import type { MemberEffort } from "@/types/effort";

// Propiedades del desglose del esfuerzo de una historia: el total y una línea por persona, tal como
// los devuelve la API.
export type EffortBreakdownProps = {
  totalHours: number;
  byMember: MemberEffort[];
};

// Total de horas de la historia y desglose por persona (006/US2). Pendiente de implementar en T039.
export function EffortBreakdown(props: EffortBreakdownProps): ReactNode {
  void props;
  throw new Error("not implemented");
}

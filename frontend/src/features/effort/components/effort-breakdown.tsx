import type { ReactNode } from "react";
import type { MemberEffort } from "@/types/effort";
import { formatHours } from "@/utils/effort-format";

// Propiedades del desglose del esfuerzo de una historia: el total y una línea por persona, tal como
// los devuelve la API.
export type EffortBreakdownProps = {
  totalHours: number;
  byMember: MemberEffort[];
};

// Total de horas de la historia y desglose por persona (006/US2). Muestra los valores tal como los
// calcula y ordena el backend, incluidas las personas que ya no son integrantes (FR-032); sin
// registros, el total es 0 h y la lista queda vacía (FR-034).
export function EffortBreakdown({
  totalHours,
  byMember,
}: EffortBreakdownProps): ReactNode {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-base font-semibold">
        Total: {formatHours(totalHours)}
      </p>
      <ul aria-label="Esfuerzo por integrante" className="flex flex-col gap-2">
        {byMember.map((member) => (
          <li
            key={member.user.id}
            className="flex items-center justify-between gap-3 text-sm"
          >
            <span className="min-w-0 break-words">{member.user.fullName}</span>
            <span className="font-medium tabular-nums">
              {formatHours(member.hours)}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

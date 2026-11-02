import type { ReactNode } from "react";
import type {
  SprintDetail,
  SprintSchedule as SprintScheduleData,
} from "@/types/sprint";

// Propiedades de los días restantes: el detalle del sprint tal como lo devuelve la API.
export type SprintScheduleProps = {
  sprint: SprintDetail;
};

// Arma el texto del cronograma: los días restantes, contando el día de fin, o el vencido en
// palabras, nunca como un número negativo (RN-23). Cada caso usa el singular con un solo día.
function describeSchedule(schedule: SprintScheduleData): string | null {
  if (schedule.remainingDays !== null) {
    const days = schedule.remainingDays;
    return days === 1 ? "1 día restante" : `${days} días restantes`;
  }
  if (schedule.overdueDays !== null) {
    const days = schedule.overdueDays;
    return days === 1 ? "Vencido hace 1 día" : `Vencido hace ${days} días`;
  }
  return null;
}

// Días restantes o vencidos del sprint Activo (004/US6, FR-053). Es de presentación: muestra el
// cronograma tal como lo calculó el backend en la zona horaria del sistema. Un sprint Planificado o
// Cerrado llega sin cronograma y no muestra nada.
export function SprintSchedule({ sprint }: SprintScheduleProps): ReactNode {
  if (sprint.schedule === null) {
    return null;
  }
  const text = describeSchedule(sprint.schedule);
  if (text === null) {
    return null;
  }
  return <p className="text-sm font-medium tabular-nums">{text}</p>;
}

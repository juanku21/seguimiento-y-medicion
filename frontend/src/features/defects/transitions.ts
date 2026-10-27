import type { DefectStatus } from "@/types/defect";

// Copia en el cliente del mapa de transiciones del backend (research.md sección 8, RN-12), con las
// transiciones de 007/US2 (Abierto → En progreso y En progreso → Resuelto) y de 007/US5 (Resuelto →
// Abierto, la reapertura). La interfaz lo usa para
// decidir qué acciones ofrecer; el backend lo vuelve a validar.
const DEFECT_TRANSITIONS: Record<DefectStatus, DefectStatus[]> = {
  open: ["in_progress"],
  in_progress: ["resolved"],
  resolved: ["open"],
  discarded: [],
};

// Devuelve los estados a los que puede pasar un defecto desde su estado vigente, como una copia
// para que quien la recibe no altere el mapa.
export function allowedTransitions(current: DefectStatus): DefectStatus[] {
  return [...DEFECT_TRANSITIONS[current]];
}

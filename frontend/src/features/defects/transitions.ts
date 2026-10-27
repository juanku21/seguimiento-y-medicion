import type { DefectStatus } from "@/types/defect";

// Firma mínima de la copia en el cliente del mapa de transiciones del backend (007/US2,
// research.md sección 8). Todavía no devuelve destinos: el mapa lo implementa T037.
export function allowedTransitions(current: DefectStatus): DefectStatus[] {
  void current;
  throw new Error("not implemented");
}

import { describe, expect, it } from "vitest";
import {
  DEFECT_CONFLICT_LABELS,
  DEFECT_EMPTY_REASON_LABELS,
  DEFECT_SEVERITY_LABELS,
  DEFECT_STATUS_LABELS,
} from "@/utils/defect-labels";

// Pruebas unitarias de las etiquetas en español de los valores que el backend envía en inglés
// (T013; research.md sección 5, Principio VIII). Es cobertura de presentación sin escenario propio:
// el describe no lleva el prefijo 007/USn.

describe("Etiquetas en español de los defectos", () => {
  // Las cuatro severidades se muestran en español.
  it("traduce las cuatro severidades", () => {
    expect(DEFECT_SEVERITY_LABELS).toEqual({
      critical: "Crítica",
      high: "Alta",
      medium: "Media",
      low: "Baja",
    });
  });

  // Los cuatro estados se muestran en español.
  it("traduce los cuatro estados", () => {
    expect(DEFECT_STATUS_LABELS).toEqual({
      open: "Abierto",
      in_progress: "En progreso",
      resolved: "Resuelto",
      discarded: "Descartado",
    });
  });

  // Los dos motivos de lista vacía se muestran como mensajes en español.
  it("traduce no_defects y no_matches", () => {
    expect(DEFECT_EMPTY_REASON_LABELS).toEqual({
      no_defects: "Todavía no hay defectos registrados.",
      no_matches: "Ningún defecto cumple los filtros elegidos.",
    });
  });

  // Los cinco códigos de conflicto se muestran con un título en español.
  it("traduce los cinco códigos de conflicto", () => {
    expect(DEFECT_CONFLICT_LABELS).toEqual({
      read_only: "Proyecto de solo lectura",
      no_detection_sprint: "Sin sprint de detección disponible",
      invalid_transition: "Transición no admitida",
      defect_discarded: "Defecto descartado",
      links_frozen: "Vínculos congelados",
    });
  });
});

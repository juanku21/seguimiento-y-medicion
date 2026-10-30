import { describe, expect, it } from "vitest";
import { explainNotCalculable } from "@/utils/metric-explanations";

// Pruebas unitarias de las explicaciones de "no calculable" (T010; FR-014; research.md sección 6).

describe("Explicación de métricas no calculables", () => {
  // Cada métrica con causa propia explica su motivo.
  it("explica el motivo de las métricas con causa propia", () => {
    expect(explainNotCalculable("velocity")).toBe(
      "Todavía no hay sprints cerrados.",
    );
    expect(explainNotCalculable("deviationPercent")).toBe(
      "Las horas estimadas son 0.",
    );
    expect(explainNotCalculable("storyCompletionPercent")).toBe(
      "No hay historias planificadas.",
    );
  });

  // Cualquier otra clave usa el texto genérico.
  it.each(["storyPoints", "hours", "deviationHours", "defects"] as const)(
    "usa el texto genérico para %s",
    (kind) => {
      expect(explainNotCalculable(kind)).toBe(
        "No se puede calcular con los datos actuales.",
      );
    },
  );
});

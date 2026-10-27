import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { EffortComparisonCard } from "@/features/effort/components/effort-comparison-card";
import type { EffortComparisonRow } from "@/types/effort";

// Pruebas de la comparación entre estimado y real de una historia (T065). El componente es de
// presentación: recibe la fila tal como la devuelve GET /api/v1/stories/{storyId}/effort-comparison
// (EffortComparisonRow de contracts/openapi.yaml), que ya calcula el backend (T068). Acá se verifica
// cómo se muestra cada caso, no el cálculo.
//
// Contrato que fijan estas pruebas para EffortComparisonCard (T070):
// - Recibe comparison (EffortComparisonRow).
// - Muestra una lista de descripción (dl) con aria-label "Comparación estimado contra real",
//   con un término (dt) seguido de su valor (dd) para "Story Points", "Horas estimadas",
//   "Horas reales", "Diferencia" y "Diferencia porcentual".
// - Story Points se muestran como "<n> SP"; null se muestra como "Sin estimar", de modo que una
//   historia de 0 Story Points ("0 SP") se distingue de una sin estimar (FR-055).
// - Las horas estimadas y reales se muestran con formatHours ("30 h", "0 h").
// - La diferencia se muestra con signo, como el porcentaje: "+" si es positiva, el signo menos
//   tipográfico (U+2212) si es negativa y sin signo en 0 ("+6 h", "−6 h", "0 h").
// - La diferencia porcentual se muestra con formatPercent ("+20,00 %", "−20,00 %", "0,00 %").
// - Todo valor null se muestra como "No calculable", nunca como 0 (FR-050, FR-051).

const minus = "−";
const notCalculable = "No calculable";

// Fila base: historia de 5 Story Points con factor 6 y 36 horas reales (escenario 1 de US5).
const base: EffortComparisonRow = {
  storyId: "3f2a1b0c-9d8e-4f7a-b6c5-d4e3f2a1b0c9",
  title: "Alta de clientes",
  storyPoints: 5,
  estimatedHours: 30,
  actualHours: 36,
  deviationHours: 6,
  deviationPercent: 20,
};

// Devuelve el texto del valor (dd) que sigue al término (dt) indicado dentro de la comparación.
function valueOf(label: string): string {
  const list = screen.getByLabelText("Comparación estimado contra real", {
    selector: "dl",
  });
  const term = within(list).getByText(label, { selector: "dt" });
  const value = term.nextElementSibling;
  expect(value?.tagName).toBe("DD");
  return value?.textContent?.trim() ?? "";
}

afterEach(() => {
  cleanup();
});

describe("006/US5 - Comparar el esfuerzo estimado con el real", () => {
  it("Caso normal — 5 Story Points con factor 6 y 36 horas reales", () => {
    render(<EffortComparisonCard comparison={base} />);

    expect(valueOf("Story Points")).toBe("5 SP");
    expect(valueOf("Horas estimadas")).toBe("30 h");
    expect(valueOf("Horas reales")).toBe("36 h");
    expect(valueOf("Diferencia")).toBe("+6 h");
    expect(valueOf("Diferencia porcentual")).toBe("+20,00 %");
  });

  it("Caso alternativo — la misma historia con 24 horas reales: el signo distingue el ahorro", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          actualHours: 24,
          deviationHours: -6,
          deviationPercent: -20,
        }}
      />,
    );

    expect(valueOf("Horas reales")).toBe("24 h");
    expect(valueOf("Diferencia")).toBe(`${minus}6 h`);
    expect(valueOf("Diferencia porcentual")).toBe(`${minus}20,00 %`);
  });

  it("Caso alternativo — horas reales iguales a las estimadas", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          actualHours: 30,
          deviationHours: 0,
          deviationPercent: 0,
        }}
      />,
    );

    expect(valueOf("Diferencia")).toBe("0 h");
    expect(valueOf("Diferencia porcentual")).toBe("0,00 %");
  });

  it("Caso límite — historia sin estimar con 12 horas reales", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          storyPoints: null,
          estimatedHours: null,
          actualHours: 12,
          deviationHours: null,
          deviationPercent: null,
        }}
      />,
    );

    expect(valueOf("Story Points")).toBe("Sin estimar");
    expect(valueOf("Horas estimadas")).toBe(notCalculable);
    expect(valueOf("Horas reales")).toBe("12 h");
    expect(valueOf("Diferencia")).toBe(notCalculable);
    expect(valueOf("Diferencia porcentual")).toBe(notCalculable);
  });

  it("Caso límite — historia de 0 Story Points con 4 horas reales", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          storyPoints: 0,
          estimatedHours: 0,
          actualHours: 4,
          deviationHours: 4,
          deviationPercent: null,
        }}
      />,
    );

    expect(valueOf("Story Points")).toBe("0 SP");
    expect(valueOf("Horas estimadas")).toBe("0 h");
    expect(valueOf("Horas reales")).toBe("4 h");
    expect(valueOf("Diferencia")).toBe("+4 h");
    expect(valueOf("Diferencia porcentual")).toBe(notCalculable);
  });

  it("Caso límite — historia de 3 Story Points sin registros de esfuerzo", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          storyPoints: 3,
          estimatedHours: 18,
          actualHours: 0,
          deviationHours: -18,
          deviationPercent: -100,
        }}
      />,
    );

    expect(valueOf("Horas estimadas")).toBe("18 h");
    expect(valueOf("Horas reales")).toBe("0 h");
    expect(valueOf("Diferencia")).toBe(`${minus}18 h`);
    expect(valueOf("Diferencia porcentual")).toBe(`${minus}100,00 %`);
  });

  it("Caso límite — historia de 0 Story Points sin registros", () => {
    render(
      <EffortComparisonCard
        comparison={{
          ...base,
          storyPoints: 0,
          estimatedHours: 0,
          actualHours: 0,
          deviationHours: 0,
          deviationPercent: null,
        }}
      />,
    );

    expect(valueOf("Horas estimadas")).toBe("0 h");
    expect(valueOf("Horas reales")).toBe("0 h");
    expect(valueOf("Diferencia")).toBe("0 h");
    expect(valueOf("Diferencia porcentual")).toBe(notCalculable);
  });
});

// Verificación de la distinción de FR-055 que contrasta dos escenarios; no corresponde a uno solo.
describe("EffortComparisonCard - distinción entre 0 Story Points y sin estimar", () => {
  it("distinción visible entre 0 Story Points y sin estimar (FR-055)", () => {
    // Las dos historias tienen las mismas horas reales; lo único que cambia es la estimación.
    const zero: EffortComparisonRow = {
      ...base,
      storyPoints: 0,
      estimatedHours: 0,
      actualHours: 4,
      deviationHours: 4,
      deviationPercent: null,
    };
    const unestimated: EffortComparisonRow = {
      ...zero,
      storyPoints: null,
      estimatedHours: null,
      deviationHours: null,
    };

    const { unmount } = render(<EffortComparisonCard comparison={zero} />);
    const zeroValues = [
      valueOf("Story Points"),
      valueOf("Horas estimadas"),
      valueOf("Diferencia"),
    ];
    unmount();
    render(<EffortComparisonCard comparison={unestimated} />);
    const unestimatedValues = [
      valueOf("Story Points"),
      valueOf("Horas estimadas"),
      valueOf("Diferencia"),
    ];

    expect(zeroValues).toEqual(["0 SP", "0 h", "+4 h"]);
    expect(unestimatedValues).toEqual([
      "Sin estimar",
      notCalculable,
      notCalculable,
    ]);
  });
});

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FinalityBadge } from "@/features/dashboard/components/finality-badge";
import { MetricValue } from "@/features/dashboard/components/metric-value";
import type { Metric, SprintMetric } from "@/types/metrics";
import type { MetricKind } from "@/utils/metric-explanations";

// Pruebas unitarias de la presentación de estados de valor y marcas de finalidad
// (T011; RN-03 a RN-06; FR-014 a FR-017).

// Formateador de prueba que envuelve el valor entre almohadillas.
const format = (value: number) => `#${value}#`;

// Renderiza una métrica y devuelve el texto visible del contenedor.
function textOf(
  metric: Metric | SprintMetric,
  kind: MetricKind = "hours",
): string {
  const { container } = render(
    <MetricValue metric={metric} kind={kind} format={format} />,
  );
  return container.textContent ?? "";
}

// Muestra del valor según el estado de la métrica y su finalidad.
describe("Ver los indicadores clave del momento (MetricValue)", () => {
  // Escenario: valor disponible igual a cero (RN-06)
  it("muestra el cero disponible con el formateador recibido", () => {
    expect(textOf({ status: "available", value: 0 })).toBe("#0#");
  });

  // Escenario: métrica no calculable con su explicación (RN-03)
  it.each([
    ["velocity", "Todavía no hay sprints cerrados."],
    ["deviationPercent", "Las horas estimadas son 0."],
    ["storyCompletionPercent", "No hay historias planificadas."],
  ] as const)(
    "muestra No calculable y la explicación para %s",
    (kind, explanation) => {
      const text = textOf({ status: "not_calculable", value: null }, kind);
      expect(text).toContain("No calculable");
      expect(text).toContain(explanation);
      for (const forbidden of ["0", "—", "-", ""]) {
        expect(text.trim()).not.toBe(forbidden);
      }
      expect(text).not.toContain("#0#");
    },
  );

  // Escenario: métrica no aplicable (RN-06)
  it("muestra No aplica para not_applicable", () => {
    expect(textOf({ status: "not_applicable", value: null })).toContain(
      "No aplica",
    );
  });

  // Escenario: los tres estados se distinguen entre sí (RN-06)
  it("los textos de disponible 0, no calculable y no aplica son distintos", () => {
    const available = textOf({ status: "available", value: 0 });
    const notCalculable = textOf({ status: "not_calculable", value: null });
    const notApplicable = textOf({ status: "not_applicable", value: null });
    expect(new Set([available, notCalculable, notApplicable]).size).toBe(3);
  });

  // Escenario: marca Parcial (RN-04)
  it("muestra Parcial cuando la finalidad es partial", () => {
    const text = textOf({ status: "available", value: 5, finality: "partial" });
    expect(text).toContain("Parcial");
    expect(text).toContain("#5#");
  });

  // Escenario: marca Provisoria distinta de Parcial (RN-05)
  it("muestra Provisoria y no Parcial cuando la finalidad es provisional", () => {
    const text = textOf({
      status: "available",
      value: 5,
      finality: "provisional",
    });
    expect(text).toContain("Provisoria");
    expect(text).not.toContain("Parcial");
  });

  // Escenario: finalidad definitiva o nula sin marca (RN-04)
  it.each(["definitive", null] as const)(
    "no muestra marca con finalidad %s",
    (finality) => {
      const text = textOf({ status: "available", value: 5, finality });
      expect(text).toBe("#5#");
    },
  );
});

// Muestra de la marca de finalidad según el estado del valor.
describe("Ver los indicadores clave del momento (FinalityBadge)", () => {
  // Escenario: Parcial y Provisoria producen textos distintos (RN-05)
  it("produce textos distintos para partial y provisional", () => {
    const partial = render(<FinalityBadge finality="partial" />).container
      .textContent;
    const provisional = render(<FinalityBadge finality="provisional" />)
      .container.textContent;
    expect(partial).toBe("Parcial");
    expect(provisional).toBe("Provisoria");
    expect(partial).not.toBe(provisional);
  });

  // Escenario: sin marca para definitive y null (RN-04)
  it.each(["definitive", null] as const)(
    "no renderiza texto con finalidad %s",
    (finality) => {
      expect(
        render(<FinalityBadge finality={finality} />).container.textContent,
      ).toBe("");
    },
  );
});

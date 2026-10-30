import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import type { SeriesPoint } from "@/types/metrics";

// Pruebas del formateador del Tooltip de SprintSeriesChart (T045, per T040). Recharts solo invoca
// el formateador al pasar el cursor, así que el Tooltip se sustituye por uno que lo llama con los
// valores indicados y muestra el resultado.
vi.mock("recharts", async (importOriginal) => {
  const original = await importOriginal<typeof import("recharts")>();
  return {
    ...original,
    Tooltip: ({ formatter }: { formatter: (value: unknown) => ReactNode }) => (
      <div data-testid="tooltip">
        <span data-testid="numeric">{formatter(7)}</span>
        <span data-testid="text">{formatter("sin dato")}</span>
      </div>
    ),
  };
});

// Punto de serie con el identificador y el valor indicados.
const point = (id: string, value: number): SeriesPoint => ({
  sprintId: id,
  sprintName: id,
  sprintStatus: "closed",
  status: "available",
  value,
  finality: "definitive",
});

// Verifica el formateo de los valores del Tooltip.
describe("SprintSeriesChart - formateador del Tooltip (T045)", () => {
  // Desmonta el componente renderizado tras cada prueba.
  afterEach(() => {
    cleanup();
  });

  it("formatea los números con format y convierte a texto el resto de valores", () => {
    render(
      <SprintSeriesChart
        title="Velocidad"
        yAxisLabel="SP"
        kind="storyPoints"
        format={(value) => `${value} SP`}
        emptyMessage="Vacío"
        errorMessage="Error"
        series={[
          { name: "A", points: [point("s1", 1)] },
          { name: "B", points: [point("s1", 2)] },
        ]}
      />,
    );
    expect(screen.getByTestId("numeric").textContent).toBe("7 SP");
    expect(screen.getByTestId("text").textContent).toBe("sin dato");
  });
});

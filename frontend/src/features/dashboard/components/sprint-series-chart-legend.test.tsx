import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import type { SeriesPoint } from "@/types/metrics";

// Pruebas de la guarda `payload ?? []` de la Legend de SprintSeriesChart (T049, per T040).
// Recharts siempre entrega el payload, así que la Legend se sustituye por una que invoca su
// `content` sin payload (rama de la guarda) y otra vez con un payload para el caso normal.
vi.mock("recharts", async (importOriginal) => {
  const original = await importOriginal<typeof import("recharts")>();
  return {
    ...original,
    Legend: ({
      content,
    }: {
      content: (props: {
        payload?: { value: string; color: string }[];
      }) => ReactNode;
    }) => (
      <div>
        <div data-testid="legend-empty">{content({})}</div>
        <div data-testid="legend-full">
          {content({ payload: [{ value: "Serie A", color: "#123456" }] })}
        </div>
      </div>
    ),
  };
});

const point = (id: string, value: number): SeriesPoint => ({
  sprintId: id,
  sprintName: id,
  sprintStatus: "closed",
  status: "available",
  value,
  finality: "definitive",
});

describe("SprintSeriesChart - guarda del payload de la Legend (T049)", () => {
  afterEach(() => {
    cleanup();
  });

  it("renderiza una leyenda vacía sin fallar cuando no llega payload", () => {
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
    const empty = screen.getByTestId("legend-empty");
    expect(empty.querySelectorAll("li")).toHaveLength(0);
    expect(empty.querySelector("ul")).not.toBeNull();
    // Con payload la leyenda sí lista sus entradas.
    expect(screen.getByTestId("legend-full").textContent).toBe("Serie A");
  });
});

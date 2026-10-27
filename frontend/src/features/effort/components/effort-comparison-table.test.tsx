import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { EffortComparisonTable } from "@/features/effort/components/effort-comparison-table";
import type { EffortComparisonRow } from "@/types/effort";

// Pruebas de la tabla de comparación entre estimado y real del proyecto (T065). El componente es de
// presentación: recibe las historias tal como las devuelve
// GET /api/v1/projects/{projectId}/effort-comparison (stories de ProjectEffortComparison en
// contracts/openapi.yaml), ya calculadas y ordenadas por el backend (T068). Acá se verifica cómo se
// muestra cada fila, no el cálculo ni el orden.
//
// Contrato que fijan estas pruebas para EffortComparisonTable (T070):
// - Recibe stories (EffortComparisonRow[]).
// - Muestra una tabla con nombre accesible "Comparación estimado contra real por historia", con los
//   encabezados de columna "Historia", "Story Points", "Horas estimadas", "Horas reales",
//   "Diferencia" y "Diferencia porcentual", y una fila de datos por historia en el orden recibido.
// - Cada celda usa el mismo formato que EffortComparisonCard: Story Points "<n> SP" o "Sin estimar"
//   (FR-055), horas con formatHours, diferencia con signo ("+6 h", "−6 h", "0 h"), porcentaje con
//   formatPercent y "No calculable" para todo null, nunca 0 (FR-050, FR-051).

const minus = "−";
const notCalculable = "No calculable";

const columns = [
  "Historia",
  "Story Points",
  "Horas estimadas",
  "Horas reales",
  "Diferencia",
  "Diferencia porcentual",
] as const;

type Column = (typeof columns)[number];

// Arma una fila de comparación con identificador y título propios.
function row(
  index: number,
  title: string,
  values: Omit<EffortComparisonRow, "storyId" | "title">,
): EffortComparisonRow {
  return {
    storyId: `00000000-0000-4000-8000-00000000000${index}`,
    title,
    ...values,
  };
}

// Devuelve, por cada fila de datos de la tabla, el texto de cada columna indexado por su
// encabezado. Las celdas se toman por posición respecto de los encabezados, sean td o th.
function tableRows(): Record<Column, string>[] {
  const table = screen.getByRole("table", {
    name: "Comparación estimado contra real por historia",
  });
  const headers = within(table)
    .getAllByRole("columnheader")
    .map((header) => header.textContent?.trim() ?? "");
  expect(headers).toEqual([...columns]);

  const dataRows = within(table)
    .getAllByRole("row")
    .filter((r) => within(r).queryAllByRole("columnheader").length === 0);
  return dataRows.map((r) => {
    const cells = Array.from(r.querySelectorAll("td, th")).map(
      (cell) => cell.textContent?.trim() ?? "",
    );
    const result = {} as Record<Column, string>;
    columns.forEach((column, i) => {
      result[column] = cells[i] ?? "";
    });
    return result;
  });
}

afterEach(() => {
  cleanup();
});

describe("006/US5 - Comparar el esfuerzo estimado con el real", () => {
  it("Caso alternativo — la misma historia con 24 horas reales: el signo distingue el ahorro", () => {
    // Escenarios 1, 2 y 3 de US5: la misma historia de 5 Story Points con 36, 24 y 30 horas reales.
    render(
      <EffortComparisonTable
        stories={[
          row(1, "Exceso", {
            storyPoints: 5,
            estimatedHours: 30,
            actualHours: 36,
            deviationHours: 6,
            deviationPercent: 20,
          }),
          row(2, "Ahorro", {
            storyPoints: 5,
            estimatedHours: 30,
            actualHours: 24,
            deviationHours: -6,
            deviationPercent: -20,
          }),
          row(3, "Exacta", {
            storyPoints: 5,
            estimatedHours: 30,
            actualHours: 30,
            deviationHours: 0,
            deviationPercent: 0,
          }),
        ]}
      />,
    );

    expect(tableRows()).toEqual([
      {
        Historia: "Exceso",
        "Story Points": "5 SP",
        "Horas estimadas": "30 h",
        "Horas reales": "36 h",
        Diferencia: "+6 h",
        "Diferencia porcentual": "+20,00 %",
      },
      {
        Historia: "Ahorro",
        "Story Points": "5 SP",
        "Horas estimadas": "30 h",
        "Horas reales": "24 h",
        Diferencia: `${minus}6 h`,
        "Diferencia porcentual": `${minus}20,00 %`,
      },
      {
        Historia: "Exacta",
        "Story Points": "5 SP",
        "Horas estimadas": "30 h",
        "Horas reales": "30 h",
        Diferencia: "0 h",
        "Diferencia porcentual": "0,00 %",
      },
    ]);
  });
});

// Verificación de la distinción de FR-055 que agrupa varios escenarios; no corresponde a uno solo.
describe("EffortComparisonTable - distinción entre 0 Story Points y sin estimar", () => {
  it("historias de 0 Story Points y sin estimar se distinguen (FR-055)", () => {
    // Escenarios 5, 6, 7 y 8 de US5 en una misma tabla.
    render(
      <EffortComparisonTable
        stories={[
          row(1, "Sin estimar con horas", {
            storyPoints: null,
            estimatedHours: null,
            actualHours: 12,
            deviationHours: null,
            deviationPercent: null,
          }),
          row(2, "Cero puntos con horas", {
            storyPoints: 0,
            estimatedHours: 0,
            actualHours: 4,
            deviationHours: 4,
            deviationPercent: null,
          }),
          row(3, "Tres puntos sin registros", {
            storyPoints: 3,
            estimatedHours: 18,
            actualHours: 0,
            deviationHours: -18,
            deviationPercent: -100,
          }),
          row(4, "Cero puntos sin registros", {
            storyPoints: 0,
            estimatedHours: 0,
            actualHours: 0,
            deviationHours: 0,
            deviationPercent: null,
          }),
        ]}
      />,
    );

    expect(tableRows()).toEqual([
      {
        Historia: "Sin estimar con horas",
        "Story Points": "Sin estimar",
        "Horas estimadas": notCalculable,
        "Horas reales": "12 h",
        Diferencia: notCalculable,
        "Diferencia porcentual": notCalculable,
      },
      {
        Historia: "Cero puntos con horas",
        "Story Points": "0 SP",
        "Horas estimadas": "0 h",
        "Horas reales": "4 h",
        Diferencia: "+4 h",
        "Diferencia porcentual": notCalculable,
      },
      {
        Historia: "Tres puntos sin registros",
        "Story Points": "3 SP",
        "Horas estimadas": "18 h",
        "Horas reales": "0 h",
        Diferencia: `${minus}18 h`,
        "Diferencia porcentual": `${minus}100,00 %`,
      },
      {
        Historia: "Cero puntos sin registros",
        "Story Points": "0 SP",
        "Horas estimadas": "0 h",
        "Horas reales": "0 h",
        Diferencia: "0 h",
        "Diferencia porcentual": notCalculable,
      },
    ]);
  });
});

describe("006/US5 - Comparar el esfuerzo estimado con el real", () => {
  it("Caso límite — proyecto con todas las historias sin estimar", () => {
    // Escenario 9 de US5: la lista completa con "No calculable", sin mensaje de error.
    render(
      <EffortComparisonTable
        stories={[
          row(1, "Primera", {
            storyPoints: null,
            estimatedHours: null,
            actualHours: 8,
            deviationHours: null,
            deviationPercent: null,
          }),
          row(2, "Segunda", {
            storyPoints: null,
            estimatedHours: null,
            actualHours: 0,
            deviationHours: null,
            deviationPercent: null,
          }),
        ]}
      />,
    );

    const rows = tableRows();
    expect(rows.map((r) => r.Historia)).toEqual(["Primera", "Segunda"]);
    rows.forEach((r) => {
      expect(r["Story Points"]).toBe("Sin estimar");
      expect(r["Horas estimadas"]).toBe(notCalculable);
      expect(r.Diferencia).toBe(notCalculable);
      expect(r["Diferencia porcentual"]).toBe(notCalculable);
    });
    expect(rows[0]["Horas reales"]).toBe("8 h");
    expect(rows[1]["Horas reales"]).toBe("0 h");
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

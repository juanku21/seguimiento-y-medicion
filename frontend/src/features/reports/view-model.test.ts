import { describe, expect, it } from "vitest";
import {
  defaultReportSprint,
  sprintReport,
} from "@/features/reports/components/reports-test-backend";
import {
  buildReportView,
  metricText,
  type SectionView,
} from "@/features/reports/view-model";
import type { Report } from "@/types/report";
import { explainNotCalculable } from "@/utils/metric-explanations";
import { formatDecimal } from "@/utils/metric-format";

// Zona horaria del navegador usada por las pruebas.
const zone = "America/Argentina/Buenos_Aires";

// Devuelve la sección con la clave indicada del reporte armado.
function section(report: Report, key: SectionView["key"]): SectionView {
  const found = buildReportView(report, zone).sections.find(
    (item) => item.key === key,
  );
  if (!found) {
    throw new Error(`falta la sección ${key}`);
  }
  return found;
}

// Reporte de sprint con las líneas de poker indicadas en las estimaciones.
function withPoker(items: Report["estimations"]["groups"]): Report {
  const base = sprintReport();
  return { ...base, estimations: { ...base.estimations, groups: items } };
}

describe("010/US1 - Generar y ver el reporte de un sprint", () => {
  // Escenario: US1-1 (seis secciones en orden fijo)
  it("buildReportView devuelve las seis secciones en orden", () => {
    const view = buildReportView(sprintReport(), zone);
    expect(view.sections.map((item) => item.key)).toEqual([
      "header",
      "stories",
      "estimations",
      "effort",
      "metrics",
      "defects",
    ]);
    expect(view.sections.map((item) => item.title)).toEqual([
      "Encabezado",
      "Historias",
      "Estimaciones",
      "Esfuerzo",
      "Métricas",
      "Defectos",
    ]);
  });

  // Escenario: US1-1 (título y encabezado)
  it("arma el título y los términos del encabezado", () => {
    const view = buildReportView(sprintReport(), zone);
    expect(view.title).toBe("Reporte del sprint Sprint 1");
    expect(view.header).toEqual([
      { label: "Proyecto", value: "Plataforma de seguimiento" },
      { label: "Alcance", value: "Sprint Sprint 1" },
      { label: "Período", value: "07/09/2026 al 27/09/2026" },
      { label: "Generado el", value: "10/11/2026 11:30:00" },
      { label: "Generado por", value: "Ana Pérez" },
    ]);
  });

  // Escenario: US1-1 (la hora sigue la zona indicada)
  it("expresa Generado el en la zona horaria indicada", () => {
    const view = buildReportView(sprintReport(), "UTC");
    const generated = view.header.find((item) => item.label === "Generado el");
    expect(generated?.value).toBe("10/11/2026 14:30:00");
  });

  // Escenario: US1-1 (reporte completo)
  it("partialNote es null con partial false", () => {
    expect(buildReportView(sprintReport(), zone).partialNote).toBeNull();
  });

  // Escenario: US1-2 (sprint activo parcial)
  it("partialNote avisa que el reporte es parcial con partial true", () => {
    const view = buildReportView(sprintReport({ partial: true }), zone);
    expect(view.partialNote).toBe(
      "Reporte parcial: incluye un sprint en curso.",
    );
  });

  // Escenario: US1-1 (columnas fijas y celdas de historias)
  it("la tabla de historias tiene las columnas fijas y los textos de estado", () => {
    const block = section(sprintReport(), "stories").blocks[0];
    expect(block.heading).toBe("Sprint 1 (07/09/2026 al 27/09/2026)");
    expect(block.table?.columns).toEqual([
      "Historia",
      "Prioridad",
      "Story Points",
      "Estado",
    ]);
    expect(block.table?.rows).toContainEqual([
      "Tablero",
      "Alta",
      "8",
      "Completada",
    ]);
    expect(block.table?.rows).toContainEqual([
      "Perfil",
      "Baja",
      "Sin estimar",
      "No completada",
    ]);
  });

  // Escenario: US1-1 (columnas fijas de estimaciones)
  it("la tabla de estimaciones tiene las columnas fijas", () => {
    const block = section(sprintReport(), "estimations").blocks[0];
    expect(block.table?.columns).toEqual([
      "Historia",
      "Story Points",
      "Planning Poker",
    ]);
  });

  // Escenario: US1-1 (columnas fijas de esfuerzo)
  it("esfuerzo tiene la tabla por historia y la tabla por integrante", () => {
    const tables = section(sprintReport(), "effort")
      .blocks.map((block) => block.table)
      .filter((table) => table !== null);
    expect(tables.map((table) => table.columns)).toEqual([
      ["Historia", "Horas"],
      ["Integrante", "Horas"],
    ]);
    expect(tables[0].footer?.[0]).toBe("Total del sprint");
  });

  // Escenario: US1-1 (columnas fijas de métricas)
  it("la tabla de métricas tiene las columnas Métrica y Valor", () => {
    const table = section(sprintReport(), "metrics").blocks[0].table;
    expect(table?.columns).toEqual(["Métrica", "Valor"]);
  });

  // Escenario: US1-1 (columnas fijas de defectos)
  it("la tabla de defectos tiene las columnas fijas", () => {
    const table = section(sprintReport(), "defects").blocks[0].table;
    expect(table?.columns).toEqual([
      "Descripción",
      "Severidad",
      "Estado",
      "Historia",
      "Detectado en",
      "Resuelto en",
    ]);
  });

  // Escenario: US1-3 (poker con valor acordado)
  it("describe las rondas y el valor acordado, en singular y plural", () => {
    const items = sprintReport().estimations.groups?.[0].items ?? [];
    const session = (rounds: number, points: number) => ({
      rounds,
      agreedStoryPoints: points,
      finishedAt: "2026-09-10T12:00:00Z",
    });
    const report = withPoker([
      {
        sprint: defaultReportSprint,
        state: "complete",
        items: [
          { ...items[0], poker: "agreed", session: session(3, 8) },
          { ...items[1], poker: "agreed", session: session(1, 5) },
        ],
      },
    ]);
    const rows = section(report, "estimations").blocks[0].table?.rows ?? [];
    expect(rows.map((row) => row[2])).toEqual([
      "3 rondas, valor acordado 8",
      "1 ronda, valor acordado 5",
    ]);
  });

  // Escenario: US1-4 (historia sin sesión de poker)
  it("indica que no hubo sesión o que no hubo valor acordado", () => {
    const items = sprintReport().estimations.groups?.[0].items ?? [];
    const report = withPoker([
      {
        sprint: defaultReportSprint,
        state: "complete",
        items: [
          { ...items[0], poker: "none", session: null },
          { ...items[1], poker: "no_agreement", session: null },
        ],
      },
    ]);
    const rows = section(report, "estimations").blocks[0].table?.rows ?? [];
    expect(rows.map((row) => row[2])).toEqual([
      "No hubo sesión de estimación",
      "Sin valor acordado por estimación colaborativa",
    ]);
  });
});

describe("010/US1 - metricText", () => {
  // Escenario: US1-7 (disponible en cero)
  it("muestra 0 cuando el valor disponible es cero", () => {
    expect(
      metricText({ status: "available", value: 0 }, "storyPoints", String),
    ).toBe("0");
  });

  // Escenario: US1-7 (RN-13: no calculable nunca es 0)
  it("explica lo no calculable y nunca lo muestra como 0", () => {
    const kinds = [
      "velocity",
      "deviationPercent",
      "storyCompletionPercent",
    ] as const;
    for (const kind of kinds) {
      const text = metricText(
        { status: "not_calculable", value: null },
        kind,
        formatDecimal,
      );
      expect(text).toBe(`No calculable: ${explainNotCalculable(kind)}`);
      expect(text).not.toBe("0");
    }
  });

  // Escenario: US1-1 (no aplica)
  it("muestra No aplica para lo no aplicable", () => {
    expect(
      metricText(
        { status: "not_applicable", value: null },
        "defects",
        formatDecimal,
      ),
    ).toBe("No aplica");
  });

  // Escenario: US1-1 (RN-14: la finalidad se conserva)
  it("agrega una marca distinta por cada finalidad", () => {
    const texts = (["partial", "provisional", "definitive"] as const).map(
      (finality) =>
        metricText(
          { status: "available", value: 14, finality },
          "velocity",
          formatDecimal,
        ),
    );
    expect(texts).toEqual([
      "14,00 · Parcial",
      "14,00 · Provisoria",
      "14,00 · Definitiva",
    ]);
    expect(new Set(texts).size).toBe(3);
  });
});

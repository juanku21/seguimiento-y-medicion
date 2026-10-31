import { describe, expect, it } from "vitest";
import { isReport } from "@/features/reports/api";
import { sprintReport } from "@/features/reports/components/reports-test-backend";

// Copia profunda del reporte de prueba como objeto sin tipo, para adulterarlo.
function tampered(): Record<string, unknown> {
  return JSON.parse(JSON.stringify(sprintReport())) as Record<string, unknown>;
}

describe("010/US1 - Generar y ver el reporte de un sprint", () => {
  // Escenario: US1-1 (reporte válido)
  it("isReport acepta el reporte de sprint completo", () => {
    expect(isReport(sprintReport())).toBe(true);
  });

  // Escenario: US1-1 (cuerpo sin encabezado)
  it("isReport rechaza un cuerpo sin header", () => {
    const body = tampered();
    delete body.header;
    expect(isReport(body)).toBe(false);
  });

  // Escenario: US1-1 (estado de historia desconocido)
  it("isReport rechaza un stories.state desconocido", () => {
    const body = tampered();
    body.stories = { state: "unknown", groups: null };
    expect(isReport(body)).toBe(false);
  });

  // Escenario: US1-1 (Story Points con tipo equivocado)
  it("isReport rechaza storyPoints como texto", () => {
    const body = tampered();
    const stories = body.stories as {
      groups: { stories: { storyPoints: unknown }[] }[];
    };
    stories.groups[0].stories[0].storyPoints = "8";
    expect(isReport(body)).toBe(false);
  });

  // Escenario: US1-1 (sección faltante)
  it("isReport rechaza un cuerpo al que le falta una sección", () => {
    const body = tampered();
    delete body.defects;
    expect(isReport(body)).toBe(false);
  });

  // Escenario: US1-1 (valores que no son objeto)
  it("isReport rechaza null y valores que no son objeto", () => {
    expect(isReport(null)).toBe(false);
    expect(isReport("reporte")).toBe(false);
  });
});

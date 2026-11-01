import { describe, expect, it } from "vitest";
import type { PdfContent } from "pdfmake/build/pdfmake";
import { sprintReport } from "@/features/reports/components/reports-test-backend";
import { buildPdfDocument } from "@/features/reports/pdf/pdf-document";
import {
  buildReportView,
  type ReportView,
} from "@/features/reports/view-model";
import type { Report } from "@/types/report";

// Zona horaria del navegador usada por las pruebas.
const zone = "America/Argentina/Buenos_Aires";

// Tabla de pdfmake tal como la declaran los tipos del proyecto.
type PdfTable = NonNullable<Exclude<PdfContent, string>["table"]>;

// Recorre el contenido y devuelve sus textos en el orden del documento.
function extractTexts(node: PdfContent | PdfContent[]): string[] {
  if (typeof node === "string") {
    return [node];
  }
  if (Array.isArray(node)) {
    return node.flatMap(extractTexts);
  }
  if (node.table) {
    return node.table.body.flatMap((row) => row.flatMap(extractTexts));
  }
  if (node.stack) {
    return node.stack.flatMap(extractTexts);
  }
  if (node.text === undefined) {
    return [];
  }
  return typeof node.text === "string" ? [node.text] : extractTexts(node.text);
}

// Recorre el contenido y devuelve todas sus tablas, a cualquier profundidad.
function collectTables(node: PdfContent | PdfContent[]): PdfTable[] {
  if (typeof node === "string") {
    return [];
  }
  if (Array.isArray(node)) {
    return node.flatMap(collectTables);
  }
  const nested: PdfContent[] = [
    ...(node.stack ?? []),
    ...(Array.isArray(node.text) ? node.text : []),
    ...(node.table ? node.table.body.flat() : []),
  ];
  return [
    ...(node.table ? [node.table] : []),
    ...nested.flatMap(collectTables),
  ];
}

// Textos que la pantalla muestra, en su orden: título, nota, y por sección su título,
// mensaje, datos del encabezado y bloques (subtítulo, mensaje, columnas, filas y pie).
function expectedTexts(view: ReportView): string[] {
  const texts = [view.title];
  if (view.partialNote !== null) {
    texts.push(view.partialNote);
  }
  for (const section of view.sections) {
    texts.push(section.title);
    if (section.message !== null) {
      texts.push(section.message);
    }
    if (section.key === "header") {
      view.header.forEach((item) => texts.push(item.label, item.value));
    }
    for (const block of section.blocks) {
      if (block.heading !== null) {
        texts.push(block.heading);
      }
      if (block.message !== null) {
        texts.push(block.message);
      }
      if (block.table) {
        texts.push(...block.table.columns, ...block.table.rows.flat());
        if (block.table.footer) {
          texts.push(...block.table.footer);
        }
      }
    }
  }
  return texts;
}

// Reporte de sprint con la cantidad indicada de historias en su único grupo.
function reportWithStories(count: number): Report {
  const base = sprintReport();
  const group = base.stories.groups?.[0];
  if (!group || !("stories" in group)) {
    throw new Error("falta el grupo de historias");
  }
  const stories = Array.from({ length: count }, (_, index) => ({
    storyId: `a0000000-0000-4000-8000-${String(index).padStart(12, "0")}`,
    title: `Historia ${index + 1}`,
    priority: "medium" as const,
    storyPoints: 3,
    status: "completed" as const,
  }));
  return {
    ...base,
    stories: { ...base.stories, groups: [{ ...group, stories }] },
  };
}

describe("010/US2 - Exportar el reporte a PDF", () => {
  // Escenario: US2-1 (el PDF tiene los mismos textos que la pantalla, RN-15)
  it("buildPdfDocument contiene los textos de la vista en el mismo orden", () => {
    const view = buildReportView(sprintReport(), zone);
    const doc = buildPdfDocument(view);
    expect(extractTexts(doc.content)).toEqual(expectedTexts(view));
  });

  // Escenario: US2-1 (el título es el primer bloque)
  it("el primer bloque del contenido es el título de la vista", () => {
    const view = buildReportView(sprintReport(), zone);
    const doc = buildPdfDocument(view);
    expect(extractTexts(doc.content[0])).toEqual([view.title]);
  });

  // Escenario: US2-2 (reporte parcial: nota bajo el título y en el pie)
  it("con reporte parcial la nota va debajo del título y en el pie", () => {
    const view = buildReportView(sprintReport({ partial: true }), zone);
    const doc = buildPdfDocument(view);
    const note = view.partialNote;
    expect(note).not.toBeNull();
    const texts = extractTexts(doc.content);
    expect(texts[0]).toBe(view.title);
    expect(texts[1]).toBe(note);
    const footerTexts = extractTexts(doc.footer?.(1, 3) ?? []);
    expect(footerTexts).toContain("Página 1 de 3");
    expect(footerTexts).toContain(note);
  });

  // Escenario: US2-2 (reporte no parcial: el pie solo numera las páginas)
  it("sin nota parcial el pie es solo Página N de M", () => {
    const view = buildReportView(sprintReport(), zone);
    const doc = buildPdfDocument(view);
    expect(view.partialNote).toBeNull();
    expect(extractTexts(doc.footer?.(2, 3) ?? [])).toEqual(["Página 2 de 3"]);
  });

  // Escenario: US2-4 (toda tabla repite el encabezado y no parte filas, FR-032)
  it("toda tabla tiene headerRows 1, dontBreakRows y las columnas como primera fila", () => {
    const view = buildReportView(sprintReport(), zone);
    const doc = buildPdfDocument(view);
    const tables = collectTables(doc.content);
    const expectedColumns = view.sections
      .flatMap((section) => section.blocks)
      .flatMap((block) => (block.table ? [block.table.columns] : []));
    expect(tables.length).toBeGreaterThan(0);
    expect(tables).toHaveLength(expectedColumns.length);
    tables.forEach((table, index) => {
      expect(table.headerRows).toBe(1);
      expect(table.dontBreakRows).toBe(true);
      expect(table.body[0].flatMap(extractTexts)).toEqual(
        expectedColumns[index],
      );
    });
  });

  // Escenario: US2-4 (80 historias: ninguna fila se pierde al paginar, RN-16)
  it("con 80 historias la tabla de historias tiene 81 filas", () => {
    const view = buildReportView(reportWithStories(80), zone);
    const doc = buildPdfDocument(view);
    const storiesTable = collectTables(doc.content).find(
      (table) =>
        extractTexts(table.body[0]).join("|") ===
        "Historia|Prioridad|Story Points|Estado",
    );
    expect(storiesTable).toBeDefined();
    expect(storiesTable?.body).toHaveLength(81);
    const titles = storiesTable?.body
      .slice(1)
      .map((row) => extractTexts(row[0])[0]);
    expect(titles).toEqual(
      Array.from({ length: 80 }, (_, index) => `Historia ${index + 1}`),
    );
  });

  // Escenario: US2-5 (acentos, eñes y raya llegan sin cambios, FR-031)
  it("los textos con acentos, eñes y raya llegan sin cambios", () => {
    const base = sprintReport();
    const report: Report = {
      ...base,
      header: {
        ...base.header,
        projectName: "Gestión Ñandú — año único",
        generatedBy: { ...base.header.generatedBy, fullName: "José Peña" },
      },
    };
    const view = buildReportView(report, zone);
    const texts = extractTexts(buildPdfDocument(view).content);
    expect(texts).toContain("Gestión Ñandú — año único");
    expect(texts).toContain("José Peña");
    expect(texts).toContain("Métricas");
  });

  // Escenario: US2-5 (la fuente por defecto es Roboto, que cubre el español)
  it("la fuente por defecto es Roboto", () => {
    const doc = buildPdfDocument(buildReportView(sprintReport(), zone));
    expect(doc.defaultStyle).toMatchObject({ font: "Roboto" });
  });

  // Escenario: US2-1 (pdfmake modifica el documento en el lugar y la pantalla sigue pintando, FR-027)
  it("la mutación de pdfmake sobre el documento no modifica las filas de la vista", () => {
    const view = buildReportView(sprintReport(), zone);
    const doc = buildPdfDocument(view);

    // Simula a pdfmake: reemplaza cada celda de texto por un objeto, en el lugar.
    collectTables(doc.content).forEach((table) =>
      table.body.forEach((row) =>
        row.forEach((cell, index) => {
          if (typeof cell === "string") {
            row[index] = { text: cell, margin: [0, 0, 0, 0] };
          }
        }),
      ),
    );

    const tables = view.sections
      .flatMap((section) => section.blocks)
      .flatMap((block) => (block.table ? [block.table] : []));
    expect(tables.length).toBeGreaterThan(0);
    tables.forEach((table) =>
      table.rows.forEach((row) =>
        row.forEach((cell) => expect(typeof cell).toBe("string")),
      ),
    );
  });
});

describe("010/US4 - Leer un reporte con secciones sin datos", () => {
  // Reporte con una sección sin datos y otra no disponible.
  function reportWithGaps(): Report {
    const base = sprintReport();
    return {
      ...base,
      estimations: { state: "empty", groups: [] },
      effort: {
        state: "unavailable",
        groups: null,
        byMember: null,
        totalHours: null,
      },
    };
  }

  // Escenario: US4-5 (el PDF conserva título y mensaje de cada sección sin datos)
  it("el PDF conserva el título y el mensaje de las secciones empty y unavailable", () => {
    const view = buildReportView(reportWithGaps(), zone);
    const texts = extractTexts(buildPdfDocument(view).content);
    const at = texts.indexOf("Estimaciones");
    expect(at).toBeGreaterThan(-1);
    expect(texts[at + 1]).toBe("Sin datos.");
    const effortAt = texts.indexOf("Esfuerzo");
    expect(effortAt).toBeGreaterThan(-1);
    expect(texts[effortAt + 1]).toBe("No se pudo obtener esta sección.");
    expect(texts[effortAt + 2]).toBe("Métricas");
  });

  // Escenario: US4-5 (mismo orden de secciones que la vista)
  it("las secciones del PDF siguen el orden de la vista", () => {
    const view = buildReportView(reportWithGaps(), zone);
    const texts = extractTexts(buildPdfDocument(view).content);
    const positions = view.sections.map((item) => texts.indexOf(item.title));
    expect(positions.every((position) => position >= 0)).toBe(true);
    expect([...positions].sort((a, b) => a - b)).toEqual(positions);
  });
});

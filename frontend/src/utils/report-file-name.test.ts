import { describe, expect, it } from "vitest";

import { reportFileName } from "./report-file-name";

const TZ = "America/Argentina/Buenos_Aires";

// Parte variable del nombre entre los guiones bajos fijos.
function parts(fileName: string): string[] {
  return fileName
    .replace(/^Reporte_/, "")
    .replace(/\.pdf$/, "")
    .split("_");
}

describe("010/US2 - Exportar el reporte a PDF (nombre del archivo)", () => {
  it("Nombre de un reporte de proyecto (010/US2)", () => {
    const name = reportFileName({
      projectName: "Gestión de Compras",
      scope: "project",
      sprintName: null,
      generatedAt: "2026-10-30T17:32:05Z",
      timeZone: TZ,
    });
    expect(name).toBe(
      "Reporte_Gestion-de-Compras_proyecto_2026-10-30_14-32-05.pdf",
    );
  });

  it("Nombre de un reporte de sprint con caracteres especiales (010/US2)", () => {
    const name = reportFileName({
      projectName: "Ñandú: App/Web *beta*",
      scope: "sprint",
      sprintName: "Sprint 3",
      generatedAt: "2026-10-30T12:05:00Z",
      timeZone: TZ,
    });
    expect(name).toBe(
      "Reporte_Nandu-App-Web-beta_sprint-Sprint-3_2026-10-30_09-05-00.pdf",
    );
  });

  it("Saneado de caracteres inválidos, controles y espacios repetidos (010/US2)", () => {
    const name = reportFileName({
      projectName: '  a\\b/c:d*e?f"g<h>i|j\tk   l  ',
      scope: "sprint",
      sprintName: '--x\\y/z:w*v?u"t<s>r|q\t\t p  ',
      generatedAt: "2026-10-30T12:05:00Z",
      timeZone: TZ,
    });
    const [project, scope] = parts(name);
    expect(project).toBe("a-b-c-d-e-f-g-h-i-j-k-l");
    expect(scope).toBe("sprint-x-y-z-w-v-u-t-s-r-q-p");
    for (const part of [project, scope]) {
      expect(part).not.toMatch(/[\\/:*?"<>|\s]/);
      expect(part).not.toMatch(/--/);
      expect(part).not.toMatch(/^-|-$/);
    }
  });

  it("Un nombre de proyecto vacío tras sanear usa proyecto (010/US2)", () => {
    const name = reportFileName({
      projectName: " /*?: ",
      scope: "project",
      sprintName: null,
      generatedAt: "2026-10-30T12:05:00Z",
      timeZone: TZ,
    });
    expect(name).toBe("Reporte_proyecto_proyecto_2026-10-30_09-05-00.pdf");
  });

  it("Un nombre de sprint vacío tras sanear usa sprint (010/US2)", () => {
    const name = reportFileName({
      projectName: "Alfa",
      scope: "sprint",
      sprintName: "***",
      generatedAt: "2026-10-30T12:05:00Z",
      timeZone: TZ,
    });
    expect(name).toBe("Reporte_Alfa_sprint-sprint_2026-10-30_09-05-00.pdf");
  });

  it("Un nombre de 200 caracteres se limita a 60 (010/US2)", () => {
    const name = reportFileName({
      projectName: "a".repeat(200),
      scope: "sprint",
      sprintName: "b".repeat(200),
      generatedAt: "2026-10-30T12:05:00Z",
      timeZone: TZ,
    });
    const [project, scope] = parts(name);
    expect(project).toBe("a".repeat(60));
    expect(scope).toBe(`sprint-${"b".repeat(60)}`);
  });

  it("Dos instantes del mismo día con un segundo de diferencia dan nombres distintos (010/US2)", () => {
    const base = {
      projectName: "Alfa",
      scope: "project" as const,
      sprintName: null,
      timeZone: TZ,
    };
    const first = reportFileName({
      ...base,
      generatedAt: "2026-10-30T12:05:00Z",
    });
    const second = reportFileName({
      ...base,
      generatedAt: "2026-10-30T12:05:01Z",
    });
    expect(first).not.toBe(second);
  });

  it("La fecha y la hora salen de la zona horaria recibida (010/US2)", () => {
    const base = {
      projectName: "Alfa",
      scope: "project" as const,
      sprintName: null,
      generatedAt: "2026-10-30T01:30:00Z",
    };
    expect(reportFileName({ ...base, timeZone: TZ })).toBe(
      "Reporte_Alfa_proyecto_2026-10-29_22-30-00.pdf",
    );
    expect(reportFileName({ ...base, timeZone: "UTC" })).toBe(
      "Reporte_Alfa_proyecto_2026-10-30_01-30-00.pdf",
    );
  });
});

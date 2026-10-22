import { describe, expect, it } from "vitest";
import {
  validateSprintEndDate,
  validateSprintGoal,
  validateSprintName,
  validateSprintStartDate,
} from "@/features/sprints/validation";

// Pruebas unitarias obligatorias de las validaciones de entrada del formulario de sprint (T014).
// Replican en TypeScript las reglas que el backend prueba en validation_test.go (RN-03 y RN-04): los
// caracteres se cuentan por punto de código tras recortar los espacios extremos, como
// utf8.RuneCountInString, y las fechas llegan como AAAA-MM-DD, el valor de un campo de tipo fecha.
// Los mensajes se verifican por las piezas que deben nombrar, no por su texto exacto.
//
// Los escenarios que dependen del estado del proyecto o de otros sprints (Proyecto Planificado,
// Varios sprints planificados, Fechas contiguas, Superposición, Proyecto finalizado y No integrante)
// no son reglas del formulario: los resuelve el backend y se prueban en sprint-form.test.tsx a través
// de la respuesta que recibe el formulario.

// Datos del caso normal de la spec (US1, escenario 1).
const sprintName = "Sprint 1";
const sprintGoal = "Cerrar el alta de clientes";
const startDate = "2026-10-01";
const endDate = "2026-10-14";

// Verifica que el mensaje de error exista y nombre todas las piezas esperadas, sin distinguir
// mayúsculas de minúsculas.
function expectMessageMentions(message: string | null, ...parts: string[]) {
  expect(message, "se esperaba un mensaje de error").not.toBeNull();
  const lower = (message ?? "").toLowerCase();
  for (const part of parts) {
    expect(lower).toContain(part.toLowerCase());
  }
}

describe("004/US1 - Planificar un sprint", () => {
  // Caso normal: "Sprint 1" del 2026-10-01 al 2026-10-14 con el objetivo "Cerrar el alta de
  // clientes" supera todas las validaciones.
  it("Creación en un proyecto En curso", () => {
    expect(validateSprintName(sprintName)).toBeNull();
    expect(validateSprintGoal(sprintGoal)).toBeNull();
    expect(validateSprintStartDate(startDate)).toBeNull();
    expect(validateSprintEndDate(startDate, endDate)).toBeNull();
  });

  // Caso límite: un Sprint Goal de exactamente 500 caracteres se acepta, también con espacios
  // extremos o caracteres de dos unidades UTF-16; 501 se rechaza indicando el máximo.
  it("Límite del Sprint Goal", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 1 carácter", "G"],
      ["exactamente 500 caracteres", "g".repeat(500)],
      ["500 caracteres acentuados", "é".repeat(500)],
      ["500 emojis (1000 unidades UTF-16)", "😀".repeat(500)],
      ["500 caracteres rodeados de espacios", "  " + "g".repeat(500) + "  "],
    ];
    for (const [name, input] of valid) {
      expect(validateSprintGoal(input), name).toBeNull();
    }

    for (const input of ["g".repeat(501), "é".repeat(501), "😀".repeat(501)]) {
      expectMessageMentions(validateSprintGoal(input), "500");
    }
  });

  // FR-002: el nombre admite 100 caracteres tras recortar espacios, con la misma cuenta por punto de
  // código; 101 se rechaza indicando el máximo.
  it("Límite del nombre", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 1 carácter", "S"],
      ["exactamente 100 caracteres", "a".repeat(100)],
      ["100 caracteres de varios bytes", "ñ".repeat(100)],
      ["100 emojis (200 unidades UTF-16)", "😀".repeat(100)],
      ["100 caracteres rodeados de espacios", "   " + "a".repeat(100) + "   "],
    ];
    for (const [name, input] of valid) {
      expect(validateSprintName(input), name).toBeNull();
    }

    for (const input of ["a".repeat(101), "ñ".repeat(101), "😀".repeat(101)]) {
      expectMessageMentions(validateSprintName(input), "100");
    }
  });

  // Caso límite: una fecha de fin igual al día siguiente de la de inicio es la duración mínima y se
  // acepta, también cuando el día siguiente cae en otro mes o en otro año.
  it("Duración mínima", () => {
    const valid: Array<[string, string]> = [
      ["2026-10-01", "2026-10-02"],
      ["2026-10-31", "2026-11-01"],
      ["2026-12-31", "2027-01-01"],
    ];
    for (const [start, end] of valid) {
      expect(validateSprintEndDate(start, end), `${start} → ${end}`).toBeNull();
    }
  });

  // Caso de error: sin nombre, sin Sprint Goal (vacíos o solo espacios), sin fecha de inicio o sin
  // fecha de fin, cada campo se rechaza indicando cuál corregir.
  it("Campos obligatorios", () => {
    for (const empty of ["", "   \t "]) {
      expectMessageMentions(validateSprintName(empty), "nombre");
      expectMessageMentions(validateSprintGoal(empty), "sprint goal");
    }

    expectMessageMentions(validateSprintStartDate(""), "inicio");
    expectMessageMentions(validateSprintEndDate(startDate, ""), "fin");
  });

  // Caso de error: una fecha de fin igual o anterior a la de inicio se rechaza indicando que debe
  // ser posterior a la de inicio.
  it("Fin no posterior al inicio", () => {
    const cases: Array<[string, string, string]> = [
      ["fin igual al inicio", "2026-10-14", "2026-10-14"],
      ["fin anterior al inicio", "2026-10-14", "2026-10-01"],
      ["fin en el mes anterior", "2026-11-01", "2026-10-31"],
    ];
    for (const [label, start, end] of cases) {
      const message = validateSprintEndDate(start, end);
      expect(message, label).not.toBeNull();
      expectMessageMentions(message, "posterior");
    }
  });
});

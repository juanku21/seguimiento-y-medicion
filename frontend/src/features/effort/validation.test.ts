import { describe, expect, it } from "vitest";
import {
  validateEffortActivity,
  validateEffortEntryDate,
  validateEffortHours,
} from "@/features/effort/validation";

// Pruebas unitarias obligatorias de las validaciones de entrada del formulario de esfuerzo (T018).
// Replican en TypeScript las reglas de forma que el backend prueba en
// backend/internal/effort/delivery/validation_test.go (T009; RN-02, RN-03, FR-006), con los mismos
// mensajes: las horas llegan como el texto del campo numérico, la actividad se cuenta por punto de
// código tras recortar los espacios extremos, como utf8.RuneCountInString, y la fecha llega como
// AAAA-MM-DD, el valor de un campo de tipo fecha.
//
// Las reglas que dependen de datos (fecha futura según la zona del sistema, fecha anterior al inicio
// del proyecto, tope diario, sprint definitivo y proyecto Finalizado) las resuelve el backend y se
// prueban en effort-entry-form.test.tsx a través de la respuesta que recibe el formulario.

// Mensajes idénticos a los del backend (validation_test.go, T009).
const msgHoursRange = "Las horas deben ser mayores que 0 y como máximo 24.";
const msgHoursMultiple =
  "Las horas se cargan en múltiplos de 0,25 (un cuarto de hora).";
const msgHoursNumber = "Las horas son obligatorias y deben ser un número.";
const msgActivityEmpty = "La actividad es obligatoria.";
const msgActivityLong = "La actividad admite como máximo 300 caracteres.";

// Datos del caso normal de la spec (US1, escenario 1).
const activity = "Implementación del formulario de alta";
const entryDate = "2026-10-24";

describe("006/US1 - Registrar el esfuerzo trabajado", () => {
  // Caso normal: 6,5 horas con la fecha de hoy y la actividad del escenario superan todas las
  // validaciones de forma.
  it("Caso normal — registro de 6,5 horas sobre una historia del sprint Activo", () => {
    expect(validateEffortEntryDate(entryDate)).toBeNull();
    expect(validateEffortActivity(activity)).toBeNull();
    expect(validateEffortHours("6.5")).toBeNull();
    expect(validateEffortHours("6.50")).toBeNull();
  });

  // Caso límite: 24 horas es el máximo admitido.
  it("Caso límite — exactamente 24 horas en una fecha sin horas cargadas", () => {
    expect(validateEffortHours("24")).toBeNull();
    expect(validateEffortHours("24.00")).toBeNull();
  });

  // Caso límite: 0,25 horas es el mínimo de la escala.
  it("Caso límite — 0,25 horas, el mínimo de la escala", () => {
    expect(validateEffortHours("0.25")).toBeNull();
  });

  // Caso límite: 300 caracteres se aceptan, también acentuados, emojis (dos unidades UTF-16 cada
  // uno) o rodeados de espacios; 301 se rechazan indicando el máximo.
  it("Caso límite — actividad de 300 y de 301 caracteres", () => {
    const valid: Array<[string, string]> = [
      ["300 caracteres", "a".repeat(300)],
      ["300 caracteres acentuados", "á".repeat(300)],
      ["300 emojis (600 unidades UTF-16)", "😀".repeat(300)],
      ["300 caracteres rodeados de espacios", "  " + "á".repeat(300) + "  "],
    ];
    for (const [name, input] of valid) {
      expect(validateEffortActivity(input), name).toBeNull();
    }

    for (const input of ["a".repeat(301), "á".repeat(301), "😀".repeat(301)]) {
      expect(validateEffortActivity(input)).toBe(msgActivityLong);
    }
  });

  // Caso de error: 0, un negativo o más de 24 horas se rechazan indicando el rango admitido.
  it("Caso de error — horas en 0, negativas o mayores que 24", () => {
    for (const input of ["0", "-1", "24.25", "25"]) {
      expect(validateEffortHours(input), input).toBe(msgHoursRange);
    }
  });

  // Caso de error: dentro del rango, un valor que no es múltiplo de 0,25 se rechaza indicando el
  // múltiplo, incluido 0.10, que tiene dos decimales pero no es un cuarto de hora.
  it("Caso de error — horas que no son múltiplo de 0,25", () => {
    for (const input of ["1.3", "1.1", "0.10"]) {
      expect(validateEffortHours(input), input).toBe(msgHoursMultiple);
    }
  });

  // Caso de error: la actividad ausente o compuesta solo por espacios se rechaza como obligatoria.
  it("Caso de error — actividad ausente o compuesta solo por espacios", () => {
    for (const input of ["", "   ", " \t "]) {
      expect(validateEffortActivity(input), JSON.stringify(input)).toBe(
        msgActivityEmpty,
      );
    }
  });
});

// Reglas de forma sin escenario propio en spec.md (research.md sección 6): no llevan el prefijo
// 006/USn para no alterar el conteo de escenarios.
describe("Reglas de forma del alta de esfuerzo", () => {
  // Las horas que no son un número con a lo sumo dos decimales y sin exponente se rechazan como
  // obligatorias y numéricas, igual que en el backend; también el campo vacío.
  it("rechaza las horas que no son un número con a lo sumo dos decimales", () => {
    for (const input of ["6.555", "1e1", "abc", "", "   "]) {
      expect(validateEffortHours(input), JSON.stringify(input)).toBe(
        msgHoursNumber,
      );
    }
  });

  // La fecha es obligatoria y debe ser un día real con el formato AAAA-MM-DD; un 29 de febrero de
  // año bisiesto es válido.
  it("exige una fecha real con el formato AAAA-MM-DD", () => {
    expect(validateEffortEntryDate("2028-02-29")).toBeNull();

    const empty = validateEffortEntryDate("");
    expect(empty).not.toBeNull();
    expect((empty ?? "").toLowerCase()).toContain("fecha");

    for (const input of [
      "2026-02-30",
      "2026-13-01",
      "24/10/2026",
      "2026-1-5",
    ]) {
      const message = validateEffortEntryDate(input);
      expect(message, input).not.toBeNull();
      expect(message ?? "", input).toContain("AAAA-MM-DD");
    }
  });
});

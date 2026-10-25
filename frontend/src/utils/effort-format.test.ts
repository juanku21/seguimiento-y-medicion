import { describe, expect, it } from "vitest";
import { formatHours, formatPercent } from "@/utils/effort-format";

// Pruebas unitarias del formato de horas y porcentajes del esfuerzo (T018; research.md sección 14).
// La API serializa siempre 2 decimales y usa null para "no calculable"; la interfaz muestra las horas
// con coma decimal, los porcentajes con signo y exactamente 2 decimales, y "No calculable" en lugar
// de null, nunca un 0. El signo negativo es el signo menos tipográfico (U+2212). Es cobertura de
// formato sin escenario propio de US1: el describe no lleva el prefijo 006/USn.

describe("Formato de horas y porcentajes del esfuerzo", () => {
  // Las horas se muestran con coma decimal, sin ceros de relleno, y con la unidad.
  it("muestra las horas con coma decimal y la unidad", () => {
    expect(formatHours(6.5)).toBe("6,5 h");
    expect(formatHours(0.25)).toBe("0,25 h");
  });

  // Los porcentajes llevan signo y exactamente 2 decimales; el negativo usa el signo menos.
  it("muestra los porcentajes con signo y 2 decimales", () => {
    expect(formatPercent(20)).toBe("+20,00 %");
    expect(formatPercent(-20)).toBe("\u221220,00 %");
  });

  // Un valor null es "no calculable" y se muestra con ese texto, no como cero.
  it("muestra No calculable para null", () => {
    expect(formatHours(null)).toBe("No calculable");
    expect(formatPercent(null)).toBe("No calculable");
  });
});

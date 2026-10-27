import { describe, expect, it } from "vitest";
import {
  formatHours,
  formatPercent,
  formatSignedHours,
  formatStoryPoints,
} from "@/utils/effort-format";

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

// Pruebas unitarias de la diferencia de horas y de los Story Points (T080). Es cobertura de formato
// sin escenario propio: el describe no lleva el prefijo 006/USn.
describe("Formato de la diferencia de horas y de los Story Points", () => {
  // La diferencia positiva lleva "+".
  it("muestra la diferencia positiva con signo más", () => {
    expect(formatSignedHours(6)).toBe("+6 h");
  });

  // La diferencia negativa usa el signo menos tipográfico (U+2212).
  it("muestra la diferencia negativa con el signo menos tipográfico", () => {
    expect(formatSignedHours(-6)).toBe("−6 h");
  });

  // El cero no lleva signo.
  it("muestra el cero sin signo", () => {
    expect(formatSignedHours(0)).toBe("0 h");
  });

  // Un null es "no calculable", no un cero.
  it("muestra No calculable para una diferencia null", () => {
    expect(formatSignedHours(null)).toBe("No calculable");
  });

  // Los Story Points se muestran como "n SP", incluido el 0.
  it("muestra los Story Points con la unidad SP", () => {
    expect(formatStoryPoints(5)).toBe("5 SP");
    expect(formatStoryPoints(0)).toBe("0 SP");
  });

  // Una historia sin estimar se distingue de una de 0 SP.
  it("muestra Sin estimar para Story Points null", () => {
    expect(formatStoryPoints(null)).toBe("Sin estimar");
  });
});

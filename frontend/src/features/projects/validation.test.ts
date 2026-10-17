import { describe, expect, it } from "vitest";
import {
  validateDescription,
  validateHoursPerStoryPoint,
  validatePlannedEndDate,
  validateProjectName,
  validateStartDate,
} from "@/features/projects/validation";

// Pruebas unitarias obligatorias de las validaciones de entrada del formulario de proyecto (T021).
// Replican en TypeScript las reglas que el backend prueba en validation_test.go (RN-04, RN-06,
// RN-07, FR-003): los caracteres se cuentan por punto de código tras recortar los espacios extremos
// y el factor se valida sobre el texto escrito, sin redondear.

// Verifica que el mensaje de error exista y nombre todas las piezas esperadas, sin distinguir
// mayúsculas de minúsculas.
function expectMessageMentions(message: string | null, ...parts: string[]) {
  expect(message, "se esperaba un mensaje de error").not.toBeNull();
  const lower = (message ?? "").toLowerCase();
  for (const part of parts) {
    expect(lower).toContain(part.toLowerCase());
  }
}

describe("002/US1 - Crear un proyecto", () => {
  // Caso normal: los datos de "Portal de Clientes" superan todas las validaciones.
  it("Creación exitosa", () => {
    expect(validateProjectName("Portal de Clientes")).toBeNull();
    expect(
      validateDescription("Rediseño del portal de autogestión."),
    ).toBeNull();
    expect(validateStartDate("2026-10-01")).toBeNull();
    expect(validatePlannedEndDate("2026-10-01", "2026-12-20")).toBeNull();
    expect(validateHoursPerStoryPoint("6.5")).toBeNull();
  });

  // FR-003: la descripción es opcional; vacía o solo de espacios es válida.
  it("Sin descripción", () => {
    expect(validateDescription("")).toBeNull();
    expect(validateDescription("   \t  ")).toBeNull();
  });

  // El nombre no se compara en el navegador: la unicidad es por propietario y la resuelve el
  // backend, así que un nombre que otro usuario ya usa es válido para el formulario.
  it("Mismo nombre que el proyecto de otro propietario", () => {
    expect(validateProjectName("Portal de Clientes")).toBeNull();
  });

  // RN-06: una fecha de finalización prevista igual a la de inicio es válida.
  it("Fechas iguales", () => {
    expect(validatePlannedEndDate("2026-10-01", "2026-10-01")).toBeNull();
  });

  // RN-07: 0,01 y 40 son el mínimo y el máximo admitidos; también valores con 0, 1 o 2 decimales.
  it("Límites del factor", () => {
    for (const value of ["0.01", "40", "40.00", "6.5", "6.55", "1"]) {
      expect(
        validateHoursPerStoryPoint(value),
        `factor ${JSON.stringify(value)}`,
      ).toBeNull();
    }
  });

  // RN-04: se aceptan 100 caracteres tras recortar espacios, aunque sean acentuados o emojis; se
  // rechazan 101 con el máximo permitido.
  it("Límites del nombre", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 1 carácter", "P"],
      ["exactamente 100 caracteres", "a".repeat(100)],
      ["100 caracteres acentuados", "é".repeat(100)],
      ["100 emojis (200 unidades UTF-16)", "😀".repeat(100)],
      ["100 caracteres rodeados de espacios", "   " + "a".repeat(100) + "   "],
    ];
    for (const [name, input] of valid) {
      expect(validateProjectName(input), name).toBeNull();
    }

    for (const input of ["a".repeat(101), "é".repeat(101), "😀".repeat(101)]) {
      expectMessageMentions(validateProjectName(input), "nombre", "100");
    }
  });

  // FR-003: la descripción admite hasta 1000 caracteres tras recortar espacios; 1001 se rechaza
  // indicando el máximo permitido.
  it("Límites de la descripción", () => {
    expect(validateDescription("d".repeat(1000))).toBeNull();
    expect(validateDescription("😀".repeat(1000))).toBeNull();
    expect(validateDescription("  " + "d".repeat(1000) + "  ")).toBeNull();

    for (const input of ["d".repeat(1001), "😀".repeat(1001)]) {
      expectMessageMentions(validateDescription(input), "descripción", "1000");
    }
  });

  // Caso de error: nombre ausente o solo de espacios, fechas ausentes y factor ausente se rechazan
  // indicando qué campo corregir.
  it("Campos faltantes", () => {
    expectMessageMentions(validateProjectName(""), "nombre");
    expectMessageMentions(validateProjectName("   \t "), "nombre");
    expectMessageMentions(validateStartDate(""), "fecha");
    expectMessageMentions(validatePlannedEndDate("2026-10-01", ""), "fecha");
    expectMessageMentions(validateHoursPerStoryPoint(""), "factor");
  });

  // RN-07: 0, negativos, más de 40 y más de 2 decimales se rechazan sin redondear, indicando el
  // rango y la precisión admitidos.
  it("Factor inválido", () => {
    for (const value of ["0", "-1", "-6.5", "40.01", "6.555", "0.001", "abc"]) {
      expectMessageMentions(
        validateHoursPerStoryPoint(value),
        "factor",
        "40",
        "2 decimales",
      );
    }
  });

  // RN-06: una fecha de finalización prevista anterior a la de inicio se rechaza.
  it("Fecha de finalización prevista anterior a la de inicio", () => {
    expectMessageMentions(
      validatePlannedEndDate("2026-10-01", "2026-09-30"),
      "anterior",
    );
  });
});

import { describe, expect, it } from "vitest";
import {
  validateAcceptanceCriterion,
  validateStoryDescription,
  validateStoryPriority,
  validateStoryTitle,
} from "@/features/backlog/validation";

// Pruebas unitarias obligatorias de las validaciones de entrada del formulario de historia (T016).
// Replican en TypeScript las reglas que el backend prueba en validation_test.go (RN-03, RN-04, RN-05
// y RN-08): los caracteres se cuentan por punto de código tras recortar los espacios extremos, como
// utf8.RuneCountInString. Los Story Points no se validan acá: el selector solo ofrece la escala.
// Los mensajes se verifican por las piezas que deben nombrar, no por su texto exacto.

// Verifica que el mensaje de error exista y nombre todas las piezas esperadas, sin distinguir
// mayúsculas de minúsculas.
function expectMessageMentions(message: string | null, ...parts: string[]) {
  expect(message, "se esperaba un mensaje de error").not.toBeNull();
  const lower = (message ?? "").toLowerCase();
  for (const part of parts) {
    expect(lower).toContain(part.toLowerCase());
  }
}

describe("003/US1 - Registrar una historia en el backlog", () => {
  // Caso normal: los datos de "Exportar el reporte de velocidad a PDF" superan todas las
  // validaciones, con prioridad Alta y dos criterios.
  it("Creación completa", () => {
    expect(
      validateStoryTitle("Exportar el reporte de velocidad a PDF"),
    ).toBeNull();
    expect(
      validateStoryDescription(
        "El equipo necesita compartir la velocidad con la gerencia.",
      ),
    ).toBeNull();
    expect(validateStoryPriority("high")).toBeNull();
    expect(
      validateAcceptanceCriterion(
        "El PDF incluye la velocidad de los últimos cinco sprints.",
      ),
    ).toBeNull();
    expect(
      validateAcceptanceCriterion(
        "El archivo se descarga con el nombre del proyecto.",
      ),
    ).toBeNull();
  });

  // Caso alternativo: con solo título y prioridad, la descripción vacía o de espacios es válida, y
  // las tres prioridades de la API se aceptan.
  it("Solo título y prioridad", () => {
    expect(
      validateStoryTitle("Revisar los permisos de los integrantes"),
    ).toBeNull();
    expect(validateStoryDescription("")).toBeNull();
    expect(validateStoryDescription("   \t ")).toBeNull();
    for (const priority of ["high", "medium", "low"]) {
      expect(validateStoryPriority(priority), priority).toBeNull();
    }
  });

  // Caso límite: el título admite 150 caracteres tras recortar espacios, aunque sean acentuados o
  // emojis; 151 se rechaza indicando el máximo.
  it("Límites del título", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 1 carácter", "T"],
      ["exactamente 150 caracteres", "a".repeat(150)],
      ["150 caracteres acentuados", "é".repeat(150)],
      ["150 emojis (300 unidades UTF-16)", "😀".repeat(150)],
      ["150 caracteres rodeados de espacios", "  " + "a".repeat(150) + "  "],
    ];
    for (const [name, input] of valid) {
      expect(validateStoryTitle(input), name).toBeNull();
    }

    for (const input of ["a".repeat(151), "é".repeat(151), "😀".repeat(151)]) {
      expectMessageMentions(validateStoryTitle(input), "título", "150");
    }
  });

  // FR-003: la descripción admite 2000 caracteres tras recortar espacios; 2001 se rechaza
  // indicando el máximo.
  it("Límites de la descripción", () => {
    expect(validateStoryDescription("d".repeat(2000))).toBeNull();
    expect(validateStoryDescription("😀".repeat(2000))).toBeNull();
    expect(validateStoryDescription("  " + "d".repeat(2000) + "  ")).toBeNull();

    for (const input of ["d".repeat(2001), "😀".repeat(2001)]) {
      expectMessageMentions(
        validateStoryDescription(input),
        "descripción",
        "2000",
      );
    }
  });

  // Caso límite: un criterio de exactamente 500 caracteres y otro de 1 carácter se aceptan, también
  // con espacios extremos o caracteres de dos unidades UTF-16; 501 se rechaza indicando el máximo.
  it("Límites del criterio", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 1 carácter", "x"],
      ["exactamente 500 caracteres", "c".repeat(500)],
      ["500 emojis (1000 unidades UTF-16)", "😀".repeat(500)],
      ["500 caracteres rodeados de espacios", "  " + "c".repeat(500) + "  "],
    ];
    for (const [name, input] of valid) {
      expect(validateAcceptanceCriterion(input), name).toBeNull();
    }

    for (const input of ["c".repeat(501), "😀".repeat(501)]) {
      expectMessageMentions(validateAcceptanceCriterion(input), "500");
    }
  });

  // Caso límite: el título no es único; la validación del formulario no lo compara con otras
  // historias, así que un título ya usado es válido.
  it("Título repetido", () => {
    const title = "Exportar el reporte de velocidad a PDF";
    expect(validateStoryTitle(title)).toBeNull();
    expect(validateStoryTitle(title)).toBeNull();
  });

  // Caso de error: sin título, con título solo de espacios o sin prioridad se rechaza indicando qué
  // campo corregir. Una prioridad fuera de la API también se rechaza.
  it("Campos obligatorios", () => {
    expectMessageMentions(validateStoryTitle(""), "título");
    expectMessageMentions(validateStoryTitle("   \t "), "título");
    expectMessageMentions(validateStoryPriority(""), "prioridad");
    expectMessageMentions(validateStoryPriority("urgent"), "prioridad");
    expectMessageMentions(validateStoryPriority("High"), "prioridad");
  });

  // Caso de error: un criterio vacío o compuesto solo por espacios se rechaza indicando que cada
  // criterio debe tener contenido.
  it("Criterio vacío", () => {
    for (const input of ["", "   ", "\t \n"]) {
      expectMessageMentions(
        validateAcceptanceCriterion(input),
        "criterio",
        "contenido",
      );
    }
  });
});

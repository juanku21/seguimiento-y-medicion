import { describe, expect, it } from "vitest";
import { normalizeEmail } from "@/utils/normalize-email";
import {
  validateEmail,
  validateFullName,
  validatePassword,
} from "@/features/auth/validation";

// Pruebas unitarias obligatorias de las validaciones de entrada del formulario de registro (T028).
// Replican en TypeScript las reglas que el backend ya prueba en validation_test.go (RN-01 a RN-04,
// FR-003): los caracteres se cuentan por punto de código y el tope de la contraseña también se
// mide en bytes UTF-8.

// Verifica que el mensaje de error exista y nombre todas las piezas esperadas, sin distinguir
// mayúsculas de minúsculas.
function expectMessageMentions(message: string | null, ...parts: string[]) {
  expect(message, "se esperaba un mensaje de error").not.toBeNull();
  const lower = (message ?? "").toLowerCase();
  for (const part of parts) {
    expect(lower).toContain(part.toLowerCase());
  }
}

// Arma un correo sintácticamente válido de exactamente n caracteres (n >= 70), con una parte
// local de 64 caracteres y un dominio de etiquetas de hasta 63 caracteres.
function buildEmail(n: number): string {
  const local = "a".repeat(64);
  let remaining = n - local.length - 1 - ".com".length;
  const labels: string[] = [];
  while (remaining > 0) {
    let size = Math.min(remaining, 63);
    // Si sobra un único carácter tras una etiqueta llena, se reserva para el punto separador.
    if (remaining > 63 && remaining - 63 < 2) {
      size = 60;
    }
    labels.push("b".repeat(size));
    remaining -= size + 1;
  }
  const email = `${local}@${labels.join(".")}.com`;
  if (email.length !== n) {
    throw new Error(
      `buildEmail generó ${email.length} caracteres en lugar de ${n}`,
    );
  }
  return email;
}

// Cuenta los bytes UTF-8 de un valor, para documentar los casos límite en los propios datos.
function utf8Bytes(value: string): number {
  return new TextEncoder().encode(value).length;
}

describe("001/US1 - Registro de cuenta", () => {
  // Los datos del caso normal de la spec superan las tres validaciones.
  it("Alta exitosa", () => {
    expect(validateFullName("Ana Pérez")).toBeNull();
    expect(validateEmail("ana@example.com")).toBeNull();
    expect(validatePassword("Proyecto2026")).toBeNull();
  });

  // RN-01 y RN-02: recorte de espacios extremos y minúsculas, sin tocar puntos ni alias.
  it("Correo normalizado", () => {
    const cases: Array<[string, string]> = [
      ["  Ana@Example.COM  ", "ana@example.com"],
      ["ANA@example.com", "ana@example.com"],
      ["\tAna@Example.com\n", "ana@example.com"],
      ["ana@example.com", "ana@example.com"],
      ["Ana.Perez+Equipo@Example.com", "ana.perez+equipo@example.com"],
    ];
    for (const [input, expected] of cases) {
      expect(
        normalizeEmail(input),
        `normalizeEmail(${JSON.stringify(input)})`,
      ).toBe(expected);
    }
  });

  // RN-04: se aceptan 8 y 72 caracteres con letra y número. Un emoji cuenta como un solo
  // carácter y una contraseña de exactamente 72 bytes en UTF-8 es válida.
  it("Límites de contraseña", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 8 caracteres", "abcdefg1"],
      ["exactamente 72 caracteres", "a".repeat(71) + "1"],
      ["8 caracteres con 6 emojis (14 unidades UTF-16)", "😀".repeat(6) + "a1"],
      [
        "37 caracteres acentuados que suman exactamente 72 bytes",
        "á".repeat(35) + "a1",
      ],
    ];
    for (const [name, input] of valid) {
      expect(
        validatePassword(input),
        `${name}: ${[...input].length} caracteres, ${utf8Bytes(input)} bytes`,
      ).toBeNull();
    }
  });

  // Un emoji es un solo carácter: 7 caracteres con emojis no llegan al mínimo de 8 aunque ocupen
  // 12 unidades UTF-16 (value.length) y 22 bytes.
  it("Límites de contraseña: un emoji cuenta como un solo carácter", () => {
    const sevenWithEmojis = "😀".repeat(5) + "a1";
    expect(sevenWithEmojis.length).toBe(12);
    expectMessageMentions(
      validatePassword(sevenWithEmojis),
      "contraseña",
      "8",
      "72",
    );
  });

  // RN-03: se aceptan 2 y 100 caracteres tras recortar los espacios extremos, aunque sean
  // multibyte o emojis; se rechazan 1 y 101 con el rango permitido.
  it("Límites de nombre", () => {
    const valid: Array<[string, string]> = [
      ["exactamente 2 caracteres", "Al"],
      ["exactamente 100 caracteres", "a".repeat(100)],
      ["100 caracteres acentuados", "é".repeat(100)],
      ["100 emojis (200 unidades UTF-16)", "😀".repeat(100)],
      ["100 caracteres rodeados de espacios", "   " + "a".repeat(100) + "   "],
      ["2 caracteres rodeados de espacios", "  Al  "],
    ];
    for (const [name, input] of valid) {
      expect(validateFullName(input), name).toBeNull();
    }

    const invalid = [
      "A",
      "a".repeat(101),
      "   A   ",
      "é".repeat(101),
      "😀".repeat(101),
    ];
    for (const input of invalid) {
      expectMessageMentions(validateFullName(input), "nombre", "2", "100");
    }
  });

  // FR-003: se acepta un correo válido de exactamente 254 caracteres y se rechaza uno de 255.
  it("Correo de hasta 254 caracteres", () => {
    expect(validateEmail(buildEmail(254))).toBeNull();
    expectMessageMentions(validateEmail(buildEmail(255)), "correo");
  });

  // Caso de error 8: nombre ausente o solo de espacios, correo ausente o con formato inválido.
  it("Entrada inválida: nombre y correo", () => {
    expectMessageMentions(validateFullName(""), "nombre");
    expectMessageMentions(validateFullName("     "), "nombre");

    const invalidEmails = [
      "",
      "ana.example.com",
      "ana@",
      "@example.com",
      "ana perez@example.com",
      "Ana <ana@example.com>",
    ];
    for (const input of invalidEmails) {
      expectMessageMentions(validateEmail(input), "correo");
    }
  });

  // Contraseña fuera de rango, en caracteres o en bytes: todas comparten el mensaje del rango y
  // ninguna se repite en el mensaje. Incluye la contraseña de 41 caracteres (40 letras
  // acentuadas y un número) que supera los 72 bytes.
  it("Entrada inválida: contraseña fuera del rango de 8 a 72 caracteres o de más de 72 bytes", () => {
    const reference = validatePassword("a".repeat(72) + "1");
    expectMessageMentions(reference, "contraseña", "8", "72");

    const rangeCases = [
      "",
      "abcdef1",
      "ááááá1a",
      "a".repeat(72) + "1",
      "á".repeat(40) + "1",
      "ñ".repeat(71) + "1",
    ];
    for (const input of rangeCases) {
      const message = validatePassword(input);
      expectMessageMentions(message, "contraseña", "8", "72");
      expect(message).toBe(reference);
      if (input !== "") {
        expect(message ?? "").not.toContain(input);
      }
    }
  });

  // El caso de 40 letras acentuadas y un número tiene 41 caracteres pero 81 bytes UTF-8.
  it("Entrada inválida: contraseña de 72 caracteres o menos que supera los 72 bytes en UTF-8", () => {
    const accented = "á".repeat(40) + "1";
    expect([...accented].length).toBe(41);
    expect(utf8Bytes(accented)).toBe(81);
    expectMessageMentions(validatePassword(accented), "contraseña", "8", "72");
  });

  // Composición mínima: sin letra o sin número, con un mensaje distinto del de rango.
  it("Entrada inválida: contraseña sin letra o sin número", () => {
    const reference = validatePassword("a".repeat(72) + "1");
    for (const input of ["12345678", "abcdefgh", "ñandúesgrande"]) {
      const message = validatePassword(input);
      expectMessageMentions(message, "contraseña", "letra", "número");
      expect(message).not.toBe(reference);
      expect(message ?? "").not.toContain(input);
    }
  });
});

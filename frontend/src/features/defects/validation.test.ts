import { describe, expect, it } from "vitest";
import {
  toDefectFieldErrors,
  validateDefectDescription,
  validateDefectFields,
  validateDefectSeverity,
} from "@/features/defects/validation";

// Pruebas unitarias de las reglas de forma del alta de un defecto (T013; research.md sección 6).
// Son las mismas reglas y los mismos mensajes que backend/internal/defects/delivery/validation.go.
// Es cobertura de validación sin escenario propio visible: el describe no lleva el prefijo 007/USn.

const msgDescriptionRequired = "La descripción es obligatoria.";
const msgDescriptionTooLong =
  "La descripción admite como máximo 2000 caracteres.";
const msgSeverityInvalid = "La severidad debe ser Crítica, Alta, Media o Baja.";
const msgStoryRequired = "La historia relacionada es obligatoria.";
const msgSprintRequired = "El sprint de detección es obligatorio.";

const storyId = "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10";
const sprintId = "9b1d4e2a-0c3f-4a8e-b6d5-7e2f1a0c3b44";

describe("Validación de forma de la descripción del defecto", () => {
  // Una descripción con texto es válida.
  it("acepta una descripción con texto", () => {
    expect(
      validateDefectDescription("El total del reporte no suma los descuentos"),
    ).toBeNull();
  });

  // Vacía o solo con espacios se rechaza como obligatoria.
  it("rechaza la descripción vacía o compuesta solo por espacios", () => {
    expect(validateDefectDescription("")).toBe(msgDescriptionRequired);
    expect(validateDefectDescription("   \t ")).toBe(msgDescriptionRequired);
  });

  // El máximo son 2000 caracteres tras recortar, contados por punto de código.
  it("acepta 2000 caracteres y rechaza 2001", () => {
    expect(validateDefectDescription("á".repeat(2000))).toBeNull();
    expect(validateDefectDescription("á".repeat(2001))).toBe(
      msgDescriptionTooLong,
    );
  });

  // Los espacios extremos no cuentan para el máximo.
  it("recorta los espacios extremos antes de contar", () => {
    expect(validateDefectDescription(`  ${"a".repeat(2000)}  `)).toBeNull();
  });

  // Un carácter fuera del plano básico cuenta como uno solo.
  it("cuenta por punto de código", () => {
    expect(validateDefectDescription("😀".repeat(2000))).toBeNull();
  });
});

describe("Validación de forma de la severidad del defecto", () => {
  // Las cuatro severidades admitidas son válidas.
  it("acepta las cuatro severidades", () => {
    for (const severity of ["critical", "high", "medium", "low"]) {
      expect(validateDefectSeverity(severity)).toBeNull();
    }
  });

  // Cualquier otro valor, incluido el vacío y el texto en español, se rechaza.
  it("rechaza cualquier otro valor", () => {
    for (const severity of ["", "urgent", "Alta", "HIGH"]) {
      expect(validateDefectSeverity(severity)).toBe(msgSeverityInvalid);
    }
  });
});

describe("Validación de forma de todos los campos del defecto", () => {
  // Con todos los campos válidos no hay errores.
  it("no devuelve errores si todo es válido", () => {
    expect(
      validateDefectFields({
        description: "Algo falla",
        severity: "high",
        storyId,
        detectedSprintId: sprintId,
      }),
    ).toEqual({});
  });

  // Devuelve a la vez todos los campos inválidos, cada uno con su mensaje.
  it("devuelve los cuatro campos inválidos a la vez", () => {
    expect(
      validateDefectFields({
        description: "  ",
        severity: "",
        storyId: "",
        detectedSprintId: "",
      }),
    ).toEqual({
      description: msgDescriptionRequired,
      severity: msgSeverityInvalid,
      storyId: msgStoryRequired,
      detectedSprintId: msgSprintRequired,
    });
  });
});

describe("Detalle por campo del 400 del backend", () => {
  // Ubica cada mensaje en su campo y descarta los campos que no son del defecto.
  it("ubica los mensajes junto a cada campo y descarta los ajenos", () => {
    expect(
      toDefectFieldErrors([
        { field: "description", message: msgDescriptionRequired },
        { field: "detectedSprintId", message: "Sprint planificado." },
        { field: "projectId", message: "No corresponde." },
      ]),
    ).toEqual({
      description: msgDescriptionRequired,
      detectedSprintId: "Sprint planificado.",
    });
  });
});

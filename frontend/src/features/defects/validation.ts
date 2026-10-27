// Reglas de forma del alta de un defecto, coherentes con
// backend/internal/defects/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido. Las reglas que dependen de datos (vínculos del proyecto,
// sprint usable) las decide el backend.
import type { FieldError } from "@/types/auth";

// Valores de los campos del formulario tal como se escriben; severity queda como el texto elegido
// hasta validarlo.
export type DefectFormValues = {
  description: string;
  severity: string;
  storyId: string;
  detectedSprintId: string;
};

export type DefectFieldName = keyof DefectFormValues;
export type DefectFieldErrors = Partial<Record<DefectFieldName, string>>;

const DESCRIPTION_MAX_LENGTH = 2000;
const SEVERITIES = ["critical", "high", "medium", "low"];
const FIELD_NAMES: DefectFieldName[] = [
  "description",
  "severity",
  "storyId",
  "detectedSprintId",
];

// Valida la descripción: obligatoria y de hasta 2000 caracteres tras recortar los espacios.
export function validateDefectDescription(value: string): string | null {
  const trimmed = value.trim();
  if (trimmed === "") {
    return "La descripción es obligatoria.";
  }
  // Se cuenta por punto de código, no por unidad UTF-16.
  if ([...trimmed].length > DESCRIPTION_MAX_LENGTH) {
    return "La descripción admite como máximo 2000 caracteres.";
  }
  return null;
}

// Valida la severidad: solo critical, high, medium o low.
export function validateDefectSeverity(value: string): string | null {
  if (!SEVERITIES.includes(value)) {
    return "La severidad debe ser Crítica, Alta, Media o Baja.";
  }
  return null;
}

// Aplica las reglas de forma a los cuatro campos y devuelve solo los que tienen error.
export function validateDefectFields(
  values: DefectFormValues,
): DefectFieldErrors {
  const errors: DefectFieldErrors = {};
  const description = validateDefectDescription(values.description);
  if (description) errors.description = description;
  const severity = validateDefectSeverity(values.severity);
  if (severity) errors.severity = severity;
  if (values.storyId.trim() === "") {
    errors.storyId = "La historia relacionada es obligatoria.";
  }
  if (values.detectedSprintId.trim() === "") {
    errors.detectedSprintId = "El sprint de detección es obligatorio.";
  }
  return errors;
}

// Ubica junto a cada campo el detalle de un 400 del backend, descartando los que no son del defecto.
export function toDefectFieldErrors(fields: FieldError[]): DefectFieldErrors {
  const errors: DefectFieldErrors = {};
  for (const { field, message } of fields) {
    const name = FIELD_NAMES.find((candidate) => candidate === field);
    if (name) errors[name] = message;
  }
  return errors;
}

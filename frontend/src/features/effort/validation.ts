// Reglas de forma del alta y la corrección de esfuerzo, coherentes con
// backend/internal/effort/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido. Las reglas que dependen de datos (fecha futura o anterior al
// inicio del proyecto, tope diario) las decide el backend.
import type { FieldError } from "@/types/auth";

// Valores de los campos de esfuerzo tal como se escriben, compartidos por el formulario de alta y la
// edición en línea: las horas quedan como el texto del campo numérico hasta validarlas.
export type EffortFormValues = {
  entryDate: string;
  activity: string;
  hours: string;
};

export type EffortFieldName = keyof EffortFormValues;
export type EffortFieldErrors = Partial<Record<EffortFieldName, string>>;

const effortFieldNames: readonly EffortFieldName[] = [
  "entryDate",
  "activity",
  "hours",
];

// Indicación de la escala de horas, visible desde el inicio en el alta y en la edición (RN-02).
export const EFFORT_HOURS_HINT = "Entre 0,25 y 24, en múltiplos de 0,25.";

// Límites de forma y mensajes, idénticos a los del backend (research.md sección 6).
const maxActivityLength = 300;
const maxEntryHundredths = 2400;
const hoursStepHundredths = 25;
const msgHoursRange = "Las horas deben ser mayores que 0 y como máximo 24.";
const msgHoursMultiple =
  "Las horas se cargan en múltiplos de 0,25 (un cuarto de hora).";
const msgHoursNumber = "Las horas son obligatorias y deben ser un número.";
const msgActivityRequired = "La actividad es obligatoria.";
const msgActivityTooLong = "La actividad admite como máximo 300 caracteres.";
const msgEntryDateInvalid =
  "La fecha es obligatoria y debe tener el formato AAAA-MM-DD.";

// Formas admitidas: la fecha AAAA-MM-DD y las horas con signo menos opcional, dígitos y hasta dos
// decimales, sin exponente.
const entryDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const hoursPattern = /^(-?)(\d+)(?:\.(\d{1,2}))?$/;

// Valida la fecha del registro: obligatoria, fecha real con el formato AAAA-MM-DD. El día se
// reconstruye en UTC y debe coincidir con el indicado, de modo que 2026-02-30 se rechaza.
export function validateEffortEntryDate(value: string): string | null {
  const match = entryDatePattern.exec(value);
  if (match === null) {
    return msgEntryDateInvalid;
  }
  const [year, month, day] = match.slice(1).map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  const isRealDay =
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day;
  return isRealDay ? null : msgEntryDateInvalid;
}

// Valida la actividad: obligatoria y de hasta 300 caracteres tras recortar los espacios extremos,
// contados por punto de código como utf8.RuneCountInString en el backend.
export function validateEffortActivity(value: string): string | null {
  const activity = value.trim();
  if (activity === "") {
    return msgActivityRequired;
  }
  return [...activity].length > maxActivityLength ? msgActivityTooLong : null;
}

// Valida las horas tal como llegan del campo: número con a lo sumo 2 decimales, mayor que 0, como
// máximo 24 y múltiplo de 0,25. El cálculo se hace en centésimas enteras, sin aritmética de coma
// flotante, igual que ParseHours en el backend.
export function validateEffortHours(value: string): string | null {
  const match = hoursPattern.exec(value.trim());
  if (match === null) {
    return msgHoursNumber;
  }
  const [, sign, whole, fraction = ""] = match;
  const hundredths = Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
  if (sign === "-" || hundredths <= 0 || hundredths > maxEntryHundredths) {
    return msgHoursRange;
  }
  return hundredths % hoursStepHundredths !== 0 ? msgHoursMultiple : null;
}

// Aplica las reglas de forma a los tres campos y devuelve solo los que tienen error.
export function validateEffortFields(
  values: EffortFormValues,
): EffortFieldErrors {
  const errors: EffortFieldErrors = {};
  const checks: [EffortFieldName, string | null][] = [
    ["entryDate", validateEffortEntryDate(values.entryDate)],
    ["activity", validateEffortActivity(values.activity)],
    ["hours", validateEffortHours(values.hours)],
  ];
  for (const [field, message] of checks) {
    if (message) {
      errors[field] = message;
    }
  }
  return errors;
}

// Indica si un nombre de campo informado por el backend corresponde a un campo de esfuerzo.
function isEffortFieldName(value: string): value is EffortFieldName {
  return effortFieldNames.some((name) => name === value);
}

// Ubica junto a cada campo el detalle de un 400 del backend, descartando los que no son de esfuerzo.
export function toEffortFieldErrors(fields: FieldError[]): EffortFieldErrors {
  const errors: EffortFieldErrors = {};
  for (const { field, message } of fields) {
    if (isEffortFieldName(field)) {
      errors[field] = message;
    }
  }
  return errors;
}

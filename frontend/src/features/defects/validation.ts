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

// Valida la descripción: obligatoria y de hasta 2000 caracteres tras recortar los espacios.
export function validateDefectDescription(value: string): string | null {
  void value;
  return null;
}

// Valida la severidad: solo critical, high, medium o low.
export function validateDefectSeverity(value: string): string | null {
  void value;
  return null;
}

// Aplica las reglas de forma a los cuatro campos y devuelve solo los que tienen error.
export function validateDefectFields(
  values: DefectFormValues,
): DefectFieldErrors {
  void values;
  return {};
}

// Ubica junto a cada campo el detalle de un 400 del backend, descartando los que no son del defecto.
export function toDefectFieldErrors(fields: FieldError[]): DefectFieldErrors {
  void fields;
  return {};
}

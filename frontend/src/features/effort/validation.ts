// Reglas de forma del alta de esfuerzo, coherentes con backend/internal/effort/delivery/validation.go.
// Cada función devuelve el mensaje de error en español o null si el valor es válido.
// Firma mínima de T018: la implementación corresponde a T032.

// Valida la fecha del registro: obligatoria, fecha real con el formato AAAA-MM-DD.
export function validateEffortEntryDate(value: string): string | null {
  throw new Error("not implemented");
}

// Valida la actividad: obligatoria y de hasta 300 caracteres tras recortar los espacios extremos.
export function validateEffortActivity(value: string): string | null {
  throw new Error("not implemented");
}

// Valida las horas tal como llegan del campo: número con a lo sumo 2 decimales, mayor que 0, como
// máximo 24 y múltiplo de 0,25.
export function validateEffortHours(value: string): string | null {
  throw new Error("not implemented");
}

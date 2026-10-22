// Reglas del formulario de sprint (FR-002 a FR-004), coherentes con
// backend/internal/sprints/delivery/validation.go. Cada función devuelve el mensaje de error en
// español o null si el valor es válido. Firmas mínimas de T014: la implementación corresponde a T022.

// Valida el nombre del sprint: entre 1 y 100 caracteres tras recortar los espacios extremos.
export function validateSprintName(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida el Sprint Goal: entre 1 y 500 caracteres tras recortar los espacios extremos.
export function validateSprintGoal(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida la fecha de inicio: obligatoria y con formato AAAA-MM-DD.
export function validateSprintStartDate(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida la fecha de fin: obligatoria, con formato AAAA-MM-DD y estrictamente posterior a la de
// inicio.
export function validateSprintEndDate(
  startDate: string,
  endDate: string,
): string | null {
  void startDate;
  void endDate;
  throw new Error("not implemented");
}

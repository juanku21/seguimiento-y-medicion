// Firmas mínimas de las reglas del formulario de proyecto (RN-04, RN-06, RN-07, FR-003). La
// implementación corresponde a la tarea T029; cada función devolverá el mensaje de error en español
// o null si el valor es válido.

// Valida el nombre del proyecto: entre 1 y 100 caracteres tras recortar los espacios extremos.
export function validateProjectName(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida la descripción opcional: hasta 1000 caracteres tras recortar los espacios extremos.
export function validateDescription(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida que la fecha de inicio (YYYY-MM-DD) esté presente.
export function validateStartDate(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida que la fecha de finalización prevista esté presente y no sea anterior a la de inicio.
export function validatePlannedEndDate(
  startDate: string,
  plannedEndDate: string,
): string | null {
  void startDate;
  void plannedEndDate;
  throw new Error("not implemented");
}

// Valida el factor tal como se escribió: mayor que 0, hasta 40 y con hasta 2 decimales, sin
// redondear.
export function validateHoursPerStoryPoint(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

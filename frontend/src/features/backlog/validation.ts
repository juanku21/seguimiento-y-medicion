// Firmas mínimas de las reglas del formulario de historia (FR-002 a FR-004, FR-007). La
// implementación corresponde a la tarea T026; cada función devolverá el mensaje de error en español
// o null si el valor es válido.

// Valida el título: entre 1 y 150 caracteres tras recortar los espacios extremos.
export function validateStoryTitle(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida la descripción opcional: hasta 2000 caracteres tras recortar los espacios extremos.
export function validateStoryDescription(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida que la prioridad sea uno de los valores admitidos de la API: high, medium o low.
export function validateStoryPriority(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida un criterio de aceptación: entre 1 y 500 caracteres tras recortar los espacios extremos.
export function validateAcceptanceCriterion(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

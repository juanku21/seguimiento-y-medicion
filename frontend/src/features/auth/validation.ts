// Reglas de RC-02 del formulario de registro. Cada función devuelve el mensaje de error en
// español o null si el valor es válido. Firmas mínimas creadas por T028; la implementación
// corresponde a T037.

// Valida el nombre completo (RN-03, FR-002).
export function validateFullName(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida el formato y la longitud del correo (FR-003).
export function validateEmail(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

// Valida el rango, el tope de bytes y la composición de la contraseña (RN-04, FR-006).
export function validatePassword(value: string): string | null {
  void value;
  throw new Error("not implemented");
}

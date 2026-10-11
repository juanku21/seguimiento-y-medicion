// Normaliza el correo igual que el backend: recorte de espacios extremos y minúsculas, sin alterar
// puntos ni alias (FR-004, RN-02).
export function normalizeEmail(value: string): string {
  return value.trim().toLowerCase();
}

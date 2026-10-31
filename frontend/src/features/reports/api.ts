// Acceso a la API de reportes y reconocedores de sus respuestas (Principio VII).
import type { Report } from "@/types/report";

// Reconoce un cuerpo de reporte válido; firma mínima, sin lógica todavía.
export function isReport(value: unknown): value is Report {
  void value;
  throw new Error("not implemented");
}

"use client";

import type { ReactNode } from "react";
import { useRequireSession } from "@/hooks/use-session";

// Envoltorio de cliente que aplica la guardia de sesión de la 001 a una pantalla cuya página es un
// Server Component (research.md, "Protección de rutas del frontend"). Sin sesión, la guardia lleva
// a /login y el contenido no se muestra; con sesión, renderiza el contenido tal cual.
export function SessionGuard({ children }: { children: ReactNode }) {
  const token = useRequireSession();
  if (token === null) {
    return null;
  }
  return children;
}

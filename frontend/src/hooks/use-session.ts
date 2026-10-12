"use client";

import { useContext } from "react";
import {
  SessionContext,
  type SessionState,
} from "@/providers/session-provider";

// Acceso al estado de sesión desde cualquier Client Component. Falla con un mensaje claro si el
// componente no está dentro de SessionProvider, en lugar de devolver un estado vacío engañoso.
export function useSession(): SessionState {
  const session = useContext(SessionContext);
  if (session === null) {
    throw new Error("useSession debe usarse dentro de SessionProvider");
  }
  return session;
}

// Guardia de ruta del lado del cliente (research.md, "Protección de rutas del frontend"): devuelve
// el token vigente y lleva a /login cuando no hay sesión. Firma mínima de T056; la implementa T061.
export function useRequireSession(): string | null {
  throw new Error("not implemented");
}

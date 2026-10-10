"use client";

import type { ReactNode } from "react";

// Contexto de sesión de la aplicación. Firma mínima creada por T043 para que
// compilen las pruebas; la implementación corresponde a T051.
export function SessionProvider({
  children,
}: {
  children: ReactNode;
}): ReactNode {
  void children;
  throw new Error("not implemented");
}

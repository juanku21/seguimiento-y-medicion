"use client";

import {
  createContext,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { readToken, subscribeToken, writeToken } from "@/lib/session-storage";

// Estado de sesión que comparten los componentes de la aplicación: el token vigente de la pestaña
// (o null si no hay sesión) y la acción que lo guarda al iniciar sesión.
export type SessionState = {
  token: string | null;
  startSession: (token: string) => void;
};

// Contexto de sesión; vale null fuera del proveedor para que use-session pueda detectarlo.
export const SessionContext = createContext<SessionState | null>(null);

// En el prerenderizado del servidor y en la hidratación no hay sesión que leer: devolver null en
// ambos evita la discrepancia de hidratación, y React relee sessionStorage al terminar de hidratar.
function getServerToken(): string | null {
  return null;
}

// Proveedor de sesión montado en el layout raíz. Lee el token de sessionStorage (research.md,
// sección 9) y se vuelve a renderizar cada vez que session-storage avisa un cambio: al iniciar
// sesión o cuando el cliente HTTP borra el token ante un 401.
export function SessionProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(subscribeToken, readToken, getServerToken);

  const value = useMemo(() => ({ token, startSession: writeToken }), [token]);

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

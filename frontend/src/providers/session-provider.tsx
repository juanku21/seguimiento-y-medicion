"use client";

import {
  createContext,
  useCallback,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { readToken, writeToken } from "@/lib/session-storage";

// Estado de sesión que comparten los componentes de la aplicación: el token vigente de la pestaña
// (o null si no hay sesión) y la acción que lo guarda al iniciar sesión.
export type SessionState = {
  token: string | null;
  startSession: (token: string) => void;
};

// Contexto de sesión; vale null fuera del proveedor para que use-session pueda detectarlo.
export const SessionContext = createContext<SessionState | null>(null);

// Suscriptores que se vuelven a renderizar cuando cambia el token. sessionStorage no emite eventos
// en la misma pestaña, así que el aviso lo hace startSession después de escribir.
const listeners = new Set<() => void>();

// Registra un suscriptor y devuelve la función que lo da de baja, como pide useSyncExternalStore.
function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// En el prerenderizado del servidor y en la hidratación no hay sesión que leer: devolver null en
// ambos evita la discrepancia de hidratación, y React relee sessionStorage al terminar de hidratar.
function getServerToken(): string | null {
  return null;
}

// Proveedor de sesión montado en el layout raíz. Lee el token de sessionStorage (research.md,
// sección 9) y expone la acción que lo guarda y avisa a los componentes suscriptos.
export function SessionProvider({ children }: { children: ReactNode }) {
  const token = useSyncExternalStore(subscribe, readToken, getServerToken);

  const startSession = useCallback((newToken: string) => {
    writeToken(newToken);
    listeners.forEach((listener) => listener());
  }, []);

  const value = useMemo(() => ({ token, startSession }), [token, startSession]);

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

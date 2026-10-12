"use client";

import { useRouter } from "next/navigation";
import { useContext, useEffect } from "react";
import { readToken, wasSessionRejected } from "@/lib/session-storage";
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

// Destinos de la guardia: el inicio de sesión a secas para quien no tiene sesión, y con el motivo
// que activa el aviso de volver a iniciar sesión cuando la API rechazó la sesión (FR-016).
const LOGIN_PATH = "/login";
const SESSION_REJECTED_LOGIN_PATH = "/login?motivo=sesion";

// Guardia de ruta del lado del cliente (research.md, "Protección de rutas del frontend"): devuelve
// el token vigente y lleva a /login cuando no hay sesión. El efecto relee sessionStorage en lugar
// de confiar en el token del contexto, que durante la hidratación vale null aunque haya sesión.
export function useRequireSession(): string | null {
  const { token } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (readToken() !== null) {
      return;
    }
    router.replace(
      wasSessionRejected() ? SESSION_REJECTED_LOGIN_PATH : LOGIN_PATH,
    );
  }, [token, router]);

  return token;
}

"use client";

import { LoadingState } from "@/components/loading-state";
import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getOwnProfile } from "@/features/auth/api";
import { useRequireSession } from "@/hooks/use-session";
import type { UserProfile } from "@/types/auth";

// Estado de la consulta: todavía sin respuesta, el perfil recibido o el mensaje de error.
type ProfileState =
  | { status: "loading" }
  | { status: "loaded"; profile: UserProfile }
  | { status: "error"; message: string };

// Tarjeta con los datos del propio perfil (001/US4). No recibe props: con la guardia de sesión
// obtiene el token vigente y, cuando lo hay, pide el perfil del titular sin ningún identificador de
// cuenta (FR-023). Solo muestra el nombre completo y el correo, aunque la respuesta trajera otros
// campos (FR-011). Si la API rechaza la sesión, el cliente HTTP borra el token y la guardia lleva a
// /login con el aviso correspondiente (FR-016).
export function ProfileCard() {
  const token = useRequireSession();
  const [state, setState] = useState<ProfileState>({ status: "loading" });

  // Pide el perfil cada vez que hay un token vigente. La bandera descarta la respuesta si el
  // componente se desmontó o el token cambió antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getOwnProfile().then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", profile: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return <LoadingState>Cargando perfil…</LoadingState>;
  }

  return (
    <dl className="flex flex-col gap-4 rounded-lg border border-border bg-surface px-5 py-4">
      <div className="flex flex-col gap-1">
        <dt className="text-sm text-foreground-muted">Nombre completo</dt>
        <dd className="text-base font-medium">{state.profile.fullName}</dd>
      </div>
      <div className="flex flex-col gap-1">
        <dt className="text-sm text-foreground-muted">Correo electrónico</dt>
        <dd className="text-base font-medium break-all">
          {state.profile.email}
        </dd>
      </div>
    </dl>
  );
}

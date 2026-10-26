"use client";

import { useEffect, useId, useState } from "react";
import { Alert } from "@/components/alert";
import { getOwnProfile } from "@/features/auth/api";
import { getStoryEffort } from "@/features/effort/api";
import { EffortBreakdown } from "@/features/effort/components/effort-breakdown";
import { EffortEntryForm } from "@/features/effort/components/effort-entry-form";
import { EffortEntryList } from "@/features/effort/components/effort-entry-list";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { useRequireSession } from "@/hooks/use-session";
import type { EffortEntry, StoryEffort } from "@/types/effort";
import { formatHours } from "@/utils/effort-format";
import { formatDate } from "@/utils/format-date";

// Propiedades de la sección "Esfuerzo" del detalle de una historia: la historia tomada de la ruta.
export type StoryEffortSectionProps = {
  storyId: string;
};

// Estado de la consulta del esfuerzo: todavía sin respuesta, el esfuerzo recibido o el mensaje de
// error.
type EffortState =
  | { status: "loading" }
  | { status: "loaded"; effort: StoryEffort }
  | { status: "error"; message: string };

// Sección "Esfuerzo" del detalle de una historia, todo en una sola pantalla (SC-002): ofrece el
// formulario de registro (006/US1), confirma el último registro dado de alta y muestra el total, el
// desglose por persona y la lista de registros (006/US2), que se vuelven a pedir después de cada
// alta, corrección o eliminación para reflejarla. Identifica a quien tiene la sesión para ofrecer
// la corrección y la eliminación de sus registros (006/US3).
export function StoryEffortSection({ storyId }: StoryEffortSectionProps) {
  const headingId = useId();
  const token = useRequireSession();
  const [lastEntry, setLastEntry] = useState<EffortEntry | null>(null);
  const [state, setState] = useState<EffortState>({ status: "loading" });
  const [reloadCount, setReloadCount] = useState(0);
  const profile = useAuthorizedQuery(getOwnProfile);
  const currentUserId =
    profile.status === "loaded" ? profile.data.id : undefined;

  // Pide el esfuerzo con un token vigente, al cambiar la historia y después de cada alta, corrección
  // o eliminación (reloadCount). Mientras llega una recarga se sigue mostrando el esfuerzo anterior, y la bandera
  // descarta la respuesta si el componente se desmontó o el pedido quedó viejo.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getStoryEffort(storyId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", effort: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, storyId, reloadCount]);

  // Registra el alta para confirmarla y fuerza la recarga del total, el desglose y la lista.
  function handleCreated(entry: EffortEntry) {
    setLastEntry(entry);
    reload();
  }

  // Fuerza la recarga del total, el desglose y la lista, por ejemplo tras corregir o eliminar.
  function reload() {
    setReloadCount((count) => count + 1);
  }

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <h2 id={headingId} className="text-lg font-semibold tracking-tight">
        Esfuerzo
      </h2>
      {state.status === "loading" && (
        <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
          Cargando esfuerzo…
        </p>
      )}
      {state.status === "error" && <Alert>{state.message}</Alert>}
      {state.status === "loaded" && (
        <>
          <EffortBreakdown
            totalHours={state.effort.totalHours}
            byMember={state.effort.byMember}
          />
          <EffortEntryList
            entries={state.effort.entries}
            currentUserId={currentUserId}
            onChanged={reload}
          />
        </>
      )}
      {lastEntry && (
        <Alert variant="success">
          Se registraron {formatHours(lastEntry.hours)} el{" "}
          {formatDate(lastEntry.entryDate)}.
        </Alert>
      )}
      <EffortEntryForm storyId={storyId} onCreated={handleCreated} />
    </section>
  );
}

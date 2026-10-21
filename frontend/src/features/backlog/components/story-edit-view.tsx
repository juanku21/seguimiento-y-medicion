"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getStory } from "@/features/backlog/api";
import { StoryForm } from "@/features/backlog/components/story-form";
import { useRequireSession } from "@/hooks/use-session";
import type { StoryDetail } from "@/types/story";

// Propiedades de la vista de refinamiento: el identificador de la historia tomado de la ruta.
export type StoryEditViewProps = {
  storyId: string;
};

// Estado de la consulta: todavía sin respuesta, la historia recibida o el mensaje de error.
type EditState =
  | { status: "loading" }
  | { status: "loaded"; story: StoryDetail }
  | { status: "error"; message: string };

// Vista de refinamiento de una historia (003/US5): pide la historia con un token vigente y monta el
// formulario precargado con sus valores, que envía solo los campos cambiados. La historia ajena y la
// inexistente llegan con el mismo 404 y se muestra solo su mensaje, sin formulario (FR-047); ante un
// 401 el cliente HTTP borra el token y la guardia lleva a /login.
export function StoryEditView({ storyId }: StoryEditViewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<EditState>({ status: "loading" });

  // Pide la historia cada vez que hay un token vigente o cambia la historia. La bandera descarta la
  // respuesta si el componente se desmontó o el pedido quedó viejo antes de que llegara.
  useEffect(() => {
    if (token === null) {
      return;
    }
    let isCurrent = true;
    getStory(storyId).then((result) => {
      if (!isCurrent) {
        return;
      }
      setState(
        result.ok
          ? { status: "loaded", story: result.data }
          : { status: "error", message: result.error.message },
      );
    });
    return () => {
      isCurrent = false;
    };
  }, [token, storyId]);

  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }
  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando historia…
      </p>
    );
  }
  return <StoryForm story={state.story} />;
}

"use client";

import { useEffect, useState } from "react";
import { Alert } from "@/components/alert";
import { getStory } from "@/features/backlog/api";
import { DeleteStoryButton } from "@/features/backlog/components/delete-story-button";
import { EstimateControl } from "@/features/backlog/components/estimate-control";
import { StoryDetailCard } from "@/features/backlog/components/story-detail-card";
import { useRequireSession } from "@/hooks/use-session";
import type { StoryDetail } from "@/types/story";

// Propiedades de la vista de detalle: el identificador de la historia tomado de la ruta.
export type StoryDetailViewProps = {
  storyId: string;
};

// Estado de la consulta: todavía sin respuesta, la historia recibida o el mensaje de error.
type DetailState =
  | { status: "loading" }
  | { status: "loaded"; story: StoryDetail }
  | { status: "error"; message: string };

// Vista del detalle de una historia (003/US3): pide la historia con un token vigente y la muestra en
// la tarjeta de detalle, junto al control de estimación (003/US4), que reemplaza la historia
// mostrada por la que devuelve el backend al estimar, y al botón de eliminación (003/US6), que
// regresa al backlog al eliminarla. La historia ajena y la inexistente llegan con
// el mismo 404 y se muestra solo su mensaje, sin ningún dato (FR-047); ante un 401 el cliente HTTP
// borra el token y la guardia lleva a /login.
export function StoryDetailView({ storyId }: StoryDetailViewProps) {
  const token = useRequireSession();
  const [state, setState] = useState<DetailState>({ status: "loading" });

  // Pide el detalle cada vez que hay un token vigente o cambia la historia. La bandera descarta la
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
  return (
    <div className="flex flex-col gap-6">
      <StoryDetailCard story={state.story} />
      <EstimateControl
        storyId={state.story.id}
        storyPoints={state.story.storyPoints}
        onEstimated={(story) => setState({ status: "loaded", story })}
      />
      <DeleteStoryButton
        storyId={state.story.id}
        projectId={state.story.projectId}
      />
    </div>
  );
}

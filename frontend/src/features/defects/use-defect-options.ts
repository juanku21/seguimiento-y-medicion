import { useEffect, useState } from "react";
import { listStories } from "@/features/backlog/api";
import { listDetectionSprints } from "@/features/defects/api";
import type { SprintListItem } from "@/types/sprint";
import type { StoryListItem } from "@/types/story";

// Opciones de historia y de sprint de detección de un defecto, con el estado de la carga y el
// mensaje de error si alguna consulta falla.
export type DefectOptions = {
  stories: StoryListItem[];
  sprints: SprintListItem[];
  isLoading: boolean;
  error: string | null;
};

// Carga las historias del backlog y los sprints Activos y Cerrados del proyecto, con las mismas
// consultas para el alta y la corrección de un defecto. La bandera descarta la respuesta si el
// componente se desmontó antes de que llegara.
export function useDefectOptions(projectId: string): DefectOptions {
  const [options, setOptions] = useState<DefectOptions>({
    stories: [],
    sprints: [],
    isLoading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const [storiesResult, sprintsResult] = await Promise.all([
        listStories(projectId, { status: [], priority: [] }),
        listDetectionSprints(projectId),
      ]);
      if (cancelled) {
        return;
      }
      // Si fallan las dos consultas se informa el error de los sprints, la última en evaluarse.
      let error: string | null = null;
      if (!storiesResult.ok) {
        error = storiesResult.error.message;
      }
      if (!sprintsResult.ok) {
        error = sprintsResult.error.message;
      }
      setOptions({
        stories: storiesResult.ok ? storiesResult.data.stories : [],
        sprints: sprintsResult.ok ? sprintsResult.data.sprints : [],
        isLoading: false,
        error,
      });
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  return options;
}

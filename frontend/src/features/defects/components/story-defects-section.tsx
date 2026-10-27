"use client";

import Link from "next/link";
import { useCallback, useId, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { listStoryDefects } from "@/features/defects/api";
import { DefectRow } from "@/features/defects/components/defect-row";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { DEFECT_EMPTY_REASON_LABELS } from "@/utils/defect-labels";

// Propiedades de la sección: el proyecto y la historia cuyos defectos se muestran.
export type StoryDefectsSectionProps = {
  projectId: string;
  storyId: string;
};

// Sección "Defectos" del detalle de una historia (007/US3, SC-003): lista los defectos de la
// historia y ofrece siempre el enlace para registrar uno nuevo con la historia preseleccionada.
export function StoryDefectsSection({
  projectId,
  storyId,
}: StoryDefectsSectionProps): ReactElement {
  const headingId = useId();
  const load = useCallback(() => listStoryDefects(storyId), [storyId]);
  const query = useAuthorizedQuery(load);

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <h2 id={headingId} className="text-lg font-semibold tracking-tight">
        Defectos
      </h2>
      {query.status === "loading" && (
        <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
          Cargando defectos…
        </p>
      )}
      {query.status === "error" && <Alert>{query.message}</Alert>}
      {query.status === "loaded" && query.data.defects.length === 0 && (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          {DEFECT_EMPTY_REASON_LABELS.no_defects}
        </p>
      )}
      {query.status === "loaded" && query.data.defects.length > 0 && (
        <ul
          aria-label="Defectos de la historia"
          className="flex flex-col gap-3"
        >
          {query.data.defects.map((defect) => (
            <DefectRow key={defect.id} defect={defect} showStory={false} />
          ))}
        </ul>
      )}
      <Link
        href={`/projects/${projectId}/defects/new?storyId=${storyId}`}
        className="self-start text-sm font-medium underline underline-offset-4"
      >
        Registrar defecto
      </Link>
    </section>
  );
}

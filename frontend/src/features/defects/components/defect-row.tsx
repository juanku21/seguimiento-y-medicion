import Link from "next/link";
import type { ReactElement } from "react";
import type { Defect } from "@/types/defect";
import {
  DEFECT_SEVERITY_LABELS,
  DEFECT_STATUS_LABELS,
} from "@/utils/defect-labels";

// Propiedades de la fila: el defecto a mostrar. Con showStory en false se omite el título de la
// historia, para la sección que ya está dentro de ella.
export type DefectRowProps = {
  defect: Defect;
  showStory?: boolean;
};

// Fila de un defecto en un listado: descripción con enlace al detalle, severidad y estado en
// español, historia y sprints de detección y, solo si lo tiene, de resolución (FR-043, FR-044).
export function DefectRow({
  defect,
  showStory = true,
}: DefectRowProps): ReactElement {
  return (
    <li className="flex flex-col gap-2 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700">
      <Link
        href={`/projects/${defect.projectId}/defects/${defect.id}`}
        className="font-medium break-words underline underline-offset-4"
      >
        {defect.description}
      </Link>
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-zinc-600 dark:text-zinc-400">
        <span>{DEFECT_SEVERITY_LABELS[defect.severity]}</span>
        <span>{DEFECT_STATUS_LABELS[defect.status]}</span>
        {showStory && <span className="break-words">{defect.story.title}</span>}
        <span>Detectado en {defect.detectedSprint.name}</span>
        {defect.resolvedSprint && (
          <span>Resuelto en {defect.resolvedSprint.name}</span>
        )}
      </div>
    </li>
  );
}

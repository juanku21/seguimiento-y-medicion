import type { DefectSeverity, DefectStatus } from "@/types/defect";
import { Badge, type BadgeTone } from "@/components/badge";
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

// Tono de cada severidad y estado: el texto de la insignia los nombra.
const SEVERITY_TONES: Record<DefectSeverity, BadgeTone> = {
  critical: "danger",
  high: "warning",
  medium: "primary",
  low: "neutral",
};
const STATUS_TONES: Record<DefectStatus, BadgeTone> = {
  open: "warning",
  in_progress: "primary",
  resolved: "success",
  discarded: "neutral",
};

// Fila de un defecto en un listado: descripción con enlace al detalle, severidad y estado en
// español, historia y sprints de detección y, solo si lo tiene, de resolución (FR-043, FR-044).
export function DefectRow({
  defect,
  showStory = true,
}: DefectRowProps): ReactElement {
  return (
    <li className="flex flex-col gap-2 px-4 py-3 text-sm transition-colors hover:bg-surface-muted">
      <Link
        href={`/projects/${defect.projectId}/defects/${defect.id}`}
        className="rounded-sm font-medium break-words underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-focus"
      >
        {defect.description}
      </Link>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-foreground-muted">
        <Badge tone={SEVERITY_TONES[defect.severity]}>
          {DEFECT_SEVERITY_LABELS[defect.severity]}
        </Badge>{" "}
        <Badge tone={STATUS_TONES[defect.status]}>
          {DEFECT_STATUS_LABELS[defect.status]}
        </Badge>{" "}
        {showStory && <span className="break-words">{defect.story.title}</span>}
        <span>Detectado en {defect.detectedSprint.name}</span>
        {defect.resolvedSprint && (
          <span>Resuelto en {defect.resolvedSprint.name}</span>
        )}
      </div>
    </li>
  );
}

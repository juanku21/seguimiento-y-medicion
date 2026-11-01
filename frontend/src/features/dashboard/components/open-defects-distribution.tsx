import { LoadingState } from "@/components/loading-state";
import type { ReactNode } from "react";
import {
  DISTRIBUTION_ERROR,
  SectionError,
} from "@/features/dashboard/components/section-error";
import type { QueryState } from "@/hooks/use-authorized-query";
import type { ProjectMetricsResponse } from "@/types/metrics";
import { DEFECT_SEVERITY_LABELS } from "@/utils/defect-labels";

// Propiedades de la distribución: la consulta de métricas del proyecto ya iniciada por la vista.
interface OpenDefectsDistributionProps {
  projectMetrics: QueryState<ProjectMetricsResponse>;
}

// Severidades en el orden de presentación.
const SEVERITY_KEYS = ["critical", "high", "medium", "low"] as const;

// Fila término/valor de la distribución.
function Row({ term, value }: { term: string; value: number }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border py-2 last:border-b-0 last:font-semibold">
      <dt className="text-sm text-foreground-muted">{term}</dt>
      <dd className="text-lg font-semibold tabular-nums">{String(value)}</dd>
    </div>
  );
}

// Defectos abiertos por severidad (009/US5): muestra las cifras de la feature de métricas tal como
// llegan, sin sumar; el total es open.total.
export function OpenDefectsDistribution({
  projectMetrics,
}: OpenDefectsDistributionProps) {
  // Mientras carga no se monta la región: así solo existe con su contenido definitivo.
  if (projectMetrics.status === "loading") {
    return <LoadingState>Cargando…</LoadingState>;
  }
  let content: ReactNode;
  if (projectMetrics.status === "error") {
    content = <SectionError message={DISTRIBUTION_ERROR} />;
  } else {
    const { total, bySeverity } = projectMetrics.data.defects.open;
    content = (
      <>
        <dl className="flex flex-col">
          {SEVERITY_KEYS.map((key) => (
            <Row
              key={key}
              term={DEFECT_SEVERITY_LABELS[key]}
              value={bySeverity[key]}
            />
          ))}
          <Row term="Total" value={total} />
        </dl>
        {total === 0 && (
          <p className="text-sm text-foreground-muted">
            No hay defectos abiertos.
          </p>
        )}
      </>
    );
  }

  return (
    <section
      aria-labelledby="open-defects-heading"
      className="flex flex-col gap-3 rounded-lg border border-border bg-surface p-5"
    >
      <h3 id="open-defects-heading" className="text-lg font-semibold">
        Defectos abiertos por severidad
      </h3>
      {content}
    </section>
  );
}

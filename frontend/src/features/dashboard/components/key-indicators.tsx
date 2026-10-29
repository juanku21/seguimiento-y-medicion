import type { ReactNode } from "react";
import { FinalityBadge } from "@/features/dashboard/components/finality-badge";
import { MetricValue } from "@/features/dashboard/components/metric-value";
import {
  PROJECT_INDICATORS_ERROR,
  SPRINT_INDICATORS_ERROR,
  SectionError,
} from "@/features/dashboard/components/section-error";
import type { QueryState } from "@/hooks/use-authorized-query";
import type {
  ActiveSprintMetrics,
  ActiveSprintMetricsResponse,
  Cumulative,
  ProjectMetricsResponse,
} from "@/types/metrics";
import {
  formatHours,
  formatPercent,
  formatSignedHours,
} from "@/utils/effort-format";
import { formatDecimal, formatShare } from "@/utils/metric-format";
import type { MetricKind } from "@/utils/metric-explanations";

// Propiedades de los indicadores clave: las dos consultas ya iniciadas por la vista.
interface KeyIndicatorsProps {
  projectMetrics: QueryState<ProjectMetricsResponse>;
  activeSprintMetrics: QueryState<ActiveSprintMetricsResponse>;
}

// Siete métricas comunes al sprint activo y al acumulado, en el orden del contrato.
const METRIC_ROWS: {
  term: string;
  key: keyof ActiveSprintMetrics & keyof Cumulative;
  kind: MetricKind;
  format: (value: number) => string;
}[] = [
  {
    term: "Story Points planificados",
    key: "plannedStoryPoints",
    kind: "storyPoints",
    format: String,
  },
  {
    term: "Story Points completados",
    key: "completedStoryPoints",
    kind: "storyPoints",
    format: String,
  },
  {
    term: "Historias completadas",
    key: "storyCompletionPercent",
    kind: "storyCompletionPercent",
    format: formatShare,
  },
  {
    term: "Horas estimadas",
    key: "estimatedHours",
    kind: "hours",
    format: formatHours,
  },
  {
    term: "Horas reales",
    key: "actualHours",
    kind: "hours",
    format: formatHours,
  },
  {
    term: "Desviación",
    key: "deviationHours",
    kind: "deviationHours",
    format: formatSignedHours,
  },
  {
    term: "Desviación porcentual",
    key: "deviationPercent",
    kind: "deviationPercent",
    format: formatPercent,
  },
];

// Grupo de indicadores: un div con su h3 que lo nombra, sin rol de región.
function Group({
  id,
  title,
  badge,
  children,
}: {
  id: string;
  title: string;
  badge?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div aria-labelledby={id} className="flex flex-col gap-2">
      <h3 id={id} className="flex items-center gap-2 text-base font-medium">
        {title}
        {badge}
      </h3>
      {children}
    </div>
  );
}

// Fila término/valor de un indicador.
function MetricRow({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <dt className="text-sm text-zinc-600 dark:text-zinc-400">{term}</dt>
      <dd className="flex items-center gap-2 text-sm font-medium">
        {children}
      </dd>
    </div>
  );
}

// Filas de las siete métricas; el valor se muestra tal como llega.
function MetricRows({
  metrics,
}: {
  metrics: ActiveSprintMetrics | Cumulative;
}) {
  return (
    <>
      {METRIC_ROWS.map(({ term, key, kind, format }) => (
        <MetricRow key={term} term={term}>
          <MetricValue metric={metrics[key]} kind={kind} format={format} />
        </MetricRow>
      ))}
    </>
  );
}

// Texto de carga de un grupo.
function Loading({ text }: { text: string }) {
  return (
    <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
      {text}
    </p>
  );
}

// Grupo del sprint activo: carga, error, "sin sprint en curso" o sus siete indicadores.
function SprintGroup({
  state,
}: {
  state: QueryState<ActiveSprintMetricsResponse>;
}) {
  let content: ReactNode;
  if (state.status === "loading") {
    content = <Loading text="Cargando…" />;
  } else if (state.status === "error") {
    content = <SectionError message={SPRINT_INDICATORS_ERROR} />;
  } else if (!state.data.activeSprint || state.data.metrics === null) {
    content = (
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        No hay ningún sprint en curso.
      </p>
    );
  } else {
    content = (
      <dl className="flex flex-col gap-1">
        <MetricRows metrics={state.data.metrics} />
      </dl>
    );
  }
  return (
    <Group id="sprint-indicators-heading" title="Indicadores del sprint activo">
      {content}
    </Group>
  );
}

// Grupo del proyecto: velocidad, defectos abiertos y acumulado con su marca de finalidad.
function ProjectGroup({
  state,
}: {
  state: QueryState<ProjectMetricsResponse>;
}) {
  let content: ReactNode;
  let badge: ReactNode = null;
  if (state.status === "loading") {
    content = <Loading text="Cargando…" />;
  } else if (state.status === "error") {
    content = <SectionError message={PROJECT_INDICATORS_ERROR} />;
  } else {
    const { velocity, defects, cumulative } = state.data;
    badge = <FinalityBadge finality={cumulative.finality} />;
    content = (
      <dl className="flex flex-col gap-1">
        <MetricRow term="Velocidad del equipo">
          <MetricValue
            metric={velocity.value}
            kind="velocity"
            format={formatDecimal}
          />
        </MetricRow>
        <MetricRow term="Defectos abiertos">
          {String(defects.open.total)}
        </MetricRow>
        <MetricRows metrics={cumulative} />
      </dl>
    );
  }
  return (
    <Group
      id="project-indicators-heading"
      title="Indicadores del proyecto"
      badge={badge}
    >
      {content}
    </Group>
  );
}

// Indicadores clave del dashboard (009/US2): una región con dos grupos independientes, cada uno con
// su carga y su error. Muestra las cifras de la feature de métricas sin hacer aritmética (RN-01).
export function KeyIndicators({
  projectMetrics,
  activeSprintMetrics,
}: KeyIndicatorsProps) {
  return (
    <section
      aria-labelledby="key-indicators-heading"
      className="flex flex-col gap-4"
    >
      <h2 id="key-indicators-heading" className="text-lg font-semibold">
        Indicadores clave
      </h2>
      <SprintGroup state={activeSprintMetrics} />
      <ProjectGroup state={projectMetrics} />
    </section>
  );
}

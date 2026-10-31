// Modelo de presentación del reporte que consumen la pantalla y el PDF.
import type {
  Finality,
  Metric,
  ProjectMetricsResponse,
  SprintMetrics,
} from "@/types/metrics";
import type {
  EstimationLine,
  Report,
  ReportSprint,
  StoryResult,
} from "@/types/report";
import {
  DEFECT_SEVERITY_LABELS,
  DEFECT_STATUS_LABELS,
} from "@/utils/defect-labels";
import { formatDate } from "@/utils/format-date";
import {
  explainNotCalculable,
  type MetricKind,
} from "@/utils/metric-explanations";
import { formatDecimal, formatShare } from "@/utils/metric-format";
import { reportFileName } from "@/utils/report-file-name";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
  UNESTIMATED_LABEL,
} from "@/utils/story-labels";

// Tabla ya formateada; footer es null si la tabla no tiene pie.
export type TableView = {
  columns: string[];
  rows: string[][];
  footer: string[] | null;
};

// Bloque de una sección: subtítulo de grupo, mensaje y tabla.
export type BlockView = {
  heading: string | null;
  message: string | null;
  table: TableView | null;
};

// Sección del reporte, siempre en el orden fijo de seis.
export type SectionView = {
  key: "header" | "stories" | "estimations" | "effort" | "metrics" | "defects";
  title: string;
  state: "complete" | "empty" | "unavailable";
  message: string | null;
  blocks: BlockView[];
};

// Reporte listo para mostrar.
export type ReportView = {
  title: string;
  partialNote: string | null;
  header: { label: string; value: string }[];
  sections: SectionView[];
  fileName: string;
};

// Texto de las secciones y grupos sin datos.
const EMPTY_MESSAGE = "Sin datos.";

// Texto de las secciones que no se pudieron obtener (RN-12).
const UNAVAILABLE_MESSAGE = "No se pudo obtener esta sección.";

// Marca de finalidad que se agrega al valor de una métrica (RN-14).
const FINALITY_LABELS: Record<Finality, string> = {
  partial: "Parcial",
  provisional: "Provisoria",
  definitive: "Definitiva",
};

// Estado de una historia en el reporte: reutiliza las etiquetas de la 003 y agrega la propia.
function storyResultLabel(status: StoryResult): string {
  return status === "not_completed"
    ? "No completada"
    : STORY_STATUS_LABELS[status];
}

// Texto de una métrica según su estado y su finalidad; lo no calculable nunca se muestra como 0 (RN-13).
export function metricText(
  metric: Metric & { finality?: Finality | null },
  kind: MetricKind,
  format: (value: number) => string,
): string {
  let text = "No aplica";
  if (metric.status === "available" && metric.value !== null) {
    text = format(metric.value);
  } else if (metric.status === "not_calculable") {
    text = `No calculable: ${explainNotCalculable(kind)}`;
  }
  return metric.finality
    ? `${text} · ${FINALITY_LABELS[metric.finality]}`
    : text;
}

// Instante ISO como "DD/MM/AAAA HH:mm:ss" en la zona horaria indicada.
function formatDateTime(isoInstant: string, timeZone: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(isoInstant));
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((item) => item.type === type)?.value ?? "";
  return `${part("day")}/${part("month")}/${part("year")} ${part("hour")}:${part("minute")}:${part("second")}`;
}

// Subtítulo de un grupo: nombre del sprint con su período y " — en curso" si está activo.
function groupHeading(sprint: ReportSprint | null): string {
  if (sprint === null) {
    return "Historias no planificadas";
  }
  const period = `${formatDate(sprint.startDate)} al ${formatDate(sprint.endDate)}`;
  const running = sprint.status === "active" ? " — en curso" : "";
  return `${sprint.name} (${period})${running}`;
}

// Texto de la columna Planning Poker de una línea de estimación.
function pokerText(line: EstimationLine): string {
  if (line.poker === "none") {
    return "No hubo sesión de estimación";
  }
  if (line.poker === "agreed" && line.session) {
    const rounds = line.session.rounds;
    const noun = rounds === 1 ? "ronda" : "rondas";
    return `${rounds} ${noun}, valor acordado ${line.session.agreedStoryPoints}`;
  }
  return "Sin valor acordado por estimación colaborativa";
}

// Bloque de grupo: tabla con datos, o "Sin datos." si el grupo está vacío.
function groupBlock(
  sprint: ReportSprint | null,
  empty: boolean,
  table: TableView,
): BlockView {
  return {
    heading: groupHeading(sprint),
    message: empty ? EMPTY_MESSAGE : null,
    table: empty ? null : table,
  };
}

// Bloque que solo lleva un texto, como un total.
function textBlock(message: string): BlockView {
  return { heading: null, message, table: null };
}

// Sección que, sin datos o no disponible, muestra su mensaje en lugar de sus bloques.
function section(
  key: SectionView["key"],
  title: string,
  state: SectionView["state"],
  blocks: BlockView[],
): SectionView {
  const messages = {
    complete: null,
    empty: EMPTY_MESSAGE,
    unavailable: UNAVAILABLE_MESSAGE,
  };
  return {
    key,
    title,
    state,
    message: messages[state],
    blocks: state === "complete" ? blocks : [],
  };
}

// Métrica con su etiqueta, su tipo y su formato; la finalidad solo viene en las de sprint.
type MetricSpec = [
  label: string,
  metric: Metric & { finality?: Finality | null },
  kind: MetricKind,
  format: (value: number) => string,
];

// Las siete métricas que comparten el sprint y el acumulado del proyecto, con los nombres del dashboard.
function sharedMetricSpecs(
  source: Record<
    | "plannedStoryPoints"
    | "completedStoryPoints"
    | "storyCompletionPercent"
    | "estimatedHours"
    | "actualHours"
    | "deviationHours"
    | "deviationPercent",
    Metric & { finality?: Finality | null }
  >,
): MetricSpec[] {
  return [
    [
      "Story Points planificados",
      source.plannedStoryPoints,
      "storyPoints",
      String,
    ],
    [
      "Story Points completados",
      source.completedStoryPoints,
      "storyPoints",
      String,
    ],
    [
      "Historias completadas",
      source.storyCompletionPercent,
      "storyCompletionPercent",
      formatShare,
    ],
    ["Horas estimadas", source.estimatedHours, "hours", formatDecimal],
    ["Horas reales", source.actualHours, "hours", formatDecimal],
    ["Desviación", source.deviationHours, "deviationHours", formatDecimal],
    [
      "Desviación porcentual",
      source.deviationPercent,
      "deviationPercent",
      formatShare,
    ],
  ];
}

// Filas [etiqueta, texto]; con `finality` se la impone a todas, si no cada métrica usa la suya.
function metricSpecRows(specs: MetricSpec[], finality?: Finality): string[][] {
  return specs.map(([label, metric, kind, format]) => [
    label,
    metricText(
      finality === undefined ? metric : { ...metric, finality },
      kind,
      format,
    ),
  ]);
}

// Filas de la tabla de métricas de un sprint: las siete compartidas y los dos conteos de defectos.
function sprintMetricRows(metrics: SprintMetrics): string[][] {
  return metricSpecRows([
    ...sharedMetricSpecs(metrics),
    ["Defectos detectados", metrics.defectsDetected, "defects", String],
    ["Defectos resueltos", metrics.defectsResolved, "defects", String],
  ]);
}

// Filas de la tabla de métricas del proyecto: el acumulado con su finalidad y las tres cifras propias.
function projectMetricRows(project: ProjectMetricsResponse): string[][] {
  const { cumulative } = project;
  return [
    ...metricSpecRows(sharedMetricSpecs(cumulative), cumulative.finality),
    [
      "Esfuerzo fuera de sprints",
      formatDecimal(project.effortOutsideSprintsHours),
    ],
    [
      "Esfuerzo en sprints planificados",
      formatDecimal(project.effortInPlannedSprintsHours),
    ],
    [
      "Historias sin estimar en el backlog",
      String(project.unestimatedBacklogStories),
    ],
  ];
}

// Arma el modelo de presentación del reporte de un sprint o del proyecto, con todas las cadenas ya formateadas.
export function buildReportView(report: Report, timeZone: string): ReportView {
  const { header } = report;
  const sprintName = header.sprint?.name ?? "";
  const isProject = report.scope === "project";
  const period = header.period
    ? `${formatDate(header.period.startDate)} al ${formatDate(header.period.endDate)}`
    : "Sin sprints reportables";

  // Historias: una tabla por grupo.
  const stories = (report.stories.groups ?? []).map((group) =>
    groupBlock(group.sprint, group.state === "empty", {
      columns: ["Historia", "Prioridad", "Story Points", "Estado"],
      rows: group.stories.map((story) => [
        story.title,
        STORY_PRIORITY_LABELS[story.priority],
        story.storyPoints === null
          ? UNESTIMATED_LABEL
          : String(story.storyPoints),
        storyResultLabel(story.status),
      ]),
      footer: null,
    }),
  );

  // Estimaciones: una tabla por grupo con el resultado del poker.
  const estimations = (report.estimations.groups ?? []).map((group) =>
    groupBlock(group.sprint, group.state === "empty", {
      columns: ["Historia", "Story Points", "Planning Poker"],
      rows: group.items.map((line) => [
        line.title,
        line.storyPoints === null
          ? UNESTIMATED_LABEL
          : String(line.storyPoints),
        pokerText(line),
      ]),
      footer: null,
    }),
  );

  // Esfuerzo: tabla por historia con su total, tabla por integrante y total registrado.
  const effortGroups = (report.effort.groups ?? []).map((group) =>
    groupBlock(group.sprint, group.state === "empty", {
      columns: ["Historia", "Horas"],
      rows: group.stories.map((story) => [
        story.title,
        formatDecimal(story.hours),
      ]),
      footer: ["Total del sprint", formatDecimal(group.totalHours)],
    }),
  );
  const byMember: BlockView = {
    heading: "Por integrante",
    message: null,
    table: {
      columns: ["Integrante", "Horas"],
      rows: (report.effort.byMember ?? []).map((member) => [
        member.fullName,
        formatDecimal(member.hours),
      ]),
      footer: null,
    },
  };
  const effort = [
    ...effortGroups,
    byMember,
    textBlock(
      `Total registrado: ${formatDecimal(report.effort.totalHours ?? 0)}`,
    ),
  ];

  // Métricas: una sola tabla con una fila por métrica, del sprint o del proyecto.
  const { sprint: sprintMetrics, project: projectMetrics } = report.metrics;
  const metricRows = sprintMetrics
    ? sprintMetricRows(sprintMetrics)
    : projectMetrics
      ? projectMetricRows(projectMetrics)
      : null;
  const metrics: BlockView[] = metricRows
    ? [
        {
          heading: null,
          message: null,
          table: {
            columns: ["Métrica", "Valor"],
            rows: metricRows,
            footer: null,
          },
        },
      ]
    : [];

  // Defectos: la tabla y los totales, que se muestran aunque no haya defectos (US4-2).
  const defectLines = report.defects.defects ?? [];
  const totals = report.defects.totals ?? { detected: 0, resolved: 0 };
  const defects: BlockView[] = [
    {
      heading: null,
      message: null,
      table: {
        columns: [
          "Descripción",
          "Severidad",
          "Estado",
          "Historia",
          "Detectado en",
          "Resuelto en",
        ],
        rows: defectLines.map((defect) => [
          defect.description,
          DEFECT_SEVERITY_LABELS[defect.severity],
          DEFECT_STATUS_LABELS[defect.status],
          defect.story.title,
          defect.detectedSprint.name,
          defect.resolvedSprint?.name ?? "—",
        ]),
        footer: null,
      },
    },
    textBlock(`Detectados: ${totals.detected}`),
    textBlock(`Resueltos: ${totals.resolved}`),
  ];

  return {
    title: isProject
      ? `Reporte del proyecto ${header.projectName}`
      : `Reporte del sprint ${sprintName}`,
    partialNote: report.partial
      ? "Reporte parcial: incluye un sprint en curso."
      : null,
    header: [
      { label: "Proyecto", value: header.projectName },
      {
        label: "Alcance",
        value: isProject ? "Proyecto completo" : `Sprint ${sprintName}`,
      },
      { label: "Período", value: period },
      {
        label: "Generado el",
        value: formatDateTime(header.generatedAt, timeZone),
      },
      { label: "Generado por", value: header.generatedBy.fullName },
    ],
    sections: [
      section("header", "Encabezado", "complete", []),
      section("stories", "Historias", report.stories.state, stories),
      section(
        "estimations",
        "Estimaciones",
        report.estimations.state,
        estimations,
      ),
      section("effort", "Esfuerzo", report.effort.state, effort),
      section("metrics", "Métricas", report.metrics.state, metrics),
      // Con defectos vacíos se conservan los totales en cero.
      {
        ...section("defects", "Defectos", report.defects.state, defects),
        blocks: report.defects.state === "unavailable" ? [] : defects,
      },
    ],
    // Nombre del PDF con proyecto, alcance y fecha de generación en la zona del navegador.
    fileName: reportFileName({
      projectName: header.projectName,
      scope: report.scope,
      sprintName: header.sprint?.name ?? null,
      generatedAt: header.generatedAt,
      timeZone,
    }),
  };
}

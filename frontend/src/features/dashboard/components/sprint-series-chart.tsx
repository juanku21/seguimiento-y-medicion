"use client";

import type { ReactNode } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { SeriesPoint } from "@/types/metrics";
import type { MetricKind } from "@/utils/metric-explanations";
import { MetricValue } from "./metric-value";
import { SectionError } from "./section-error";

// Una serie del gráfico: su nombre en la leyenda y sus puntos por sprint.
export interface SeriesInput {
  name: string;
  points: SeriesPoint[];
}

// Propiedades del gráfico común de barras agrupadas por sprint.
export interface SprintSeriesChartProps {
  title: string;
  yAxisLabel: string;
  kind: MetricKind;
  format: (value: number) => string;
  emptyMessage: string;
  errorMessage: string;
  series: [SeriesInput, SeriesInput];
}

// Colores de las dos series: tokens con contraste de 3:1 entre sí y contra el fondo en ambos temas.
const SERIES_COLORS = ["var(--chart-1)", "var(--chart-2)"];

// Indica si ambas series tienen los mismos sprints en el mismo orden.
function haveSameSprints(first: SeriesPoint[], second: SeriesPoint[]): boolean {
  return (
    first.length === second.length &&
    first.every((point, index) => point.sprintId === second[index].sprintId)
  );
}

// Gráfico de barras agrupadas con dos series por sprint y su tabla de datos equivalente.
export function SprintSeriesChart({
  title,
  yAxisLabel,
  kind,
  format,
  emptyMessage,
  errorMessage,
  series,
}: SprintSeriesChartProps): ReactNode {
  const [first, second] = series;

  // Series con sprints distintos o en otro orden: no se dibuja nada engañoso.
  if (!haveSameSprints(first.points, second.points)) {
    return <SectionError message={errorMessage} />;
  }
  // Sin puntos no hay ejes ni tabla, solo el mensaje de vacío.
  if (first.points.length === 0) {
    return <p className="text-sm text-foreground-muted">{emptyMessage}</p>;
  }

  // Filas del gráfico armadas por posición; el sprint Activo se marca como parcial.
  const rows = first.points.map((point, index) => ({
    label:
      point.sprintStatus === "active"
        ? `${point.sprintName} (parcial)`
        : point.sprintName,
    first: point.value,
    second: second.points[index].value,
  }));

  return (
    <figure
      aria-label={title}
      className="min-w-0 space-y-3 rounded-lg border border-border bg-surface p-5"
    >
      <figcaption className="text-lg font-semibold">{title}</figcaption>
      <ResponsiveContainer
        width="100%"
        height={320}
        initialDimension={{ width: 800, height: 320 }}
      >
        <BarChart data={rows} margin={{ top: 8, right: 8, bottom: 8 }}>
          <CartesianGrid vertical={false} stroke="var(--border)" />
          <XAxis
            dataKey="label"
            interval={0}
            angle={-20}
            textAnchor="end"
            height={72}
            stroke="var(--border-strong)"
            tick={{ fill: "var(--foreground-muted)", fontSize: 12 }}
            label={{
              value: "Sprint",
              position: "insideBottom",
              offset: 0,
              fill: "var(--foreground-muted)",
              fontSize: 12,
            }}
          />
          <YAxis
            stroke="var(--border-strong)"
            tick={{ fill: "var(--foreground-muted)", fontSize: 12 }}
            label={{
              value: yAxisLabel,
              angle: -90,
              position: "insideLeft",
              fill: "var(--foreground-muted)",
              fontSize: 12,
            }}
          />
          <Legend
            verticalAlign="bottom"
            content={({ payload }) => (
              <ul className="flex flex-wrap justify-center gap-x-4 gap-y-1 pt-3 text-sm text-foreground-muted">
                {(payload ?? []).map((entry) => (
                  <li key={String(entry.value)} className="flex items-center">
                    <span
                      aria-hidden="true"
                      className="mr-1.5 inline-block size-3 rounded-sm"
                      style={{ backgroundColor: entry.color }}
                    />
                    {entry.value}
                  </li>
                ))}
              </ul>
            )}
          />
          <Tooltip
            cursor={{ fill: "var(--surface-muted)" }}
            contentStyle={{
              backgroundColor: "var(--surface)",
              borderColor: "var(--border)",
              borderRadius: 8,
              color: "var(--foreground)",
            }}
            labelStyle={{ color: "var(--foreground)" }}
            itemStyle={{ color: "var(--foreground)" }}
            formatter={(value) =>
              typeof value === "number" ? format(value) : String(value)
            }
          />
          <Bar
            dataKey="first"
            name={first.name}
            fill={SERIES_COLORS[0]}
            radius={[3, 3, 0, 0]}
          />
          <Bar
            dataKey="second"
            name={second.name}
            fill={SERIES_COLORS[1]}
            radius={[3, 3, 0, 0]}
          />
        </BarChart>
      </ResponsiveContainer>
      {first.points.length === 1 && (
        <p className="text-sm text-foreground-muted">
          Todavía no hay evolución que comparar.
        </p>
      )}
      <details>
        <summary className="min-h-6 cursor-pointer text-sm font-medium text-primary">
          Ver datos
        </summary>
        <table
          aria-label={`Datos de ${title}`}
          className="mt-3 w-full text-sm tabular-nums [&_td]:border-t [&_td]:border-border [&_td]:px-2 [&_td]:py-1.5 [&_td]:text-right [&_th]:px-2 [&_th]:py-1.5 [&_th]:text-left [&_th]:font-medium [&_thead_th:not(:first-child)]:text-right [&_tbody_th]:border-t [&_tbody_th]:border-border"
        >
          <thead>
            <tr>
              <th scope="col">Sprint</th>
              <th scope="col">{first.name}</th>
              <th scope="col">{second.name}</th>
            </tr>
          </thead>
          <tbody>
            {first.points.map((point, index) => (
              <tr key={point.sprintId}>
                <th scope="row">{point.sprintName}</th>
                <td>
                  <MetricValue metric={point} kind={kind} format={format} />
                </td>
                <td>
                  <MetricValue
                    metric={second.points[index]}
                    kind={kind}
                    format={format}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </details>
    </figure>
  );
}

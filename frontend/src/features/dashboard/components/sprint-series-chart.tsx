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

// Colores de las dos series, distinguibles también por la leyenda.
const SERIES_COLORS = ["#2563eb", "#f59e0b"];

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
    return <p className="text-sm text-gray-600">{emptyMessage}</p>;
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
    <figure aria-label={title} className="space-y-3">
      <figcaption className="text-lg font-semibold">{title}</figcaption>
      <ResponsiveContainer
        width="100%"
        height={320}
        initialDimension={{ width: 800, height: 320 }}
      >
        <BarChart data={rows} margin={{ bottom: 40 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis
            dataKey="label"
            interval={0}
            angle={-35}
            textAnchor="end"
            height={70}
            label={{ value: "Sprint", position: "insideBottom", offset: -5 }}
          />
          <YAxis
            label={{ value: yAxisLabel, angle: -90, position: "insideLeft" }}
          />
          <Legend
            verticalAlign="bottom"
            content={({ payload }) => (
              <ul className="flex justify-center gap-4 text-sm">
                {(payload ?? []).map((entry) => (
                  <li key={String(entry.value)} className="flex items-center">
                    <span
                      aria-hidden="true"
                      className="mr-1 inline-block h-3 w-3"
                      style={{ backgroundColor: entry.color }}
                    />
                    {entry.value}
                  </li>
                ))}
              </ul>
            )}
          />
          <Tooltip
            formatter={(value) =>
              typeof value === "number" ? format(value) : String(value)
            }
          />
          <Bar dataKey="first" name={first.name} fill={SERIES_COLORS[0]} />
          <Bar dataKey="second" name={second.name} fill={SERIES_COLORS[1]} />
        </BarChart>
      </ResponsiveContainer>
      {first.points.length === 1 && (
        <p className="text-sm text-gray-600">
          Todavía no hay evolución que comparar.
        </p>
      )}
      <details>
        <summary className="cursor-pointer text-sm">Ver datos</summary>
        <table aria-label={`Datos de ${title}`} className="mt-2 text-sm">
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

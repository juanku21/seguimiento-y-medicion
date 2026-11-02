import type { ReactNode } from "react";
import { readSeries } from "@/features/dashboard/api";
import { EFFORT_ERROR } from "@/features/dashboard/components/section-error";
import { SeriesSection } from "@/features/dashboard/components/series-section";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import type { QueryState } from "@/hooks/use-authorized-query";
import type { ProjectSeriesResponse } from "@/types/metrics";
import { formatHours } from "@/utils/effort-format";

// Propiedades del gráfico de esfuerzo: la consulta de series ya iniciada por la vista.
interface EffortChartProps {
  series: QueryState<ProjectSeriesResponse>;
}

// Gráfico de esfuerzo por sprint (009/US4): compara las horas estimadas y reales. Incluye el sprint
// activo (se identifica como parcial), por eso no filtra los puntos a los sprints cerrados.
export function EffortChart({ series }: EffortChartProps): ReactNode {
  return (
    <SeriesSection
      series={series}
      errorMessage={EFFORT_ERROR}
      render={(data) => {
        // Lee cada serie con su reconocedor; null indica una porción malformada.
        const estimated = readSeries(data, "estimatedHours");
        const actual = readSeries(data, "actualHours");
        if (estimated === null || actual === null) {
          return null;
        }
        return (
          <SprintSeriesChart
            title="Esfuerzo por sprint"
            yAxisLabel="Horas"
            kind="hours"
            format={formatHours}
            emptyMessage="Todavía no hay sprints iniciados para mostrar esfuerzo."
            errorMessage={EFFORT_ERROR}
            series={[
              { name: "Horas estimadas", points: estimated },
              { name: "Horas reales", points: actual },
            ]}
          />
        );
      }}
    />
  );
}

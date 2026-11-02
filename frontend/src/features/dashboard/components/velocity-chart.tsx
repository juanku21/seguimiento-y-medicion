import type { ReactNode } from "react";
import { readSeries } from "@/features/dashboard/api";
import { VELOCITY_ERROR } from "@/features/dashboard/components/section-error";
import { SeriesSection } from "@/features/dashboard/components/series-section";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import { closedSprintPoints } from "@/features/dashboard/series";
import type { QueryState } from "@/hooks/use-authorized-query";
import type { ProjectSeriesResponse } from "@/types/metrics";

// Propiedades del gráfico de velocidad: la consulta de series ya iniciada por la vista.
interface VelocityChartProps {
  series: QueryState<ProjectSeriesResponse>;
}

// Gráfico de velocidad por sprint (009/US3): compara los Story Points planificados y completados de
// los sprints cerrados. Si la consulta falló o una de las series es inválida, muestra solo el error.
export function VelocityChart({ series }: VelocityChartProps): ReactNode {
  return (
    <SeriesSection
      series={series}
      errorMessage={VELOCITY_ERROR}
      render={(data) => {
        // Lee cada serie con su reconocedor; null indica una porción malformada.
        const planned = readSeries(data, "plannedStoryPoints");
        const completed = readSeries(data, "completedStoryPoints");
        if (planned === null || completed === null) {
          return null;
        }
        return (
          <SprintSeriesChart
            title="Velocidad por sprint"
            yAxisLabel="Story Points"
            kind="storyPoints"
            format={String}
            emptyMessage="Todavía no hay sprints cerrados para mostrar la velocidad."
            errorMessage={VELOCITY_ERROR}
            series={[
              {
                name: "Story Points planificados",
                points: closedSprintPoints(planned),
              },
              {
                name: "Story Points completados",
                points: closedSprintPoints(completed),
              },
            ]}
          />
        );
      }}
    />
  );
}

import type { ReactNode } from "react";
import { readSeries } from "@/features/dashboard/api";
import { QUALITY_ERROR } from "@/features/dashboard/components/section-error";
import { SeriesSection } from "@/features/dashboard/components/series-section";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import type { QueryState } from "@/hooks/use-authorized-query";
import type { ProjectSeriesResponse } from "@/types/metrics";

// Propiedades del gráfico de calidad: la consulta de series ya iniciada por la vista.
interface QualityChartProps {
  series: QueryState<ProjectSeriesResponse>;
}

// Gráfico de defectos por sprint (009/US5): compara los defectos detectados y resueltos. Incluye el
// sprint activo, por eso no filtra los puntos a los sprints cerrados.
export function QualityChart({ series }: QualityChartProps): ReactNode {
  return (
    <SeriesSection
      series={series}
      errorMessage={QUALITY_ERROR}
      render={(data) => {
        // Lee cada serie con su reconocedor; null indica una porción malformada.
        const detected = readSeries(data, "defectsDetected");
        const resolved = readSeries(data, "defectsResolved");
        if (detected === null || resolved === null) {
          return null;
        }
        return (
          <SprintSeriesChart
            title="Defectos por sprint"
            yAxisLabel="Defectos"
            kind="defects"
            format={String}
            emptyMessage="Todavía no hay sprints iniciados para mostrar defectos."
            errorMessage={QUALITY_ERROR}
            series={[
              { name: "Defectos detectados", points: detected },
              { name: "Defectos resueltos", points: resolved },
            ]}
          />
        );
      }}
    />
  );
}

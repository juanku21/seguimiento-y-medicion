import { LoadingState } from "@/components/loading-state";
import type { ReactNode } from "react";
import { SectionError } from "@/features/dashboard/components/section-error";
import type { QueryState } from "@/hooks/use-authorized-query";
import type { ProjectSeriesResponse } from "@/types/metrics";

// Propiedades del armazón de un gráfico de series: la consulta, su mensaje de falla y cómo dibujar.
interface SeriesSectionProps {
  series: QueryState<ProjectSeriesResponse>;
  errorMessage: string;
  // Dibuja el gráfico con las series recibidas; devuelve null si alguna está malformada.
  render: (data: ProjectSeriesResponse) => ReactNode | null;
}

// Armazón común de los gráficos por sprint: resuelve la carga, el error de la consulta y la serie
// malformada, y delega el gráfico en render cuando los datos son válidos.
export function SeriesSection({
  series,
  errorMessage,
  render,
}: SeriesSectionProps): ReactNode {
  if (series.status === "loading") {
    return <LoadingState>Cargando…</LoadingState>;
  }
  if (series.status === "error") {
    return <SectionError message={errorMessage} />;
  }
  return render(series.data) ?? <SectionError message={errorMessage} />;
}

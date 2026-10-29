import type { ReactNode } from "react";
import type { MetricsWarning } from "@/types/metrics";

// Muestra las advertencias de las métricas separadas por kind, con el message de la 008 sin cambios.
export function DashboardWarnings({
  warnings,
}: {
  warnings: MetricsWarning[];
}): ReactNode {
  // Sin advertencias la sección no se muestra.
  if (warnings.length === 0) {
    return null;
  }

  const planning = warnings.filter((w) => w.kind === "planning");
  const inconsistencies = warnings.filter(
    (w) => w.kind === "data_inconsistency",
  );

  return (
    <section aria-label="Advertencias" className="space-y-3">
      {planning.length > 0 && (
        // Avisos informativos de planificación.
        <div
          role="status"
          aria-label="Avisos de planificación"
          className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900"
        >
          <ul className="list-disc space-y-1 pl-5">
            {planning.map((w) => (
              <li key={`${w.code}-${w.message}`}>{w.message}</li>
            ))}
          </ul>
        </div>
      )}
      {inconsistencies.length > 0 && (
        // Inconsistencias de datos: alerta con texto que pide revisión.
        <div
          role="alert"
          aria-label="Datos que hay que revisar"
          className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900"
        >
          <p className="mb-2 font-medium">
            Hay información rota que alguien debe revisar.
          </p>
          <ul className="list-disc space-y-1 pl-5">
            {inconsistencies.map((w) => (
              <li key={`${w.code}-${w.message}`}>{w.message}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

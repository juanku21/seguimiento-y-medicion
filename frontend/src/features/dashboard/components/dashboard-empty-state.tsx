import type { ReactNode } from "react";

// Estado vacío del dashboard (009/US6): reemplaza las secciones de métricas cuando el proyecto no
// tiene sprints. No ofrece enlaces porque la navegación hacia los módulos está fuera de alcance.
export function DashboardEmptyState(): ReactNode {
  return (
    <section aria-labelledby="dashboard-empty-title" className="space-y-2">
      <h2 id="dashboard-empty-title" className="text-lg font-semibold">
        Sin datos todavía
      </h2>
      <p className="text-sm text-gray-600">
        Este proyecto todavía no tiene sprints, así que no hay métricas que
        mostrar. Planificá el primer sprint desde la sección Sprints del
        proyecto.
      </p>
    </section>
  );
}

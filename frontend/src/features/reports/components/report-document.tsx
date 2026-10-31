"use client";

import { ReportSection } from "@/features/reports/components/report-section";
import type { ReportView } from "@/features/reports/view-model";

// Documento del reporte en pantalla: solo dispone el modelo de presentación, sin formatear nada.
export function ReportDocument({ view }: { view: ReportView }) {
  return (
    <article
      aria-label="Reporte"
      className="flex flex-col gap-6 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700"
    >
      <h2 className="text-xl font-semibold break-words">{view.title}</h2>
      {view.partialNote !== null && (
        <p role="status" className="text-sm font-medium">
          {view.partialNote}
        </p>
      )}
      {view.sections.map((section) => (
        <ReportSection key={section.key} section={section}>
          {section.key === "header" && (
            // El encabezado muestra sus datos como lista de términos.
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
              {view.header.map((item) => (
                <div key={item.label} className="contents">
                  <dt className="font-medium">{item.label}</dt>
                  <dd className="break-words">{item.value}</dd>
                </div>
              ))}
            </dl>
          )}
        </ReportSection>
      ))}
    </article>
  );
}

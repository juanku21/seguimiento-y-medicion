"use client";

import { useId, type ReactNode } from "react";
import type { BlockView, SectionView } from "@/features/reports/view-model";

// Propiedades de una sección: su modelo de presentación y, opcionalmente, contenido propio
// (el encabezado lo usa para su lista de datos).
interface ReportSectionProps {
  section: SectionView;
  children?: ReactNode;
}

// Un bloque: subtítulo del grupo, mensaje y tabla, todo ya formateado por el modelo de presentación.
function ReportBlock({ block }: { block: BlockView }) {
  const { table } = block;
  return (
    <div className="flex flex-col gap-2">
      {block.heading !== null && (
        <h4 className="text-sm font-semibold break-words">{block.heading}</h4>
      )}
      {block.message !== null &&
        (block.heading !== null ? (
          // Un grupo sin datos lleva su aviso como nota.
          <p role="note" className="text-sm text-zinc-600 dark:text-zinc-400">
            {block.message}
          </p>
        ) : (
          <p className="text-sm">{block.message}</p>
        ))}
      {table !== null && (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead>
              <tr>
                {table.columns.map((column) => (
                  <th
                    key={column}
                    scope="col"
                    className="border-b border-zinc-300 px-3 py-2 font-medium dark:border-zinc-700"
                  >
                    {column}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {table.rows.map((row, rowIndex) => (
                <tr key={rowIndex}>
                  {row.map((cell, cellIndex) => (
                    <td
                      key={cellIndex}
                      className="border-b border-zinc-200 px-3 py-2 break-words dark:border-zinc-800"
                    >
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
            {table.footer !== null && (
              <tfoot>
                <tr>
                  {table.footer.map((cell, cellIndex) => (
                    <td key={cellIndex} className="px-3 py-2 font-medium">
                      {cell}
                    </td>
                  ))}
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      )}
    </div>
  );
}

// Sección del reporte: una región con su título y, según el estado, la nota "Sin datos.", la alerta de no disponible o sus bloques.
export function ReportSection({ section, children }: ReportSectionProps) {
  const titleId = useId();
  return (
    <section
      aria-labelledby={titleId}
      className="flex flex-col gap-4 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
    >
      <h3 id={titleId} className="text-lg font-semibold">
        {section.title}
      </h3>
      {section.message !== null &&
        (section.state === "unavailable" ? (
          // Una sección que falló se anuncia como alerta.
          <p role="alert" className="text-sm text-red-700 dark:text-red-400">
            {section.message}
          </p>
        ) : (
          <p role="note" className="text-sm text-zinc-600 dark:text-zinc-400">
            {section.message}
          </p>
        ))}
      {children}
      {section.blocks.map((block, index) => (
        <ReportBlock key={index} block={block} />
      ))}
    </section>
  );
}

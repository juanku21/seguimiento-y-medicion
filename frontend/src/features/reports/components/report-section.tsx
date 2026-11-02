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
          <p role="note" className="text-sm text-foreground-muted">
            {block.message}
          </p>
        ) : (
          <p className="text-sm">{block.message}</p>
        ))}
      {table !== null && (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm tabular-nums">
            <thead>
              <tr>
                {table.columns.map((column) => (
                  <th
                    key={column}
                    scope="col"
                    className="border-b border-border px-3 py-2 font-medium text-foreground-muted"
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
                      className="border-b border-border px-3 py-2 break-words"
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

// Sección del reporte: una región con su título, el aviso de su estado y sus bloques.
export function ReportSection({ section, children }: ReportSectionProps) {
  const titleId = useId();
  const unavailable = section.state === "unavailable";
  return (
    <section
      aria-labelledby={titleId}
      className="flex flex-col gap-4 rounded-lg border border-border bg-surface px-5 py-4"
    >
      <h3 id={titleId} className="text-lg font-semibold">
        {section.title}
      </h3>
      {section.message !== null && (
        // Una sección que falló se anuncia como alerta; sin datos, como nota.
        <p
          role={unavailable ? "alert" : "note"}
          className={
            unavailable
              ? "text-sm text-danger"
              : "text-sm text-foreground-muted"
          }
        >
          {section.message}
        </p>
      )}
      {children}
      {section.blocks.map((block, index) => (
        <ReportBlock key={index} block={block} />
      ))}
    </section>
  );
}

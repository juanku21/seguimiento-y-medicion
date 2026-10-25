"use client";

import { useId, useState } from "react";
import { Alert } from "@/components/alert";
import { EffortEntryForm } from "@/features/effort/components/effort-entry-form";
import type { EffortEntry } from "@/types/effort";
import { formatHours } from "@/utils/effort-format";
import { formatDate } from "@/utils/format-date";

// Propiedades de la sección "Esfuerzo" del detalle de una historia: la historia tomada de la ruta.
export type StoryEffortSectionProps = {
  storyId: string;
};

// Sección "Esfuerzo" del detalle de una historia (006/US1): ofrece el formulario de registro y
// confirma el último registro dado de alta con sus horas y su fecha. El total, el desglose por
// integrante y la lista de registros se suman en la 006/US2.
export function StoryEffortSection({ storyId }: StoryEffortSectionProps) {
  const headingId = useId();
  const [lastEntry, setLastEntry] = useState<EffortEntry | null>(null);

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <h2 id={headingId} className="text-lg font-semibold tracking-tight">
        Esfuerzo
      </h2>
      {lastEntry && (
        <Alert variant="success">
          Se registraron {formatHours(lastEntry.hours)} el{" "}
          {formatDate(lastEntry.entryDate)}.
        </Alert>
      )}
      <EffortEntryForm storyId={storyId} onCreated={setLastEntry} />
    </section>
  );
}

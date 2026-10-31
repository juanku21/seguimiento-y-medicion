"use client";

import { useCallback, useMemo, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import {
  generateProjectReport,
  generateSprintReport,
} from "@/features/reports/api";
import { buildPdfDocument } from "@/features/reports/pdf/pdf-document";
import { exportReportPdf } from "@/features/reports/pdf/export-pdf";
import { ReportDocument } from "@/features/reports/components/report-document";
import {
  PROJECT_SCOPE,
  ReportScopeSelect,
} from "@/features/reports/components/report-scope-select";
import {
  buildReportView,
  type ReportView,
} from "@/features/reports/view-model";
import { listSprints } from "@/features/sprints/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";

// Propiedades de la vista: el proyecto cuyos reportes se muestran.
interface ReportsViewProps {
  projectId: string;
}

// Estado de la generación: sin pedido, en curso, con el reporte a la vista o con un error.
type GenerationState =
  | { status: "idle" }
  | { status: "generating" }
  | { status: "shown"; view: ReportView }
  | { status: "error"; message: string };

// Mensaje cuando el servidor falla o su respuesta no se reconoce.
const GENERIC_ERROR = "No se pudo generar el reporte. Intentá de nuevo.";

// Mensaje cuando falla la generación o la descarga del PDF.
const EXPORT_ERROR = "No se pudo generar el PDF. Intentá de nuevo.";

// Vista de reportes del proyecto: elige el alcance, genera el reporte del sprint o del proyecto
// completo y lo muestra.
export function ReportsView({ projectId }: ReportsViewProps) {
  // Solo se ofrecen el sprint Activo y los Cerrados; los Planificados no tienen nada que reportar.
  const loadSprints = useCallback(
    () => listSprints(projectId, ["active", "closed"]),
    [projectId],
  );
  const sprints = useAuthorizedQuery(loadSprints);
  const [scope, setScope] = useState(PROJECT_SCOPE);
  const [generation, setGeneration] = useState<GenerationState>({
    status: "idle",
  });
  // Estado de la exportación: no toca el reporte en pantalla (FR-033).
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  // Zona horaria del navegador, con la que se muestra la fecha de generación.
  const timeZone = useMemo(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
    [],
  );

  // Pide el reporte del alcance elegido; los rechazos del dominio muestran el mensaje del backend.
  async function handleGenerate() {
    setGeneration({ status: "generating" });
    const result =
      scope === PROJECT_SCOPE
        ? await generateProjectReport(projectId)
        : await generateSprintReport(projectId, scope);
    if (result.ok) {
      setGeneration({
        status: "shown",
        view: buildReportView(result.data, timeZone),
      });
      return;
    }
    const { status, message } = result.error;
    setGeneration({
      status: "error",
      message: status === 404 || status === 409 ? message : GENERIC_ERROR,
    });
  }

  // Exporta a PDF el reporte en pantalla; una falla se informa sin alterar el reporte.
  async function handleExport(view: ReportView) {
    setExporting(true);
    setExportError(null);
    try {
      await exportReportPdf(buildPdfDocument(view), view.fileName);
    } catch {
      setExportError(EXPORT_ERROR);
    } finally {
      setExporting(false);
    }
  }

  if (sprints.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando reportes…
      </p>
    );
  }
  if (sprints.status === "error") {
    return <Alert>{sprints.message}</Alert>;
  }

  const isGenerating = generation.status === "generating";
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 sm:max-w-md">
        <ReportScopeSelect
          sprints={sprints.data.sprints}
          value={scope}
          onChange={setScope}
          disabled={isGenerating}
        />
        <Button onClick={handleGenerate} disabled={isGenerating}>
          {isGenerating ? "Generando…" : "Generar reporte"}
        </Button>
      </div>
      {generation.status === "error" && <Alert>{generation.message}</Alert>}
      {generation.status === "shown" && (
        <>
          <div className="flex flex-col gap-4 sm:max-w-md">
            <Button
              onClick={() => handleExport(generation.view)}
              disabled={exporting}
            >
              {exporting ? "Exportando…" : "Exportar a PDF"}
            </Button>
            {exportError && <Alert>{exportError}</Alert>}
          </div>
          <ReportDocument view={generation.view} />
        </>
      )}
    </div>
  );
}

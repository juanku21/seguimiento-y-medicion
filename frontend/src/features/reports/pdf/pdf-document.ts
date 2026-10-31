// Documento de pdfmake del reporte: se arma solo con el modelo de presentación.
import type { PdfDocumentDefinition } from "pdfmake/build/pdfmake";
import type { ReportView } from "@/features/reports/view-model";

// Firma mínima: la implementación llega en T027.
export function buildPdfDocument(view: ReportView): PdfDocumentDefinition {
  void view;
  throw new Error("not implemented");
}

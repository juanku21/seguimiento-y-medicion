// Exportación del reporte a PDF: carga pdfmake y descarga el documento.
import type { PdfDocumentDefinition } from "pdfmake/build/pdfmake";

// Firma mínima: la implementación llega en T028.
export async function exportReportPdf(
  definition: PdfDocumentDefinition,
  fileName: string,
): Promise<void> {
  void definition;
  void fileName;
  throw new Error("not implemented");
}

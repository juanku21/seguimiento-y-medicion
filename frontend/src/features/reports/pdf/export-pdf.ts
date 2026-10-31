// Exportación del reporte a PDF: carga pdfmake y descarga el documento.
import type { PdfDocumentDefinition } from "pdfmake/build/pdfmake";

// Genera el PDF y lo descarga con el nombre indicado. pdfmake y sus fuentes se cargan con import()
// recién al exportar: no pesan en la carga de la página y no se evalúan en el servidor de Next.js
// (research.md, sección 14).
export async function exportReportPdf(
  definition: PdfDocumentDefinition,
  fileName: string,
): Promise<void> {
  const pdfMake = await import("pdfmake/build/pdfmake");
  const fonts = await import("pdfmake/build/vfs_fonts");
  // Registra la fuente embebida (Roboto) antes de crear el documento.
  pdfMake.addVirtualFileSystem(fonts.default);
  await pdfMake.createPdf(definition).download(fileName);
}

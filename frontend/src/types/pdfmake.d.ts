// Declaraciones mínimas de pdfmake 0.3.13 (el paquete no trae tipos): solo lo que usa la exportación del reporte.

declare module "pdfmake/build/pdfmake" {
  // Celda o bloque del documento: texto con estilo, pila de bloques o tabla.
  export type PdfContent =
    | string
    | {
        text?: string | PdfContent[];
        style?: string | string[];
        bold?: boolean;
        italics?: boolean;
        fontSize?: number;
        color?: string;
        alignment?: "left" | "center" | "right";
        margin?: number | number[];
        fillColor?: string;
        stack?: PdfContent[];
        table?: {
          headerRows?: number;
          dontBreakRows?: boolean;
          widths?: (string | number)[];
          body: PdfContent[][];
        };
      };

  // Definición del documento que recibe createPdf.
  export type PdfDocumentDefinition = {
    content: PdfContent[];
    footer?: (currentPage: number, pageCount: number) => PdfContent;
    pageMargins?: number | number[];
    styles?: Record<string, Exclude<PdfContent, string>>;
    defaultStyle?: Exclude<PdfContent, string>;
    info?: { title?: string };
  };

  // Crea el PDF; download lo descarga en el navegador.
  export function createPdf(definition: PdfDocumentDefinition): {
    download(fileName: string): Promise<void>;
  };

  // Registra las fuentes embebidas.
  export function addVirtualFileSystem(vfs: Record<string, string>): void;
}

declare module "pdfmake/build/vfs_fonts" {
  // Fuentes embebidas en base64, por nombre de archivo.
  const vfs: Record<string, string>;
  export default vfs;
}

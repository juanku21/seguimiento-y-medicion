// Documento de pdfmake del reporte: se arma solo con el modelo de presentación.
import type { PdfContent, PdfDocumentDefinition } from "pdfmake/build/pdfmake";
import type {
  BlockView,
  ReportView,
  SectionView,
  TableView,
} from "@/features/reports/view-model";

// Índice de la columna de texto largo: la que acumula más caracteres en sus celdas.
function longTextColumn(table: TableView): number {
  const lengths = table.columns.map((column) => column.length);
  [...table.rows, ...(table.footer ? [table.footer] : [])].forEach((row) =>
    row.forEach((cell, index) => {
      lengths[index] += cell.length;
    }),
  );
  return lengths.indexOf(Math.max(...lengths));
}

// Tabla de pdfmake: encabezado repetido, filas sin partir y pie en negrita como última fila.
function buildTable(table: TableView): PdfContent {
  const longColumn = longTextColumn(table);
  const body: PdfContent[][] = [
    table.columns.map((column) => ({ text: column, style: "tableHeader" })),
    // Copia de cada fila: pdfmake muta los arreglos y no debe tocar la vista.
    ...table.rows.map((row) => [...row]),
  ];
  if (table.footer) {
    body.push(table.footer.map((cell) => ({ text: cell, bold: true })));
  }
  return {
    table: {
      headerRows: 1,
      dontBreakRows: true,
      widths: table.columns.map((_, index) =>
        index === longColumn ? "*" : "auto",
      ),
      body,
    },
    margin: [0, 4, 0, 8],
  };
}

// Bloque de una sección: subtítulo, mensaje y tabla, en ese orden.
function buildBlock(block: BlockView): PdfContent[] {
  const content: PdfContent[] = [];
  if (block.heading !== null) {
    content.push({ text: block.heading, style: "blockHeading" });
  }
  if (block.message !== null) {
    content.push({ text: block.message, style: "message" });
  }
  if (block.table) {
    content.push(buildTable(block.table));
  }
  return content;
}

// Sección: título, mensaje, datos del encabezado (solo en la primera) y bloques.
function buildSection(section: SectionView, view: ReportView): PdfContent[] {
  const content: PdfContent[] = [
    { text: section.title, style: "sectionTitle" },
  ];
  if (section.message !== null) {
    content.push({ text: section.message, style: "message" });
  }
  if (section.key === "header") {
    view.header.forEach((item) =>
      content.push({
        stack: [
          { text: item.label, style: "label" },
          { text: item.value, margin: [0, 0, 0, 4] },
        ],
      }),
    );
  }
  section.blocks.forEach((block) => content.push(...buildBlock(block)));
  return content;
}

// Arma el documento completo a partir de la vista, sin formatear nada.
export function buildPdfDocument(view: ReportView): PdfDocumentDefinition {
  const content: PdfContent[] = [{ text: view.title, style: "title" }];
  if (view.partialNote !== null) {
    content.push({ text: view.partialNote, style: "partialNote" });
  }
  view.sections.forEach((section) =>
    content.push(...buildSection(section, view)),
  );

  return {
    content,
    // Pie de cada página: numeración y, en reportes parciales, la nota.
    footer: (currentPage, pageCount) => ({
      stack: [
        { text: `Página ${currentPage} de ${pageCount}`, alignment: "center" },
        ...(view.partialNote !== null
          ? [
              {
                text: view.partialNote,
                alignment: "center" as const,
                color: "#b45309",
              },
            ]
          : []),
      ],
      fontSize: 8,
      margin: [40, 8, 40, 0],
    }),
    pageMargins: [40, 40, 40, 50],
    styles: {
      title: { fontSize: 20, bold: true, margin: [0, 0, 0, 6] },
      partialNote: {
        bold: true,
        color: "#b45309",
        fillColor: "#fef3c7",
        margin: [0, 0, 0, 8],
      },
      sectionTitle: { fontSize: 14, bold: true, margin: [0, 14, 0, 6] },
      blockHeading: { fontSize: 11, bold: true, margin: [0, 6, 0, 2] },
      message: { italics: true, color: "#475569", margin: [0, 0, 0, 4] },
      label: { bold: true, color: "#475569", fontSize: 9 },
      tableHeader: { bold: true, fillColor: "#f1f5f9" },
    },
    defaultStyle: { font: "Roboto", fontSize: 10 },
    info: { title: view.title },
  };
}

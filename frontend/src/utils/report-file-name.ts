import type { ReportScope } from "@/types/report";

export type ReportFileNameInput = {
  projectName: string;
  scope: ReportScope;
  sprintName: string | null;
  generatedAt: string;
  timeZone: string;
};

const MAX_PART_LENGTH = 60;

// Sanea una parte variable del nombre: quita diacríticos, reemplaza lo que no sea [A-Za-z0-9-] por
// guiones, colapsa y recorta guiones, limita el largo y usa el valor por defecto si queda vacía.
function sanitizePart(value: string, fallback: string): string {
  const clean = value
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .replace(/[^A-Za-z0-9-]/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, MAX_PART_LENGTH)
    .replace(/-+$/g, "");
  return clean === "" ? fallback : clean;
}

// Arma AAAA-MM-DD_HH-mm-ss del instante en la zona IANA indicada, como formatInstant: Intl separa
// las partes y se ordenan en un formato fijo.
function formatTimestamp(isoInstant: string, timeZone: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(isoInstant));
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((item) => item.type === type)?.value ?? "";
  return `${part("year")}-${part("month")}-${part("day")}_${part("hour")}-${part("minute")}-${part("second")}`;
}

// Devuelve Reporte_<proyecto>_<alcance>_<AAAA-MM-DD>_<HH-mm-ss>.pdf con cada parte saneada.
export function reportFileName(input: ReportFileNameInput): string {
  const project = sanitizePart(input.projectName, "proyecto");
  const scope =
    input.scope === "sprint"
      ? `sprint-${sanitizePart(input.sprintName ?? "", "sprint")}`
      : "proyecto";
  const timestamp = formatTimestamp(input.generatedAt, input.timeZone);
  return `Reporte_${project}_${scope}_${timestamp}.pdf`;
}

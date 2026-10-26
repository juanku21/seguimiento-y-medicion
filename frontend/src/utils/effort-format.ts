// Formato de horas y porcentajes del esfuerzo (research.md sección 14). Un valor null es "no
// calculable" y se muestra con ese texto, nunca como cero.
const notCalculable = "No calculable";

// Muestra las horas con coma decimal, sin ceros de relleno, y la unidad, o "No calculable" si llegan
// en null.
export function formatHours(hours: number | null): string {
  if (hours === null) {
    return notCalculable;
  }
  return `${String(hours).replace(".", ",")} h`;
}

// Muestra el porcentaje con signo y exactamente 2 decimales, o "No calculable" si llega en null. El
// positivo lleva "+", el negativo el signo menos tipográfico (U+2212) y el cero no lleva signo.
export function formatPercent(percent: number | null): string {
  if (percent === null) {
    return notCalculable;
  }
  const magnitude = Math.abs(percent).toFixed(2).replace(".", ",");
  let sign = "";
  if (magnitude !== "0,00") {
    sign = percent > 0 ? "+" : "\u2212";
  }
  return `${sign}${magnitude} %`;
}

// Muestra una diferencia de horas con el mismo criterio de signo que formatPercent: "+" si es
// positiva, el signo menos tipográfico (U+2212) si es negativa y sin signo en 0, o "No calculable"
// si llega en null.
export function formatSignedHours(hours: number | null): string {
  if (hours === null) {
    return notCalculable;
  }
  let sign = "";
  if (hours !== 0) {
    sign = hours > 0 ? "+" : "−";
  }
  return `${sign}${formatHours(Math.abs(hours))}`;
}

// Muestra los Story Points como "<n> SP", o "Sin estimar" si llegan en null, de modo que una
// historia de 0 Story Points se distingue de una sin estimar (FR-055).
export function formatStoryPoints(storyPoints: number | null): string {
  if (storyPoints === null) {
    return "Sin estimar";
  }
  return `${storyPoints} SP`;
}

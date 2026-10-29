// Formato de velocidad y porcentajes del dashboard (FR-013).
// El valor llega con a lo sumo 2 decimales desde la 008: toFixed(2) solo restituye
// los ceros que JSON.parse quitó, no redondea nada.

// Formatea un número con 2 decimales y coma decimal.
export function formatDecimal(value: number): string {
  return value.toFixed(2).replace(".", ",");
}

// Formatea un porcentaje con 2 decimales, coma decimal, " %" y sin signo.
export function formatShare(value: number): string {
  return `${formatDecimal(value)} %`;
}

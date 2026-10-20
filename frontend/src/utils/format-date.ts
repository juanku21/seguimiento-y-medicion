// Convierte una fecha AAAA-MM-DD, o un instante ISO que empieza con ella, en DD/MM/AAAA partiendo
// el texto, sin pasar por Date para que la zona horaria no corra el día.
export function formatDate(isoDate: string): string {
  const [year, month, day] = isoDate.slice(0, 10).split("-");
  return `${day}/${month}/${year}`;
}

// Convierte un instante ISO en UTC en la fecha DD/MM/AAAA del día que corresponde en la zona horaria
// IANA indicada. Firma mínima de la etapa RED de T085.
export function formatInstant(isoInstant: string, timeZone: string): string {
  void isoInstant;
  void timeZone;
  throw new Error("not implemented");
}

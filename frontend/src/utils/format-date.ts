// Convierte una fecha AAAA-MM-DD, o un instante ISO que empieza con ella, en DD/MM/AAAA partiendo
// el texto, sin pasar por Date para que la zona horaria no corra el día.
export function formatDate(isoDate: string): string {
  const [year, month, day] = isoDate.slice(0, 10).split("-");
  return `${day}/${month}/${year}`;
}

// Convierte un instante ISO en UTC en la fecha DD/MM/AAAA del día que corresponde en la zona horaria
// IANA indicada: Intl separa día, mes y año del instante en esa zona y se arman en el orden fijo.
export function formatInstant(isoInstant: string, timeZone: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date(isoInstant));
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((item) => item.type === type)?.value ?? "";
  return `${part("day")}/${part("month")}/${part("year")}`;
}

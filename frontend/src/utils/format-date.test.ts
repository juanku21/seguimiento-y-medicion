import { describe, expect, it } from "vitest";
import { formatDate, formatInstant } from "@/utils/format-date";

// Pruebas unitarias del formateo de fechas (T085). Las fechas de calendario (startDate,
// plannedEndDate, actualEndDate) se muestran tal como llegan, sin conversión de zona; los instantes
// UTC (joinedAt) se muestran con el día que corresponde en la zona horaria indicada. La zona se pasa
// explícita para que el resultado no dependa de la zona de la máquina que corre las pruebas.
// Es cobertura adicional sin escenario de spec.md: el describe no lleva el prefijo 002/USn para no
// alterar el conteo de escenarios de T084.

// argentinaTimeZone es la zona del sistema configurada en APP_TIMEZONE (UTC-3, sin horario de verano).
const argentinaTimeZone = "America/Argentina/Buenos_Aires";

describe("Formateo de fechas de calendario y de instantes", () => {
  // Caso del defecto: a las 23:00 del 19/10 en Argentina el instante UTC ya es del 20/10.
  it("muestra un instante UTC de la noche argentina con el día de Argentina", () => {
    expect(formatInstant("2026-10-20T02:00:03Z", argentinaTimeZone)).toBe(
      "19/10/2026",
    );
  });

  // Caso límite: las 00:00 de Argentina son las 03:00 UTC y ya cuentan como el día siguiente.
  it("muestra la medianoche argentina con el día nuevo", () => {
    expect(formatInstant("2026-10-20T03:00:00Z", argentinaTimeZone)).toBe(
      "20/10/2026",
    );
    expect(formatInstant("2026-10-20T02:59:59Z", argentinaTimeZone)).toBe(
      "19/10/2026",
    );
  });

  // Caso alternativo: en UTC el instante conserva su propio día.
  it("muestra el día UTC cuando la zona es UTC", () => {
    expect(formatInstant("2026-10-20T02:00:03Z", "UTC")).toBe("20/10/2026");
  });

  // Caso de regresión: una fecha de calendario no se corre de día por ninguna zona horaria.
  it("muestra una fecha de calendario sin conversión de zona", () => {
    expect(formatDate("2026-10-20")).toBe("20/10/2026");
    expect(formatDate("2026-01-01")).toBe("01/01/2026");
  });
});

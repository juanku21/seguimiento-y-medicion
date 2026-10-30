import { describe, expect, it } from "vitest";
import { formatDecimal, formatShare } from "@/utils/metric-format";

// Pruebas unitarias del formato de velocidad y porcentajes del dashboard (T010; FR-013).
// Los formateadores solo restituyen los 2 decimales y cambian el punto por la coma: nunca redondean.

describe("009/US2 - Formato de métricas del dashboard", () => {
  // La velocidad se muestra con coma decimal y exactamente 2 decimales.
  it("formatDecimal muestra 2 decimales con coma", () => {
    expect(formatDecimal(14)).toBe("14,00");
    expect(formatDecimal(0)).toBe("0,00");
    expect(formatDecimal(13.1)).toBe("13,10");
  });

  // Los porcentajes llevan " %", 2 decimales y nunca el signo "+".
  it("formatShare muestra 2 decimales, el símbolo % y sin signo", () => {
    expect(formatShare(27.78)).toBe("27,78 %");
    expect(formatShare(0)).toBe("0,00 %");
    expect(formatShare(100)).toBe("100,00 %");
    expect(formatShare(27.78)).not.toContain("+");
  });

  // El texto leído de vuelta es exactamente el número recibido (ningún redondeo).
  it.each([14, 27.78, 13.1, 0])(
    "el texto de %s vuelve al mismo número",
    (value) => {
      const decimal = Number(formatDecimal(value).replace(",", "."));
      const share = Number(
        formatShare(value).replace(" %", "").replace(",", "."),
      );
      expect(decimal).toBe(value);
      expect(share).toBe(value);
    },
  );
});

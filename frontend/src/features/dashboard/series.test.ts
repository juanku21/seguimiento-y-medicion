import { describe, expect, it } from "vitest";
import { closedSprintPoints } from "@/features/dashboard/series";
import type { SeriesPoint } from "@/types/metrics";

// Punto de serie con el estado de sprint indicado.
const point = (
  sprintId: string,
  sprintStatus: "active" | "closed",
): SeriesPoint => ({
  sprintId,
  sprintName: `Sprint ${sprintId}`,
  sprintStatus,
  status: "available",
  value: 3,
  finality: sprintStatus === "active" ? "partial" : "definitive",
});

describe("closedSprintPoints (RN-20)", () => {
  it("omite el sprint activo y conserva los mismos objetos en el mismo orden", () => {
    const a = point("A", "closed");
    const b = point("B", "active");
    const c = point("C", "closed");
    const result = closedSprintPoints([a, b, c]);
    expect(result).toHaveLength(2);
    expect(result[0]).toBe(a);
    expect(result[1]).toBe(c);
  });

  it("devuelve la lista igual si no hay sprint activo", () => {
    const a = point("A", "closed");
    const c = point("C", "closed");
    expect(closedSprintPoints([a, c])).toEqual([a, c]);
  });
});

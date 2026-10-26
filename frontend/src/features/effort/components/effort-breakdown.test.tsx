import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { EffortBreakdown } from "@/features/effort/components/effort-breakdown";
import type { MemberEffort } from "@/types/effort";

// Pruebas del desglose del esfuerzo de una historia (T035). El componente es de presentación: recibe
// el total y el desglose tal como los devuelve GET /api/v1/stories/{storyId}/effort (StoryEffort de
// contracts/openapi.yaml), que ya los calcula y ordena el backend (T034).
//
// Contrato que fijan estas pruebas para el desglose:
// - Recibe totalHours y byMember.
// - El total se muestra como el texto "Total: <horas>", con las horas en el formato de formatHours
//   (coma decimal y unidad: "16 h", "0,25 h").
// - El desglose es una lista con nombre accesible "Esfuerzo por integrante", con un elemento por
//   persona en el orden recibido, que muestra su nombre completo y sus horas con formatHours.
// - Sin personas, el total es "Total: 0 h" y no hay ningún elemento en el desglose.

const ana: MemberEffort = {
  user: { id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d", fullName: "Ana Pérez" },
  hours: 10,
};
const bruno: MemberEffort = {
  user: { id: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9", fullName: "Bruno Díaz" },
  hours: 6,
};

// Devuelve los elementos del desglose por persona.
function breakdownItems(): HTMLElement[] {
  const list = screen.getByRole("list", { name: "Esfuerzo por integrante" });
  return within(list).queryAllByRole("listitem");
}

afterEach(() => {
  cleanup();
});

describe("006/US2 - Consultar el esfuerzo de una historia", () => {
  it("Caso normal — total de 16 horas con 10 para una persona y 6 para otra", () => {
    render(<EffortBreakdown totalHours={16} byMember={[ana, bruno]} />);

    expect(screen.getByText("Total: 16 h")).toBeTruthy();
    const items = breakdownItems();
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("Ana Pérez")).toBeTruthy();
    expect(within(items[0]).getByText("10 h")).toBeTruthy();
    expect(within(items[1]).getByText("Bruno Díaz")).toBeTruthy();
    expect(within(items[1]).getByText("6 h")).toBeTruthy();
  });

  it("Caso alternativo — registros de una persona quitada del proyecto", () => {
    // La ex integrante llega en byMember como cualquier otra persona y sus horas cuentan en el total.
    const carla: MemberEffort = {
      user: {
        id: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d",
        fullName: "Carla Ruiz",
      },
      hours: 3.5,
    };
    render(<EffortBreakdown totalHours={13.5} byMember={[ana, carla]} />);

    expect(screen.getByText("Total: 13,5 h")).toBeTruthy();
    const items = breakdownItems();
    expect(items).toHaveLength(2);
    expect(within(items[1]).getByText("Carla Ruiz")).toBeTruthy();
    expect(within(items[1]).getByText("3,5 h")).toBeTruthy();
  });

  it("Caso límite — historia sin ningún registro de esfuerzo", () => {
    render(<EffortBreakdown totalHours={0} byMember={[]} />);

    expect(screen.getByText("Total: 0 h")).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
  });

  it("Caso límite — un único registro de 0,25 horas", () => {
    const quarter: MemberEffort = { ...ana, hours: 0.25 };
    render(<EffortBreakdown totalHours={0.25} byMember={[quarter]} />);

    expect(screen.getByText("Total: 0,25 h")).toBeTruthy();
    const items = breakdownItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("0,25 h")).toBeTruthy();
  });

  it("Caso límite — una sola persona con esfuerzo en tres fechas distintas", () => {
    // El backend ya suma las tres fechas en una sola línea: el desglose no repite a la persona.
    const anaThreeDates: MemberEffort = { ...ana, hours: 9.75 };
    render(<EffortBreakdown totalHours={9.75} byMember={[anaThreeDates]} />);

    expect(screen.getByText("Total: 9,75 h")).toBeTruthy();
    const items = breakdownItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("Ana Pérez")).toBeTruthy();
    expect(within(items[0]).getByText("9,75 h")).toBeTruthy();
  });
});

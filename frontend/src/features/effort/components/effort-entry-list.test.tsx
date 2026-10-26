import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { EffortEntryList } from "@/features/effort/components/effort-entry-list";
import type { EffortEntry } from "@/types/effort";

// Pruebas de la lista de registros de esfuerzo de una historia (T035). El componente es de
// presentación: recibe los registros tal como los devuelve GET /api/v1/stories/{storyId}/effort
// (entries de StoryEffort en contracts/openapi.yaml), ya ordenados por el backend (FR-035, T034).
//
// Contrato que fijan estas pruebas para la lista:
// - Recibe entries.
// - Los registros se muestran en una lista con nombre accesible "Registros de esfuerzo", un elemento
//   por registro en el orden recibido, con la fecha en DD/MM/AAAA, el nombre completo del autor, la
//   actividad, las horas con formatHours ("6,5 h") y el nombre del sprint, o "Sin sprint" cuando el
//   registro no tiene sprint asociado.
// - Sin registros se muestra el aviso "Todavía no hay esfuerzo registrado." y ningún elemento de
//   lista; no es un error, así que no se usa role="alert".

const ana = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const bruno = {
  id: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9",
  fullName: "Bruno Díaz",
};
const story = {
  id: "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f",
  title: "Alta de clientes",
};

// Arma un registro de esfuerzo de la historia con los datos que importan en cada prueba.
function entry(
  id: string,
  overrides: Partial<EffortEntry> & Pick<EffortEntry, "entryDate">,
): EffortEntry {
  return {
    id,
    activity: "Desarrollo del formulario",
    hours: 2,
    author: ana,
    story,
    sprint: null,
    definitive: false,
    createdAt: `${overrides.entryDate}T15:00:00Z`,
    updatedAt: `${overrides.entryDate}T15:00:00Z`,
    ...overrides,
  };
}

// Devuelve los elementos de la lista de registros.
function entryItems(): HTMLElement[] {
  const list = screen.getByRole("list", { name: "Registros de esfuerzo" });
  return within(list).queryAllByRole("listitem");
}

afterEach(() => {
  cleanup();
});

describe("006/US2 - Consultar el esfuerzo de una historia", () => {
  it("Caso normal — cada registro con fecha, autor, actividad, horas y sprint", () => {
    const entries = [
      entry("a1", {
        entryDate: "2026-10-20",
        activity: "Diseño de la pantalla",
        hours: 6.5,
        author: ana,
        sprint: { id: "s1", name: "Sprint 2", status: "active" },
      }),
      entry("b1", {
        entryDate: "2026-10-19",
        activity: "Revisión de código",
        hours: 4,
        author: bruno,
        sprint: { id: "s1", name: "Sprint 2", status: "active" },
      }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("20/10/2026")).toBeTruthy();
    expect(within(items[0]).getByText("Ana Pérez")).toBeTruthy();
    expect(within(items[0]).getByText("Diseño de la pantalla")).toBeTruthy();
    expect(within(items[0]).getByText("6,5 h")).toBeTruthy();
    expect(within(items[0]).getByText("Sprint 2")).toBeTruthy();
    expect(within(items[1]).getByText("19/10/2026")).toBeTruthy();
    expect(within(items[1]).getByText("Bruno Díaz")).toBeTruthy();
    expect(within(items[1]).getByText("Revisión de código")).toBeTruthy();
    expect(within(items[1]).getByText("4 h")).toBeTruthy();
  });

  it("Caso alternativo — registros de dos sprints distintos y uno sin sprint", () => {
    const entries = [
      entry("e1", {
        entryDate: "2026-10-22",
        activity: "Pruebas de aceptación",
        sprint: { id: "s2", name: "Sprint 3", status: "active" },
      }),
      entry("e2", {
        entryDate: "2026-10-12",
        activity: "Análisis del requisito",
        sprint: { id: "s1", name: "Sprint 2", status: "closed" },
        definitive: true,
      }),
      entry("e3", {
        entryDate: "2026-10-05",
        activity: "Relevamiento inicial",
        sprint: null,
      }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(3);
    expect(within(items[0]).getByText("Sprint 3")).toBeTruthy();
    expect(within(items[1]).getByText("Sprint 2")).toBeTruthy();
    expect(within(items[2]).getByText("Sin sprint")).toBeTruthy();
    expect(within(items[0]).queryByText("Sin sprint")).toBeNull();
    expect(within(items[1]).queryByText("Sin sprint")).toBeNull();
  });

  it("Caso alternativo — registros de una persona quitada del proyecto", () => {
    // El registro de la ex integrante llega con su nombre como cualquier otro y se muestra igual.
    const carla = {
      id: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d",
      fullName: "Carla Ruiz",
    };
    render(
      <EffortEntryList
        entries={[
          entry("c1", {
            entryDate: "2026-10-08",
            activity: "Maquetado",
            author: carla,
          }),
        ]}
      />,
    );

    const items = entryItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("Carla Ruiz")).toBeTruthy();
  });

  it("Caso límite — historia sin ningún registro de esfuerzo", () => {
    render(<EffortEntryList entries={[]} />);

    expect(
      screen.getByText("Todavía no hay esfuerzo registrado."),
    ).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("Caso límite — un único registro de 0,25 horas", () => {
    render(
      <EffortEntryList
        entries={[entry("q1", { entryDate: "2026-10-21", hours: 0.25 })]}
      />,
    );

    const items = entryItems();
    expect(items).toHaveLength(1);
    expect(within(items[0]).getByText("0,25 h")).toBeTruthy();
  });

  it("Caso límite — una sola persona con esfuerzo en tres fechas distintas", () => {
    const entries = [
      entry("d3", { entryDate: "2026-10-21", activity: "Tercer día" }),
      entry("d2", { entryDate: "2026-10-20", activity: "Segundo día" }),
      entry("d1", { entryDate: "2026-10-19", activity: "Primer día" }),
    ];
    render(<EffortEntryList entries={entries} />);

    const items = entryItems();
    expect(items).toHaveLength(3);
    expect(within(items[0]).getByText("21/10/2026")).toBeTruthy();
    expect(within(items[1]).getByText("20/10/2026")).toBeTruthy();
    expect(within(items[2]).getByText("19/10/2026")).toBeTruthy();
    expect(
      screen.queryByText("Todavía no hay esfuerzo registrado."),
    ).toBeNull();
  });
});

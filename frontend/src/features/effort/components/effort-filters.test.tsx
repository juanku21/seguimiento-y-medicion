import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import {
  EffortFilters,
  type SprintOption,
} from "@/features/effort/components/effort-filters";
import type { ProjectEffortFilter, StoryRef, UserRef } from "@/types/effort";

// Pruebas de los filtros del esfuerzo del proyecto (T057). El componente es controlado y de
// presentación: recibe los filtros elegidos y las opciones, y comunica cada cambio. La consulta que
// se arma con esos filtros se prueba en project-effort-view.test.tsx.
//
// Contrato que fijan estas pruebas para EffortFilters (T062):
// - Recibe value (ProjectEffortFilter, con "" en cada filtro sin aplicar), onChange, members,
//   stories y sprints.
// - Ofrece tres selectores (role="combobox") con nombre accesible "Integrante", "Historia" y
//   "Sprint", y dos campos de fecha con etiqueta "Desde" y "Hasta".
// - "Integrante" tiene primero la opción "Todos" (valor "") y después una opción por persona
//   recibida, en el orden recibido, con su nombre completo y su id como valor.
// - "Historia" tiene primero "Todas" (valor "") y después una opción por historia con su título.
// - "Sprint" tiene primero "Todos" (valor ""), después "Sin sprint" (valor "none") y una opción por
//   sprint con su nombre (FR-039).
// - Cada cambio llama a onChange con los cinco filtros completos, conservando los que no cambiaron
//   (los filtros se combinan, FR-037).

const ana: UserRef = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const bruno: UserRef = {
  id: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9",
  fullName: "Bruno Díaz",
};
const carla: UserRef = {
  id: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d",
  fullName: "Carla Ruiz",
};
const clientsStory: StoryRef = {
  id: "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f",
  title: "Alta de clientes",
};
const velocityStory: StoryRef = {
  id: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  title: "Reporte de velocidad",
};
const sprintOne: SprintOption = {
  id: "7f6e5d4c-3b2a-4190-8f7e-6d5c4b3a2910",
  name: "Sprint 1",
};
const sprintTwo: SprintOption = {
  id: "8a7b6c5d-4e3f-4a1b-9c8d-7e6f5a4b3c2d",
  name: "Sprint 2",
};

const noFilters: ProjectEffortFilter = {
  userId: "",
  storyId: "",
  sprintId: "",
  from: "",
  to: "",
};

// Renderiza los filtros con las opciones de prueba y devuelve el espía de onChange.
function renderFilters(value: ProjectEffortFilter = noFilters) {
  const onChange = vi.fn<(next: ProjectEffortFilter) => void>();
  render(
    <EffortFilters
      value={value}
      onChange={onChange}
      members={[ana, bruno, carla]}
      stories={[clientsStory, velocityStory]}
      sprints={[sprintOne, sprintTwo]}
    />,
  );
  return onChange;
}

// Localiza un selector de filtro por su nombre accesible.
function select(name: "Integrante" | "Historia" | "Sprint"): HTMLSelectElement {
  return screen.getByRole<HTMLSelectElement>("combobox", { name });
}

// Devuelve los textos de las opciones de un selector, en orden.
function optionLabels(name: "Integrante" | "Historia" | "Sprint"): string[] {
  return within(select(name))
    .getAllByRole("option")
    .map((option) => option.textContent ?? "");
}

// Devuelve los filtros entregados en la única llamada a onChange.
function onlyChange(
  onChange: ReturnType<typeof renderFilters>,
): ProjectEffortFilter {
  expect(onChange).toHaveBeenCalledTimes(1);
  return onChange.mock.calls[0][0];
}

afterEach(() => {
  cleanup();
});

describe("006/US4 - Consultar el esfuerzo del proyecto con filtros", () => {
  // Caso normal: sin filtros elegidos, cada selector ofrece la opción de no filtrar y las opciones
  // recibidas, y nada está aplicado.
  it("Caso normal — sin filtros, con todas las opciones disponibles", () => {
    renderFilters();

    expect(optionLabels("Integrante")).toEqual([
      "Todos",
      "Ana Pérez",
      "Bruno Díaz",
      "Carla Ruiz",
    ]);
    expect(optionLabels("Historia")).toEqual([
      "Todas",
      "Alta de clientes",
      "Reporte de velocidad",
    ]);
    expect(optionLabels("Sprint")).toEqual([
      "Todos",
      "Sin sprint",
      "Sprint 1",
      "Sprint 2",
    ]);
    expect(select("Integrante").value).toBe("");
    expect(select("Historia").value).toBe("");
    expect(select("Sprint").value).toBe("");
    expect(screen.getByLabelText<HTMLInputElement>("Desde").value).toBe("");
    expect(screen.getByLabelText<HTMLInputElement>("Hasta").value).toBe("");
  });

  // Caso alternativo: elegir un integrante entrega su id en userId sin tocar los demás filtros.
  it("Caso alternativo — filtro por un integrante concreto", () => {
    const onChange = renderFilters();

    fireEvent.change(select("Integrante"), { target: { value: bruno.id } });

    expect(onlyChange(onChange)).toEqual({ ...noFilters, userId: bruno.id });
  });

  // Caso alternativo: elegir un sprint entrega su id en sprintId.
  it("Caso alternativo — filtro por un sprint concreto", () => {
    const onChange = renderFilters();

    fireEvent.change(select("Sprint"), { target: { value: sprintOne.id } });

    expect(onlyChange(onChange)).toEqual({
      ...noFilters,
      sprintId: sprintOne.id,
    });
  });

  // Caso alternativo: con un rango ya elegido, elegir una historia conserva el rango, y cambiar el
  // fin del rango conserva la historia y el inicio (FR-037).
  it("Caso alternativo — rango de fechas e historia a la vez", () => {
    const value: ProjectEffortFilter = {
      ...noFilters,
      storyId: clientsStory.id,
      from: "2026-10-10",
    };
    const onChange = renderFilters(value);
    expect(select("Historia").value).toBe(clientsStory.id);
    expect(screen.getByLabelText<HTMLInputElement>("Desde").value).toBe(
      "2026-10-10",
    );

    fireEvent.change(screen.getByLabelText("Hasta"), {
      target: { value: "2026-10-22" },
    });

    expect(onlyChange(onChange)).toEqual({ ...value, to: "2026-10-22" });
  });

  // Caso alternativo: "Sin sprint" es una opción propia del filtro por sprint y se entrega como
  // sprintId "none" (FR-039).
  it("Caso alternativo — filtro por sin sprint", () => {
    const onChange = renderFilters();

    fireEvent.change(select("Sprint"), { target: { value: "none" } });

    expect(onlyChange(onChange)).toEqual({ ...noFilters, sprintId: "none" });
  });

  // Caso límite: el mismo día como inicio y fin del rango se entrega tal cual, sin rechazarlo.
  it("Caso límite — rango de un solo día", () => {
    const value: ProjectEffortFilter = { ...noFilters, from: "2026-10-20" };
    const onChange = renderFilters(value);

    fireEvent.change(screen.getByLabelText("Hasta"), {
      target: { value: "2026-10-20" },
    });

    expect(onlyChange(onChange)).toEqual({ ...value, to: "2026-10-20" });
  });

  // Caso límite: una persona que ya no es integrante se ofrece y se elige como cualquier otra, si
  // llega entre las opciones (FR-040).
  it("Caso límite — filtro por una persona quitada del proyecto", () => {
    const onChange = renderFilters();

    fireEvent.change(select("Integrante"), { target: { value: carla.id } });

    expect(onlyChange(onChange)).toEqual({ ...noFilters, userId: carla.id });
  });

  // Volver a "Todos" quita el filtro por integrante y conserva los demás.
  it("al elegir Todos quita el filtro por integrante", () => {
    const value: ProjectEffortFilter = {
      ...noFilters,
      userId: bruno.id,
      sprintId: "none",
    };
    const onChange = renderFilters(value);
    expect(select("Integrante").value).toBe(bruno.id);
    expect(select("Sprint").value).toBe("none");

    fireEvent.change(select("Integrante"), { target: { value: "" } });

    expect(onlyChange(onChange)).toEqual({ ...value, userId: "" });
  });
});

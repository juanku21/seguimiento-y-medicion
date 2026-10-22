import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { SprintResults } from "@/features/sprints/components/sprint-results";
import type {
  SprintDetail,
  SprintResults as SprintResultsData,
  SprintStatus,
} from "@/types/sprint";

// Pruebas del resultado congelado de un sprint cerrado (T065). El componente es de presentación:
// recibe el SprintDetail que la pantalla ya tiene y no hace pedidos. Los totales los calcula el
// backend al cerrar (FR-043, FR-045) y llegan en results; acá se verifica que se muestran tal cual
// llegan, junto con la diferencia entre comprometidos al inicio y planificados al cierre (US5-3,
// SC-009).
//
// Contrato que fijan estas pruebas para SprintResults:
// - Recibe el sprint (SprintDetail). Si results es null (sprint Planificado o Activo) no muestra
//   nada.
// - En un sprint Cerrado muestra una región con el título "Resultado del sprint" y una lista de datos
//   (dt con el rótulo y dd con el valor) con estos rótulos:
//   "Story Points comprometidos al inicio", "Story Points planificados al cierre",
//   "Diferencia de alcance", "Story Points completados", "Historias planificadas",
//   "Historias completadas", "Factor al cierre", "Cierre real" y "Cerrado por".
// - Los totales se muestran como enteros. La diferencia de alcance es planificados menos
//   comprometidos, con signo: "+16" si se sumó alcance, "-5" si se quitó y "0" si no cambió.
// - El factor se muestra con formato es-AR seguido de "horas por Story Point" (6,5 horas por Story
//   Point), la fecha real de cierre como DD/MM/AAAA y quién cerró por su nombre completo.
// - No ofrece ninguna acción ni ningún aviso de error: es de solo lectura.

const sprintId = "6a1b2c3d-4e5f-4a6b-8c9d-9e0f1a2b3c4d";
const closedAt = "2026-10-10T15:00:00Z";
const closerName = "Bruno Gómez";

// Arma el sprint con el estado, los totales y la fecha de fin prevista indicados. Los datos de
// cierre solo se completan en un sprint Cerrado.
function sprintWith(
  status: SprintStatus,
  results: SprintResultsData | null,
  endDate = "2026-10-14",
): SprintDetail {
  const closed = status === "closed";
  return {
    id: sprintId,
    projectId: "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e",
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate: "2026-10-01",
    endDate,
    startedAt: status === "planned" ? null : "2026-10-01T12:00:00Z",
    closedAt: closed ? closedAt : null,
    createdBy: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    closedBy: closed
      ? { id: "5d2e8b1c-4a3f-4c6d-9e7a-2b1c0d9e8f7a", fullName: closerName }
      : null,
    createdAt: "2026-09-28T12:00:00Z",
    updatedAt: closed ? closedAt : "2026-10-01T12:00:00Z",
    stories: [],
    results,
    hoursPerStoryPoint: closed ? 6.5 : null,
    schedule: closed ? null : { remainingDays: 4, overdueDays: null },
  };
}

// Arma los totales de la instantánea.
function results(
  committed: number,
  planned: number,
  completed: number,
  plannedCount: number,
  completedCount: number,
): SprintResultsData {
  return {
    committedStoryPoints: committed,
    plannedStoryPoints: planned,
    completedStoryPoints: completed,
    plannedStoryCount: plannedCount,
    completedStoryCount: completedCount,
  };
}

// Región del resultado del sprint.
function resultsRegion(): HTMLElement {
  return screen.getByRole("region", { name: "Resultado del sprint" });
}

// Texto del valor que acompaña al rótulo dado en la lista de datos del resultado.
function valueOf(label: string): string | undefined {
  return within(resultsRegion())
    .getByText(label)
    .nextElementSibling?.textContent?.trim();
}

// Verifica los cinco totales de la instantánea y la diferencia de alcance mostrados.
function expectTotals(expected: SprintResultsData, difference: string) {
  expect(valueOf("Story Points comprometidos al inicio")).toBe(
    String(expected.committedStoryPoints),
  );
  expect(valueOf("Story Points planificados al cierre")).toBe(
    String(expected.plannedStoryPoints),
  );
  expect(valueOf("Diferencia de alcance")).toBe(difference);
  expect(valueOf("Story Points completados")).toBe(
    String(expected.completedStoryPoints),
  );
  expect(valueOf("Historias planificadas")).toBe(
    String(expected.plannedStoryCount),
  );
  expect(valueOf("Historias completadas")).toBe(
    String(expected.completedStoryCount),
  );
}

describe("004/US5 - Cerrar un sprint y congelar su resultado", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: con 21 Story Points planificados y 13 completados en cuatro historias, se muestran
  // los totales tal como llegan y el alcance no cambió.
  it("Cierre parcial", () => {
    const snapshot = results(21, 21, 13, 4, 2);
    render(<SprintResults sprint={sprintWith("closed", snapshot)} />);

    expectTotals(snapshot, "0");
  });

  // Caso alternativo: con todas las historias Completadas, los planificados al cierre y los
  // completados coinciden.
  it("Todas completadas", () => {
    const snapshot = results(13, 13, 13, 2, 2);
    render(<SprintResults sprint={sprintWith("closed", snapshot)} />);

    expectTotals(snapshot, "0");
    expect(valueOf("Story Points completados")).toBe(
      valueOf("Story Points planificados al cierre"),
    );
  });

  // Caso alternativo: con 20 comprometidos al inicio y 36 planificados al cierre se muestra la
  // diferencia +16; si durante el sprint se quitó alcance, la diferencia queda negativa (SC-009).
  it("Alcance agregado", () => {
    const added = results(20, 36, 13, 4, 2);
    render(<SprintResults sprint={sprintWith("closed", added)} />);
    expectTotals(added, "+16");
    cleanup();

    const removed = results(20, 15, 10, 3, 2);
    render(<SprintResults sprint={sprintWith("closed", removed)} />);
    expectTotals(removed, "-5");
  });

  // Caso alternativo: el resultado muestra lo congelado en la instantánea, incluido el factor
  // copiado al cerrar y quién cerró, y no ofrece ninguna acción para cambiarlo.
  it("Inmutable tras el cierre", () => {
    render(
      <SprintResults
        sprint={sprintWith("closed", results(21, 21, 13, 4, 2))}
      />,
    );

    expect(valueOf("Factor al cierre")).toContain("6,5 horas por Story Point");
    expect(valueOf("Cerrado por")).toBe(closerName);
    expect(within(resultsRegion()).queryAllByRole("button")).toHaveLength(0);
    expect(within(resultsRegion()).queryAllByRole("textbox")).toHaveLength(0);
  });

  // Caso límite: sin historias completadas, los completados se muestran en cero sin que sea un error.
  it("Sin completadas", () => {
    const snapshot = results(21, 21, 0, 4, 0);
    render(<SprintResults sprint={sprintWith("closed", snapshot)} />);

    expectTotals(snapshot, "0");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un sprint cerrado sin historias muestra la instantánea toda en cero.
  it("Sprint vacío", () => {
    const snapshot = results(0, 0, 0, 0, 0);
    render(<SprintResults sprint={sprintWith("closed", snapshot)} />);

    expectTotals(snapshot, "0");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un sprint cerrado antes de su fecha de fin muestra el momento real de cierre.
  it("Cierre anticipado", () => {
    render(
      <SprintResults
        sprint={sprintWith("closed", results(21, 21, 13, 4, 2), "2026-10-14")}
      />,
    );

    expect(valueOf("Cierre real")).toContain("10/10/2026");
  });

  // Caso límite: un sprint cerrado después de su fecha de fin muestra su resultado sin ninguna
  // penalización ni aviso de error.
  it("Cierre tardío", () => {
    const snapshot = results(21, 21, 13, 4, 2);
    render(
      <SprintResults sprint={sprintWith("closed", snapshot, "2026-10-05")} />,
    );

    expectTotals(snapshot, "0");
    expect(valueOf("Cierre real")).toContain("10/10/2026");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Regla de la interfaz: un sprint Planificado o Activo todavía no tiene resultado, así que no se
  // muestra nada.
  it("no muestra resultado en un sprint sin cerrar", () => {
    for (const status of ["planned", "active"] as const) {
      render(<SprintResults sprint={sprintWith(status, null)} />);
      expect(
        screen.queryByRole("region", { name: "Resultado del sprint" }),
        status,
      ).toBeNull();
      cleanup();
    }
  });
});

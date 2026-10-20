import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { BacklogFilters } from "@/features/backlog/components/backlog-filters";
import { StoryList } from "@/features/backlog/components/story-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type {
  AppliedFilters,
  StoryListItem,
  StoryListResponse,
  StoryPriority,
  StoryStatus,
} from "@/types/story";

// Pruebas de los filtros del backlog (T031). Combinan BacklogFilters con StoryList y el cliente
// HTTP real, como los compondrá la pantalla del backlog; solo se sustituye fetch por un backend de
// prueba que filtra un conjunto fijo de historias y elige el motivo del vacío como el contrato.
//
// Contrato que fijan estas pruebas para BacklogFilters:
// - Es un componente controlado: recibe los filtros elegidos (value) y comunica cada cambio con
//   onChange, entregando los dos filtros completos.
// - Ofrece un grupo (role="group") llamado "Estado" con una casilla por estado (Pendiente,
//   En progreso, Completada) y otro llamado "Prioridad" con una casilla por prioridad (Alta, Media,
//   Baja). Marcar varias casillas de un grupo elige varios valores del mismo filtro (FR-034).

// Sustituye el enrutador de Next.js por si los componentes lo usan: fuera del App Router no hay
// enrutador montado.
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    refresh: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";

const noFilters: AppliedFilters = { status: [], priority: [] };

// Historias del backlog en el orden de FR-032, con historias en los tres estados y las tres
// prioridades.
const highPending: StoryListItem = {
  id: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  status: "pending",
  storyPoints: 5,
  acceptanceCriteriaCount: 2,
  readyToPlan: true,
};
const highInProgress: StoryListItem = {
  id: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "high",
  status: "in_progress",
  storyPoints: 3,
  acceptanceCriteriaCount: 1,
  readyToPlan: true,
};
const mediumCompleted: StoryListItem = {
  id: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  title: "Mostrar el burndown del sprint",
  priority: "medium",
  status: "completed",
  storyPoints: 8,
  acceptanceCriteriaCount: 3,
  readyToPlan: true,
};
const lowPending: StoryListItem = {
  id: "6d7e8f90-1a2b-4c3d-9e4f-5a6b7c8d9e0f",
  title: "Revisar los permisos de los integrantes",
  priority: "low",
  status: "pending",
  storyPoints: null,
  acceptanceCriteriaCount: 0,
  readyToPlan: false,
};
const fullBacklog = [highPending, highInProgress, mediumCompleted, lowPending];

// Backend de prueba del listado: lee los filtros repetidos de la consulta, filtra las historias
// dadas combinando los valores de un filtro con O y los dos filtros con Y, y elige el motivo del
// vacío como lo indica el contrato (FR-034 a FR-036).
function fakeListBackend(stories: StoryListItem[]) {
  return async (input: RequestInfo | URL) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const query = new URL(raw, "http://localhost").searchParams;
    const appliedFilters: AppliedFilters = {
      status: query.getAll("status") as StoryStatus[],
      priority: query.getAll("priority") as StoryPriority[],
    };
    const matches = stories.filter(
      (story) =>
        (appliedFilters.status.length === 0 ||
          appliedFilters.status.includes(story.status)) &&
        (appliedFilters.priority.length === 0 ||
          appliedFilters.priority.includes(story.priority)),
    );
    let emptyReason: StoryListResponse["emptyReason"] = null;
    if (matches.length === 0) {
      emptyReason = stories.length === 0 ? "backlog_empty" : "no_matches";
    }
    const body: StoryListResponse = {
      stories: matches,
      appliedFilters,
      emptyReason,
    };
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
}

// Devuelve los parámetros de consulta de la última llamada a fetch.
function lastQuery(fetchMock: FetchMock): URLSearchParams {
  const calls = fetchMock.mock.calls;
  expect(calls.length).toBeGreaterThan(0);
  const [input] = calls[calls.length - 1];
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost").searchParams;
}

// Compone los filtros y el listado con el estado de los filtros compartido, como la pantalla del
// backlog.
function FilteredBacklog() {
  const [filters, setFilters] = useState<AppliedFilters>(noFilters);
  return (
    <>
      <BacklogFilters value={filters} onChange={setFilters} />
      <StoryList projectId={projectId} filters={filters} />
    </>
  );
}

// Monta los filtros y el listado dentro del proveedor de sesión, como en el layout raíz.
function renderFilteredBacklog() {
  return render(
    <SessionProvider>
      <FilteredBacklog />
    </SessionProvider>,
  );
}

// Localiza la casilla con la etiqueta dada dentro del grupo de filtros indicado.
function checkbox(group: "Estado" | "Prioridad", label: string): HTMLElement {
  const fieldset = screen.getByRole("group", { name: new RegExp(group, "i") });
  return within(fieldset).getByRole("checkbox", { name: label });
}

// Espera a que el listado muestre exactamente las historias dadas, en ese orden.
async function expectShownTitles(titles: string[]) {
  await waitFor(() => {
    const items = screen.queryAllByRole("listitem");
    expect(items.map((item) => item.textContent ?? "")).toHaveLength(
      titles.length,
    );
    titles.forEach((title, index) => {
      expect(items[index].textContent).toContain(title);
    });
  });
}

describe("003/US2 - Ver el backlog ordenado y filtrado", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y el backend de prueba con el backlog
  // completo.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockImplementation(fakeListBackend(fullBacklog));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso alternativo: con un backlog en los tres estados, marcar Pendiente pide ?status=pending y
  // deja solo las historias Pendientes, en el mismo orden de prioridad y fecha (FR-034).
  it("Filtro por estado", async () => {
    renderFilteredBacklog();
    await expectShownTitles(fullBacklog.map((story) => story.title));

    fireEvent.click(checkbox("Estado", "Pendiente"));

    await expectShownTitles([highPending.title, lowPending.title]);
    expect(lastQuery(fetchMock).getAll("status")).toEqual(["pending"]);
    expect(lastQuery(fetchMock).getAll("priority")).toEqual([]);
    expect((checkbox("Estado", "Pendiente") as HTMLInputElement).checked).toBe(
      true,
    );
  });

  // Caso alternativo: marcar la prioridad Alta y el estado Pendiente a la vez envía los dos filtros
  // y deja solo las historias que cumplen ambas condiciones (FR-034).
  it("Filtros combinados", async () => {
    renderFilteredBacklog();
    await expectShownTitles(fullBacklog.map((story) => story.title));

    fireEvent.click(checkbox("Prioridad", "Alta"));
    fireEvent.click(checkbox("Estado", "Pendiente"));

    await expectShownTitles([highPending.title]);
    expect(lastQuery(fetchMock).getAll("status")).toEqual(["pending"]);
    expect(lastQuery(fetchMock).getAll("priority")).toEqual(["high"]);
  });

  // Marcar dos estados elige los dos valores del mismo filtro: se ven las historias Pendientes y En
  // progreso y ninguna Completada (FR-034).
  it("admite varios valores en un mismo filtro", async () => {
    renderFilteredBacklog();
    await expectShownTitles(fullBacklog.map((story) => story.title));

    fireEvent.click(checkbox("Estado", "Pendiente"));
    fireEvent.click(checkbox("Estado", "En progreso"));

    await expectShownTitles([
      highPending.title,
      highInProgress.title,
      lowPending.title,
    ]);
    expect([...lastQuery(fetchMock).getAll("status")].sort()).toEqual([
      "in_progress",
      "pending",
    ]);
  });

  // Desmarcar el único filtro elegido vuelve a pedir el backlog sin filtros y muestra todas las
  // historias.
  it("al quitar el filtro vuelve a mostrar el backlog completo", async () => {
    renderFilteredBacklog();
    await expectShownTitles(fullBacklog.map((story) => story.title));

    fireEvent.click(checkbox("Estado", "Completada"));
    await expectShownTitles([mediumCompleted.title]);
    fireEvent.click(checkbox("Estado", "Completada"));

    await expectShownTitles(fullBacklog.map((story) => story.title));
    expect([...lastQuery(fetchMock).keys()]).toHaveLength(0);
  });

  // Caso límite: en un backlog que solo tiene historias Bajas, filtrar por Alta muestra que ninguna
  // historia coincide con los filtros, sin presentarlo como error y sin el mensaje de backlog vacío
  // (FR-036).
  it("Filtro sin coincidencias", async () => {
    fetchMock.mockImplementation(fakeListBackend([lowPending]));
    renderFilteredBacklog();
    await expectShownTitles([lowPending.title]);

    fireEvent.click(checkbox("Prioridad", "Alta"));

    expect(await screen.findByText(/ninguna historia coincide/i)).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/todavía no tiene historias/i)).toBeNull();
  });

  // Caso de error, prevenido en la interfaz: los filtros ofrecen solo los estados y las
  // prioridades admitidos, con sus etiquetas en español, de modo que no se puede elegir un valor
  // que no existe (FR-037). El rechazo del backend ante un valor inválido se prueba en
  // story-list.test.tsx.
  it("Filtro inválido", () => {
    render(<BacklogFilters value={noFilters} onChange={vi.fn()} />);

    const statusGroup = screen.getByRole("group", { name: /estado/i });
    const priorityGroup = screen.getByRole("group", { name: /prioridad/i });

    expect(within(statusGroup).getAllByRole("checkbox")).toHaveLength(3);
    expect(within(priorityGroup).getAllByRole("checkbox")).toHaveLength(3);
    for (const label of ["Pendiente", "En progreso", "Completada"]) {
      expect(
        within(statusGroup).getByRole("checkbox", { name: label }),
      ).toBeTruthy();
    }
    for (const label of ["Alta", "Media", "Baja"]) {
      expect(
        within(priorityGroup).getByRole("checkbox", { name: label }),
      ).toBeTruthy();
    }
    for (const box of screen.getAllByRole<HTMLInputElement>("checkbox")) {
      expect(box.checked).toBe(false);
    }
  });

  // Los filtros son controlados: muestran marcados los valores recibidos y, al marcar otro,
  // entregan los dos filtros completos sin perder lo ya elegido.
  it("refleja los filtros recibidos y comunica el cambio conservando los demás", () => {
    const onChange = vi.fn<(next: AppliedFilters) => void>();
    render(
      <BacklogFilters
        value={{ status: ["completed"], priority: ["medium"] }}
        onChange={onChange}
      />,
    );

    expect((checkbox("Estado", "Completada") as HTMLInputElement).checked).toBe(
      true,
    );
    expect((checkbox("Prioridad", "Media") as HTMLInputElement).checked).toBe(
      true,
    );

    fireEvent.click(checkbox("Prioridad", "Alta"));

    expect(onChange).toHaveBeenCalledTimes(1);
    const [next] = onChange.mock.calls[0];
    expect(next.status).toEqual(["completed"]);
    expect([...next.priority].sort()).toEqual(["high", "medium"]);
  });
});

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
import { StoryEffortSection } from "@/features/effort/components/story-effort-section";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  EffortComparisonRow,
  EffortEntry,
  StoryEffort,
} from "@/types/effort";

// Pruebas de la comparación entre estimado y real dentro de StoryEffortSection (T079). Atraviesan la
// sección, las llamadas de api.ts y el cliente HTTP real; solo se sustituye fetch por un backend
// falso con estado, que responde como lo indica contracts/openapi.yaml.
//
// Contrato que fijan estas pruebas para StoryEffortSection:
// - Pide el esfuerzo y la comparación de la historia (GET .../effort y GET .../effort-comparison) y
//   muestra la tarjeta "Comparación estimado contra real" junto con el desglose y la lista.
// - Después de registrar, corregir o eliminar vuelve a pedir la comparación y muestra las horas
//   reales actualizadas (FR-053).
// - Si la consulta de comparación falla muestra su mensaje con role="alert" y no muestra la tarjeta.

// Sustituye el enrutador de Next.js: la guardia de sesión lo usa.
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
const storyId = "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f";
const effortPath = `/api/v1/stories/${storyId}/effort`;
const comparisonPath = `/api/v1/stories/${storyId}/effort-comparison`;
const ana = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const storyRef = { id: storyId, title: "Alta de clientes" };
const cardName = "Comparación estimado contra real";

// Registro propio de 4 horas, del que parte cada prueba.
const baseEntry: EffortEntry = {
  id: "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a",
  entryDate: "2026-10-20",
  activity: "Desarrollo del formulario",
  hours: 4,
  author: ana,
  story: storyRef,
  sprint: null,
  definitive: false,
  createdAt: "2026-10-20T15:00:00Z",
  updatedAt: "2026-10-20T15:00:00Z",
};

// Backend falso con estado: guarda los registros y arma el esfuerzo y la comparación a partir de
// ellos, con una historia de 5 SP a 6 horas por punto (30 horas estimadas).
type FakeBackend = {
  entries: EffortEntry[];
  comparisonError: { status: number; body: ErrorResponse } | null;
};

function newBackend(): FakeBackend {
  return { entries: [baseEntry], comparisonError: null };
}

// Arma el esfuerzo de la historia con los registros vigentes.
function effortOf(backend: FakeBackend): StoryEffort {
  const total = backend.entries.reduce((sum, item) => sum + item.hours, 0);
  return {
    storyId,
    totalHours: total,
    byMember: backend.entries.length === 0 ? [] : [{ user: ana, hours: total }],
    entries: backend.entries,
  };
}

// Arma la comparación de la historia con las horas reales vigentes.
function comparisonOf(backend: FakeBackend): EffortComparisonRow {
  const actual = effortOf(backend).totalHours;
  return {
    storyId,
    title: storyRef.title,
    storyPoints: 5,
    estimatedHours: 30,
    actualHours: actual,
    deviationHours: actual - 30,
    deviationPercent: ((actual - 30) / 30) * 100,
  };
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta de una llamada a fetch.
function pathOf(input: RequestInfo | URL): string {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost").pathname;
}

// Responde cada ruta del contrato y aplica al estado las altas, correcciones y eliminaciones.
function respondWith(backend: FakeBackend) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = pathOf(input);
    const method = init?.method ?? "GET";
    if (path === "/api/v1/users/me") {
      return jsonResponse(200, { ...ana, email: "ana@example.com" });
    }
    if (path === effortPath) {
      return jsonResponse(200, effortOf(backend));
    }
    if (path === comparisonPath) {
      if (backend.comparisonError !== null) {
        return jsonResponse(
          backend.comparisonError.status,
          backend.comparisonError.body,
        );
      }
      return jsonResponse(200, comparisonOf(backend));
    }
    if (method === "POST" && path === `${effortPath}-entries`) {
      const body = JSON.parse(String(init?.body)) as Partial<EffortEntry>;
      const created: EffortEntry = {
        ...baseEntry,
        id: "9b8a7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d",
        ...body,
      };
      backend.entries = [created, ...backend.entries];
      return jsonResponse(201, created);
    }
    if (method === "PATCH") {
      const body = JSON.parse(String(init?.body)) as Partial<EffortEntry>;
      const updated = { ...baseEntry, ...body };
      backend.entries = backend.entries.map((item) =>
        item.id === baseEntry.id ? updated : item,
      );
      return jsonResponse(200, updated);
    }
    if (method === "DELETE") {
      backend.entries = backend.entries.filter(
        (item) => item.id !== baseEntry.id,
      );
      return new Response(null, { status: 204 });
    }
    return jsonResponse(404, { error: "Ruta no encontrada." });
  };
}

// Monta la sección dentro del proveedor de sesión, como en el layout raíz.
function renderSection() {
  render(
    <SessionProvider>
      <StoryEffortSection storyId={storyId} />
    </SessionProvider>,
  );
}

// Cuenta cuántas veces se pidió una ruta con GET.
function countGets(fetchMock: FetchMock, path: string): number {
  return fetchMock.mock.calls.filter(
    ([input, init]) =>
      pathOf(input) === path && (init?.method ?? "GET") === "GET",
  ).length;
}

// Devuelve la tarjeta de comparación una vez cargada.
function findCard(): Promise<HTMLElement> {
  return screen.findByLabelText(cardName);
}

describe("006/US5 - Comparar el esfuerzo estimado con el real", () => {
  let fetchMock: FetchMock;
  let backend: FakeBackend;

  // Cada prueba arranca con una sesión válida en la pestaña y un backend con un registro de 4 horas.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    backend = newBackend();
    fetchMock = vi.fn<typeof fetch>(respondWith(backend));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("Caso normal — la sección carga la tarjeta de comparación junto con el esfuerzo", async () => {
    renderSection();

    const card = await findCard();
    expect(within(card).getByText("5 SP")).toBeTruthy();
    expect(within(card).getByText("30 h")).toBeTruthy();
    expect(within(card).getByText("4 h")).toBeTruthy();
    expect(within(card).getByText("−26 h")).toBeTruthy();
    expect(
      screen.getByRole("list", { name: "Registros de esfuerzo" }),
    ).toBeTruthy();
    expect(countGets(fetchMock, effortPath)).toBe(1);
    expect(countGets(fetchMock, comparisonPath)).toBe(1);
  });

  it("Caso normal — después de registrar esfuerzo la comparación vuelve a pedirse y se actualiza", async () => {
    renderSection();
    await findCard();

    fireEvent.change(screen.getByLabelText(/^actividad/i), {
      target: { value: "Pruebas de aceptación" },
    });
    fireEvent.change(screen.getByLabelText(/^horas/i), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Registrar esfuerzo" }));

    await waitFor(() => expect(countGets(fetchMock, comparisonPath)).toBe(2));
    await waitFor(() =>
      expect(
        within(screen.getByLabelText(cardName)).getByText("6 h"),
      ).toBeTruthy(),
    );
    expect(countGets(fetchMock, effortPath)).toBe(2);
  });

  it("Caso normal — después de corregir un registro la comparación vuelve a pedirse y se actualiza", async () => {
    renderSection();
    await findCard();
    const item = await screen.findByRole("button", { name: "Editar" });

    fireEvent.click(item);
    // La edición en línea vive dentro del elemento de la lista; el formulario de alta tiene otro "Horas".
    const row = screen
      .getByRole("button", { name: "Guardar cambios" })
      .closest("li") as HTMLElement;
    fireEvent.change(within(row).getByLabelText(/^horas/i), {
      target: { value: "6" },
    });
    fireEvent.click(
      within(row).getByRole("button", { name: "Guardar cambios" }),
    );

    await waitFor(() => expect(countGets(fetchMock, comparisonPath)).toBe(2));
    await waitFor(() =>
      expect(
        within(screen.getByLabelText(cardName)).getByText("6 h"),
      ).toBeTruthy(),
    );
  });

  it("Caso normal — después de eliminar un registro la comparación vuelve a pedirse y se actualiza", async () => {
    renderSection();
    await findCard();

    fireEvent.click(await screen.findByRole("button", { name: "Eliminar" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Confirmar eliminación" }),
    );

    await waitFor(() => expect(countGets(fetchMock, comparisonPath)).toBe(2));
    await waitFor(() =>
      expect(
        within(screen.getByLabelText(cardName)).getByText("0 h"),
      ).toBeTruthy(),
    );
  });

  it("Caso de error — falla la consulta de la comparación de la historia", async () => {
    backend.comparisonError = {
      status: 404,
      body: { error: "Historia no encontrada." },
    };
    renderSection();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Historia no encontrada.");
    expect(screen.queryByLabelText(cardName)).toBeNull();
    expect(
      screen.queryByRole("list", { name: "Registros de esfuerzo" }),
    ).toBeNull();
  });
});

import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { ProjectList } from "@/features/projects/components/project-list";
import {
  readToken,
  wasSessionRejected,
  writeToken,
} from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type { ProjectListItem, ProjectListResponse } from "@/types/project";

// Pruebas del listado de proyectos (T034). Atraviesan el componente y el cliente HTTP real; solo se
// sustituye fetch, que es la frontera con el backend. El filtrado por membresía y el orden los
// resuelve el backend (T033): acá se verifica que la vista muestra lo que recibe, como lo recibe.
//
// Contrato que fijan estas pruebas para el listado:
// - El componente no recibe props: pide GET /api/v1/projects con la sesión de la pestaña.
// - Cada proyecto se muestra como un elemento de lista (role="listitem"), en el orden recibido, con
//   su nombre, la etiqueta en español de su estado, la fecha de inicio y la de finalización
//   prevista (en formato AAAA-MM-DD o DD/MM/AAAA), la cantidad de integrantes ("1 integrante",
//   "3 integrantes") y la palabra "Propietario" solo cuando quien consulta lo es.
// - Con la lista vacía se muestra el mensaje que envía el backend y no un error (role="alert").

// Sustituye el enrutador de Next.js por si el listado lo usa: fuera del App Router no hay
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
const listPath = "/api/v1/projects";
const emptyMessage = "Todavía no participás en ningún proyecto.";

// Proyectos de Ana: propietaria de dos e integrante de un tercero, ya ordenados por el backend por
// fecha de inicio descendente y, a igual fecha, por nombre ascendente.
const ownedPortal: ProjectListItem = {
  id: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  name: "Portal de Clientes",
  status: "in_progress",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-20",
  memberCount: 3,
  isOwner: true,
};
const ownedBilling: ProjectListItem = {
  id: "1e5d7c3b-2a4f-4b8e-9c60-7d3f2a1b0c94",
  name: "Facturación",
  status: "finished",
  startDate: "2026-09-15",
  plannedEndDate: "2026-10-10",
  memberCount: 2,
  isOwner: true,
};
const memberMobileApp: ProjectListItem = {
  id: "4c7a8e21-9b40-4d16-a5f3-2e8b6c1d0a97",
  name: "App Móvil",
  status: "planned",
  startDate: "2026-09-15",
  plannedEndDate: "2026-11-30",
  memberCount: 5,
  isOwner: false,
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: ProjectListResponse | ErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Hace que fetch responda siempre con el estado y el cuerpo indicados; cada llamada recibe una
// respuesta nueva porque el cuerpo de una Response solo se puede leer una vez.
function respondWith(
  fetchMock: FetchMock,
  status: number,
  body: ProjectListResponse | ErrorResponse,
) {
  fetchMock.mockImplementation(async () => jsonResponse(status, body));
}

// Extrae la URL, el método y la cabecera Authorization de la única llamada a fetch.
function sentRequest(fetchMock: FetchMock) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [input, init] = fetchMock.mock.calls[0];
  if (input instanceof Request) {
    return {
      url: input.url,
      method: input.method,
      authorization: input.headers.get("Authorization"),
    };
  }
  return {
    url: input instanceof URL ? input.href : input,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Monta el listado dentro del proveedor de sesión, como en el layout raíz.
function renderProjectList() {
  return render(
    <SessionProvider>
      <ProjectList />
    </SessionProvider>,
  );
}

// Espera a que se muestren los proyectos y devuelve el texto de cada elemento de la lista.
async function shownItems(): Promise<string[]> {
  const items = await screen.findAllByRole("listitem");
  return items.map((item) => item.textContent ?? "");
}

// Indica si el texto muestra la fecha AAAA-MM-DD en ese formato o como DD/MM/AAAA.
function showsDate(text: string, isoDate: string): boolean {
  const [year, month, day] = isoDate.split("-");
  return text.includes(isoDate) || text.includes(`${day}/${month}/${year}`);
}

describe("002/US2 - Ver mis proyectos y quedar aislado de los ajenos", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que devuelve los tres
  // proyectos de Ana.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    respondWith(fetchMock, 200, {
      projects: [ownedPortal, ownedBilling, memberMobileApp],
      message: null,
    });
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el listado pide GET /api/v1/projects con la sesión de la pestaña y muestra los
  // tres proyectos en el orden recibido, cada uno con nombre, estado, fechas, cantidad de
  // integrantes y la marca de propietario solo en los propios (FR-032, FR-035).
  it("Listado completo", async () => {
    renderProjectList();

    const items = await shownItems();
    expect(items).toHaveLength(3);
    expect(items[0]).toContain("Portal de Clientes");
    expect(items[1]).toContain("Facturación");
    expect(items[2]).toContain("App Móvil");

    expect(items[0]).toContain("En curso");
    expect(items[1]).toContain("Finalizado");
    expect(items[2]).toContain("Planificado");

    const expected = [ownedPortal, ownedBilling, memberMobileApp];
    for (const [index, project] of expected.entries()) {
      expect(showsDate(items[index], project.startDate), project.name).toBe(
        true,
      );
      expect(
        showsDate(items[index], project.plannedEndDate),
        project.name,
      ).toBe(true);
      expect(items[index], project.name).toMatch(
        new RegExp(`\\b${project.memberCount} integrantes\\b`),
      );
    }

    expect(items[0]).toMatch(/propietario/i);
    expect(items[1]).toMatch(/propietario/i);
    expect(items[2]).not.toMatch(/propietario/i);

    const request = sentRequest(fetchMock);
    expect(request.url.endsWith(listPath)).toBe(true);
    expect(request.method).toBe("GET");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  });

  // Caso alternativo: sin proyectos, el listado muestra el mensaje del backend como estado vacío,
  // sin ningún elemento de lista y sin presentarlo como un error (FR-034).
  it("Listado vacío", async () => {
    respondWith(fetchMock, 200, { projects: [], message: emptyMessage });
    renderProjectList();

    expect(await screen.findByText(emptyMessage)).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: el proyecto del que la persona fue quitada ya no llega en la respuesta, y el
  // listado muestra solo los que recibe, sin conservar ni agregar ninguno (FR-033).
  it("Integrante quitado no ve el proyecto", async () => {
    respondWith(fetchMock, 200, {
      projects: [ownedPortal, ownedBilling],
      message: null,
    });
    renderProjectList();

    const items = await shownItems();
    expect(items).toHaveLength(2);
    expect(document.body.textContent).not.toContain("App Móvil");
  });

  // Caso límite: un proyecto cuyo único integrante es su propietario se muestra con
  // "1 integrante", en singular.
  it("Proyecto con un integrante", async () => {
    respondWith(fetchMock, 200, {
      projects: [{ ...ownedPortal, memberCount: 1 }],
      message: null,
    });
    renderProjectList();

    const [item] = await shownItems();
    expect(item).toMatch(/\b1 integrante\b/);
    expect(item).not.toMatch(/\b1 integrantes\b/);
    expect(item).toMatch(/propietario/i);
  });

  // Caso de error: si el backend rechaza la sesión con 401, la sesión de la pestaña se descarta
  // para que la guardia pida iniciar sesión, y no se muestra ningún proyecto ni el estado vacío.
  it("Sin sesión", async () => {
    respondWith(fetchMock, 401, { error: "Se requiere iniciar sesión." });
    renderProjectList();

    await waitFor(() => expect(readToken()).toBeNull());
    expect(wasSessionRejected()).toBe(true);
    expect(sentRequest(fetchMock).authorization).toBe(`Bearer ${sessionToken}`);
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByText(emptyMessage)).toBeNull();
  });
});

import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render } from "@testing-library/react";
import { getProjectDetail } from "@/features/projects/api";
import { ProjectStatusCard } from "@/features/projects/components/project-status-card";
import {
  readToken,
  wasSessionRejected,
  writeToken,
} from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { ProjectDetail } from "@/types/project";

// Pruebas de la vista de estado del proyecto (T057). La tarjeta es de presentación: recibe el
// ProjectDetail que la pantalla (T061) pide una sola vez con getProjectDetail y lo comparte con la
// lista de integrantes (SC-003). Por eso los escenarios de datos se verifican montando la tarjeta,
// y los de error (proyecto ajeno y sin sesión) atravesando la llamada real y el cliente HTTP; solo
// se sustituye fetch, que es la frontera con el backend. El cálculo de las horas y el armado de la
// vista los resuelve el backend (T055 y T056): acá se verifica qué muestra la vista.
//
// Contrato que fijan estas pruebas:
// - ProjectStatusCard recibe project (ProjectDetail) y muestra el nombre, la descripción, la
//   etiqueta en español del estado, la fecha de inicio, la de finalización prevista y, si existe,
//   la real (AAAA-MM-DD o DD/MM/AAAA), el factor ("6,5" o "6.5") seguido de "horas por Story
//   Point", el propietario con la palabra "Propietario", el nombre del sprint activo, el total de
//   historias del backlog ("N historias"), una lista "Historias por estado" con un elemento por
//   estado recibido, y "N Story Points" y "N horas" estimadas.
// - Sin sprint activo muestra "No hay sprint activo" y no lo presenta como error (role="alert").
// - Con el proyecto de solo lectura (readOnly) muestra "solo lectura"; si no, no lo muestra.
// - getProjectDetail(projectId) pide GET /api/v1/projects/{projectId} con la sesión de la pestaña
//   y devuelve ApiResult<ProjectDetail>; ante un 401 el cliente HTTP descarta la sesión (FR-016).

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";

// Proyecto En curso de Ana con tres integrantes, sprint activo y backlog con historias en
// distintos estados, como el ejemplo enCurso del contrato.
const inProgressProject: ProjectDetail = {
  id: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  name: "Portal de Clientes",
  description: "Rediseño del portal de autogestión.",
  status: "in_progress",
  startDate: "2026-10-01",
  plannedEndDate: "2026-12-20",
  actualEndDate: null,
  hoursPerStoryPoint: 6.5,
  readOnly: false,
  owner: {
    id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
    fullName: "Ana Pérez",
  },
  members: [
    {
      userId: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
      email: "ana@example.com",
      isOwner: true,
      joinedAt: "2026-10-01T12:00:00Z",
    },
    {
      userId: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
      fullName: "Bruno Gómez",
      email: "bruno@example.com",
      isOwner: false,
      joinedAt: "2026-10-03T09:15:00Z",
    },
    {
      userId: "5a9e3c71-4f28-4b06-8d1e-6c2b7f0a9e13",
      fullName: "Carla Ruiz",
      email: "carla@example.com",
      isOwner: false,
      joinedAt: "2026-10-05T12:00:00Z",
    },
  ],
  hasActiveSprint: true,
  activeSprint: {
    id: "e1c4a7b9-2d58-4036-9f1e-7b5c8a2d4e03",
    name: "Sprint 4",
  },
  backlog: {
    countsByState: { pending: 5, in_progress: 2, completed: 7 },
    estimatedStoryPoints: 16,
    estimatedHours: 104,
  },
};

// Proyecto recién creado, sin backlog ni sprints: el backend envía countsByState vacío y los
// totales en cero (FR-043, FR-044), como el ejemplo reciénCreado del contrato.
const newProject: ProjectDetail = {
  ...inProgressProject,
  id: "4c7a8e21-9b40-4d16-a5f3-2e8b6c1d0a97",
  name: "App Móvil",
  description: null,
  status: "planned",
  startDate: "2026-09-15",
  plannedEndDate: "2026-11-30",
  members: [inProgressProject.members[0]],
  hasActiveSprint: false,
  activeSprint: null,
  backlog: { countsByState: {}, estimatedStoryPoints: 0, estimatedHours: 0 },
};

// Monta la tarjeta y devuelve su texto completo.
function renderCardText(project: ProjectDetail): string {
  const { container } = render(<ProjectStatusCard project={project} />);
  return container.textContent ?? "";
}

// Indica si el texto muestra la fecha AAAA-MM-DD en ese formato o como DD/MM/AAAA.
function showsDate(text: string, isoDate: string): boolean {
  const [year, month, day] = isoDate.split("-");
  return text.includes(isoDate) || text.includes(`${day}/${month}/${year}`);
}

// Hace que fetch responda siempre con el estado y el cuerpo indicados.
function respondWith(
  fetchMock: FetchMock,
  status: number,
  body: ProjectDetail | ErrorResponse,
) {
  fetchMock.mockImplementation(
    async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
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

describe("002/US4 - Consultar el estado del proyecto", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: la tarjeta muestra en una sola vista los datos generales, el estado, las fechas,
  // el factor, el propietario, el sprint activo, las historias por estado y los totales (FR-042).
  it("Estado completo", () => {
    const text = renderCardText(inProgressProject);

    expect(text).toContain("Portal de Clientes");
    expect(text).toContain("Rediseño del portal de autogestión.");
    expect(text).toContain("En curso");
    expect(showsDate(text, "2026-10-01")).toBe(true);
    expect(showsDate(text, "2026-12-20")).toBe(true);
    expect(text).toMatch(/6[,.]5 horas por Story Point/i);
    expect(text).toMatch(/propietari[oa]/i);
    expect(text).toContain("Ana Pérez");
    expect(text).toContain("Sprint 4");
    expect(text).not.toMatch(/no hay sprint activo/i);
    expect(text).not.toMatch(/solo lectura/i);

    const byState = document.querySelector(
      '[aria-label="Historias por estado"]',
    );
    expect(byState).not.toBeNull();
    const items = Array.from(byState?.querySelectorAll("li") ?? []).map(
      (item) => item.textContent ?? "",
    );
    expect(items).toHaveLength(3);
    expect(items.some((item) => /\b5\b/.test(item))).toBe(true);
    expect(items.some((item) => /\b2\b/.test(item))).toBe(true);
    expect(items.some((item) => /\b7\b/.test(item))).toBe(true);

    expect(text).toMatch(/\b14 historias\b/);
    expect(text).toMatch(/\b16 Story Points\b/);
    expect(text).toMatch(/\b104 horas\b/);
  });

  // Caso alternativo: un proyecto Finalizado muestra además la fecha de finalización real y la
  // indicación de solo lectura.
  it("Proyecto finalizado", () => {
    const text = renderCardText({
      ...inProgressProject,
      status: "finished",
      actualEndDate: "2026-12-18",
      readOnly: true,
      hasActiveSprint: false,
      activeSprint: null,
    });

    expect(text).toContain("Finalizado");
    expect(showsDate(text, "2026-12-18")).toBe(true);
    expect(showsDate(text, "2026-12-20")).toBe(true);
    expect(text).toMatch(/solo lectura/i);
  });

  // Caso alternativo: sin sprint activo la vista lo indica de forma explícita y no como un error
  // (FR-043).
  it("Sin sprint activo", () => {
    const text = renderCardText({
      ...inProgressProject,
      hasActiveSprint: false,
      activeSprint: null,
    });

    expect(text).toMatch(/no hay sprint activo/i);
    expect(text).not.toContain("Sprint 4");
    expect(document.querySelector('[role="alert"]')).toBeNull();
  });

  // Caso límite: un proyecto recién creado muestra los contadores, el total de Story Points y las
  // horas en cero, y que no hay sprint activo (FR-043, FR-044).
  it("Proyecto recién creado", () => {
    const text = renderCardText(newProject);

    expect(text).toContain("App Móvil");
    expect(text).toContain("Planificado");
    expect(text).toMatch(/\b0 historias\b/);
    expect(text).toMatch(/\b0 Story Points\b/);
    expect(text).toMatch(/\b0 horas\b/);
    expect(text).toMatch(/no hay sprint activo/i);
    expect(document.querySelector('[role="alert"]')).toBeNull();
  });

  // Caso límite: con factor 6,5 y historias de 3, 5 y 8 Story Points más una sin estimar, la
  // vista muestra los totales que calcula el backend, 16 Story Points y 104 horas (FR-045).
  it("Horas estimadas", () => {
    const text = renderCardText({
      ...inProgressProject,
      backlog: {
        countsByState: { pending: 4 },
        estimatedStoryPoints: 16,
        estimatedHours: 104,
      },
    });

    expect(text).toMatch(/\b4 historias\b/);
    expect(text).toMatch(/\b16 Story Points\b/);
    expect(text).toMatch(/\b104 horas\b/);
  });

  // Caso de error: la consulta de un proyecto del que no se es integrante recibe el 404 de
  // proyecto inexistente y devuelve solo el error, sin ningún dato del proyecto (FR-049).
  it("No integrante es inexistente", async () => {
    const message = "El proyecto no existe.";
    respondWith(fetchMock, 404, { error: message });

    const result = await getProjectDetail(inProgressProject.id);

    expect(result).toEqual({
      ok: false,
      error: { status: 404, message, fields: [] },
    });
    const request = sentRequest(fetchMock);
    expect(
      request.url.endsWith(`/api/v1/projects/${inProgressProject.id}`),
    ).toBe(true);
    expect(request.method).toBe("GET");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  });

  // Caso de error: si el backend rechaza la sesión con 401, la consulta falla y la sesión de la
  // pestaña se descarta para que la guardia pida iniciar sesión (FR-016).
  it("Sin sesión", async () => {
    respondWith(fetchMock, 401, { error: "Se requiere iniciar sesión." });

    const result = await getProjectDetail(inProgressProject.id);

    expect(result.ok).toBe(false);
    expect(readToken()).toBeNull();
    expect(wasSessionRejected()).toBe(true);
    expect(sentRequest(fetchMock).authorization).toBe(`Bearer ${sessionToken}`);
  });
});

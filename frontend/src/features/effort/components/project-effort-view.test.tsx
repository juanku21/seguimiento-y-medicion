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
import { ProjectEffortView } from "@/features/effort/components/project-effort-view";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type {
  EffortEntry,
  ProjectEffort,
  StoryRef,
  UserRef,
} from "@/types/effort";
import type { ProjectDetail } from "@/types/project";
import type { SprintListItem, SprintListResponse } from "@/types/sprint";
import type { StoryListResponse } from "@/types/story";

// Pruebas de la vista del esfuerzo del proyecto (T057). Atraviesan la vista, los filtros, las
// llamadas de api.ts y el cliente HTTP real; solo se sustituye fetch por un backend de prueba que
// responde el estado del proyecto (002), su backlog (003), sus sprints (004) y su esfuerzo filtrado
// como lo indica contracts/openapi.yaml (listProjectEffort). El filtrado, el total y el motivo del
// vacío los resuelve el backend: acá se verifica qué consulta arma la vista y qué muestra con cada
// respuesta.
//
// Contrato que fijan estas pruebas para ProjectEffortView (T062):
// - Recibe projectId y se monta dentro de SessionProvider, con la sesión de la pestaña.
// - Pide GET /api/v1/projects/{projectId}/effort-entries; cada filtro elegido en EffortFilters
//   viaja como parámetro (userId, storyId, sprintId, from, to) y los filtros sin aplicar no viajan.
//   "Sin sprint" viaja como sprintId=none. Sin filtros, la consulta no lleva parámetros.
// - El selector "Integrante" ofrece, sin repetir, a los integrantes vigentes (members del estado del
//   proyecto, 002) y a los autores de los registros del listado sin filtros, aunque ya no sean
//   integrantes (FR-040, research.md sección 14). Los selectores "Historia" y "Sprint" ofrecen al
//   menos las historias y los sprints de los registros del proyecto.
// - Los registros se muestran en una lista con nombre accesible "Registros de esfuerzo del
//   proyecto", un elemento por registro en el orden recibido, con la fecha en DD/MM/AAAA, el autor,
//   el título de la historia, la actividad, las horas con formatHours y el sprint, o "Sin sprint".
// - El total del conjunto se muestra como "Total: <horas>" con formatHours (FR-041), también en 0.
// - Con emptyReason no_effort muestra "Todavía no hay esfuerzo registrado en el proyecto." y con
//   no_matches "Ningún registro coincide con los filtros."; ninguno de los dos es un error, así que
//   no usan role="alert" (FR-042).
// - Un rechazo 400 de los filtros se muestra con role="alert" e incluye el mensaje del campo.

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador montado y la guardia de
// sesión lo usa.
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

const invalidRangeMessage =
  "La fecha de fin del rango no puede ser anterior a la de inicio.";
const noEffortMessage = "Todavía no hay esfuerzo registrado en el proyecto.";
const noMatchesMessage = "Ningún registro coincide con los filtros.";

// Integrantes vigentes (Ana, Bruno y Diego, que todavía no cargó horas) y una persona quitada del
// proyecto que dejó registros (Carla).
const ana: UserRef = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const bruno: UserRef = {
  id: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9",
  fullName: "Bruno Díaz",
};
const diego: UserRef = {
  id: "4c3b2a19-0f8e-4d7c-a6b5-948372615a0b",
  fullName: "Diego Sosa",
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

const sprintOne = {
  id: "7f6e5d4c-3b2a-4190-8f7e-6d5c4b3a2910",
  name: "Sprint 1",
  status: "closed",
} as const;
const sprintTwo = {
  id: "8a7b6c5d-4e3f-4a1b-9c8d-7e6f5a4b3c2d",
  name: "Sprint 2",
  status: "active",
} as const;

// Arma un registro del proyecto con los datos que importan en cada prueba.
function entry(
  id: string,
  fields: Pick<
    EffortEntry,
    "entryDate" | "activity" | "hours" | "author" | "story" | "sprint"
  >,
): EffortEntry {
  return {
    id,
    definitive: false,
    createdAt: `${fields.entryDate}T15:00:00Z`,
    updatedAt: `${fields.entryDate}T15:00:00Z`,
    ...fields,
  };
}

// Registros de tres personas en dos sprints y uno sin sprint, en el orden de FR-045. Total: 11,5 h.
const projectEntries: EffortEntry[] = [
  entry("e1", {
    entryDate: "2026-10-22",
    activity: "Diseño de la pantalla",
    hours: 3,
    author: ana,
    story: clientsStory,
    sprint: sprintTwo,
  }),
  entry("e2", {
    entryDate: "2026-10-20",
    activity: "Revisión de código",
    hours: 4.5,
    author: bruno,
    story: velocityStory,
    sprint: sprintTwo,
  }),
  entry("e3", {
    entryDate: "2026-10-12",
    activity: "Maquetado",
    hours: 2,
    author: carla,
    story: clientsStory,
    sprint: sprintOne,
  }),
  entry("e4", {
    entryDate: "2026-10-05",
    activity: "Relevamiento inicial",
    hours: 2,
    author: ana,
    story: velocityStory,
    sprint: null,
  }),
];

// Estado del proyecto (002) con los integrantes vigentes; Carla ya no figura.
const projectDetail: ProjectDetail = {
  id: projectId,
  name: "Software Metrics",
  description: null,
  status: "in_progress",
  startDate: "2026-09-01",
  plannedEndDate: "2026-12-15",
  actualEndDate: null,
  hoursPerStoryPoint: 6,
  readOnly: false,
  owner: ana,
  members: [ana, bruno, diego].map((member) => ({
    userId: member.id,
    fullName: member.fullName,
    isOwner: member.id === ana.id,
    joinedAt: "2026-09-01T12:00:00Z",
  })),
  hasActiveSprint: true,
  activeSprint: { id: sprintTwo.id, name: sprintTwo.name },
  backlog: {
    countsByState: { pending: 0, in_progress: 2, completed: 0 },
    estimatedStoryPoints: 8,
    estimatedHours: 48,
  },
};

// Backlog del proyecto (003) con las dos historias.
const storyList: StoryListResponse = {
  stories: [clientsStory, velocityStory].map((story) => ({
    id: story.id,
    title: story.title,
    priority: "high",
    status: "in_progress",
    storyPoints: 5,
    acceptanceCriteriaCount: 1,
    readyToPlan: true,
  })),
  appliedFilters: { status: [], priority: [] },
  emptyReason: null,
};

// Sprints del proyecto (004).
function sprintItem(
  sprint: typeof sprintOne | typeof sprintTwo,
  startDate: string,
  endDate: string,
): SprintListItem {
  return {
    id: sprint.id,
    name: sprint.name,
    goal: "Objetivo",
    status: sprint.status,
    startDate,
    endDate,
    startedAt: `${startDate}T12:00:00Z`,
    closedAt: sprint.status === "closed" ? `${endDate}T20:00:00Z` : null,
    storyCount: 2,
    results: null,
  };
}
const sprintList: SprintListResponse = {
  sprints: [
    sprintItem(sprintTwo, "2026-10-15", "2026-10-29"),
    sprintItem(sprintOne, "2026-10-01", "2026-10-14"),
  ],
  appliedFilters: { status: [] },
  emptyReason: null,
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body:
    | ProjectDetail
    | StoryListResponse
    | SprintListResponse
    | ProjectEffort
    | ErrorResponse
    | ValidationErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la URL completa de una llamada a fetch.
function urlOf(input: RequestInfo | URL): URL {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost");
}

const effortPath = `/api/v1/projects/${projectId}/effort-entries`;

// Esfuerzo del proyecto según el contrato: filtros conjuntivos, sprintId=none para los registros
// sin sprint, rango inclusivo, 400 si el fin es anterior al inicio, total del conjunto y motivo del
// vacío (no_effort sin ningún registro, no_matches si los filtros no encuentran ninguno).
function effortResponse(entries: EffortEntry[], query: URLSearchParams) {
  const userId = query.get("userId");
  const storyId = query.get("storyId");
  const sprintId = query.get("sprintId");
  const from = query.get("from");
  const to = query.get("to");
  if (from !== null && to !== null && to < from) {
    return jsonResponse(400, {
      error: "Hay campos inválidos en la solicitud.",
      fields: [{ field: "to", message: invalidRangeMessage }],
    });
  }
  const matches = entries.filter(
    (item) =>
      (userId === null || item.author.id === userId) &&
      (storyId === null || item.story.id === storyId) &&
      (sprintId === null ||
        (sprintId === "none"
          ? item.sprint === null
          : item.sprint?.id === sprintId)) &&
      (from === null || item.entryDate >= from) &&
      (to === null || item.entryDate <= to),
  );
  let emptyReason: ProjectEffort["emptyReason"] = null;
  if (matches.length === 0) {
    emptyReason = entries.length === 0 ? "no_effort" : "no_matches";
  }
  const totalHours = matches.reduce((sum, item) => sum + item.hours, 0);
  return jsonResponse(200, { entries: matches, totalHours, emptyReason });
}

// Backend de prueba del proyecto: responde el estado, el backlog, los sprints y el esfuerzo
// filtrado sobre los registros dados; cualquier otra ruta es 404.
function fakeBackend(entries: EffortEntry[]) {
  return async (input: RequestInfo | URL) => {
    const url = urlOf(input);
    switch (url.pathname) {
      case `/api/v1/projects/${projectId}`:
        return jsonResponse(200, projectDetail);
      case `/api/v1/projects/${projectId}/stories`:
        return jsonResponse(200, storyList);
      case `/api/v1/projects/${projectId}/sprints`:
        return jsonResponse(200, sprintList);
      case effortPath:
        return effortResponse(entries, url.searchParams);
      default:
        return jsonResponse(404, { error: "No encontrado." });
    }
  };
}

// Devuelve los parámetros de las consultas de esfuerzo, en el orden en que se pidieron.
function effortQueries(fetchMock: FetchMock): URLSearchParams[] {
  return fetchMock.mock.calls
    .map(([input]) => urlOf(input))
    .filter((url) => url.pathname === effortPath)
    .map((url) => url.searchParams);
}

// Devuelve los parámetros de la última consulta de esfuerzo como objeto, para compararlos enteros.
function lastEffortQuery(fetchMock: FetchMock): Record<string, string> {
  const queries = effortQueries(fetchMock);
  expect(queries.length).toBeGreaterThan(0);
  return Object.fromEntries(queries[queries.length - 1].entries());
}

// Monta la vista dentro del proveedor de sesión, como en el layout raíz.
function renderView() {
  render(
    <SessionProvider>
      <ProjectEffortView projectId={projectId} />
    </SessionProvider>,
  );
}

// Localiza un selector de filtro por su nombre accesible.
function select(name: "Integrante" | "Historia" | "Sprint"): HTMLSelectElement {
  return screen.getByRole<HTMLSelectElement>("combobox", { name });
}

// Espera a que el selector ofrezca la opción con el texto dado y la elige.
async function choose(
  name: "Integrante" | "Historia" | "Sprint",
  label: string,
) {
  const option = await within(
    await screen.findByRole("combobox", { name }),
  ).findByRole<HTMLOptionElement>("option", { name: label });
  fireEvent.change(select(name), { target: { value: option.value } });
}

// Escribe una fecha en el campo del rango indicado.
async function typeDate(label: "Desde" | "Hasta", value: string) {
  fireEvent.change(await screen.findByLabelText(label), { target: { value } });
}

// Espera a que la lista muestre exactamente los registros con las actividades dadas, en ese orden.
async function expectShownActivities(activities: string[]) {
  await waitFor(() => {
    const list = screen.getByRole("list", {
      name: "Registros de esfuerzo del proyecto",
    });
    const items = within(list).queryAllByRole("listitem");
    expect(items).toHaveLength(activities.length);
    activities.forEach((activity, index) => {
      expect(items[index].textContent).toContain(activity);
    });
  });
}

describe("006/US4 - Consultar el esfuerzo del proyecto con filtros", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y el backend de prueba con los
  // registros del proyecto.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(fakeBackend(projectEntries));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: sin filtros se pide el esfuerzo sin parámetros y se ven todos los registros con
  // fecha, autor, historia, actividad, horas y sprint, y el total general.
  it("Caso normal — sin filtros, todos los registros y el total general", async () => {
    renderView();

    await expectShownActivities(projectEntries.map((item) => item.activity));
    const list = screen.getByRole("list", {
      name: "Registros de esfuerzo del proyecto",
    });
    const [first, , , last] = within(list).getAllByRole("listitem");
    expect(within(first).getByText("22/10/2026")).toBeTruthy();
    expect(within(first).getByText("Ana Pérez")).toBeTruthy();
    expect(within(first).getByText("Alta de clientes")).toBeTruthy();
    expect(within(first).getByText("Diseño de la pantalla")).toBeTruthy();
    expect(within(first).getByText("3 h")).toBeTruthy();
    expect(within(first).getByText("Sprint 2")).toBeTruthy();
    expect(within(last).getByText("Sin sprint")).toBeTruthy();
    expect(screen.getByText("Total: 11,5 h")).toBeTruthy();
    expect(lastEffortQuery(fetchMock)).toEqual({});
  });

  // Caso alternativo: elegir un integrante pide solo sus registros y muestra su total.
  it("Caso alternativo — filtro por un integrante concreto", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await choose("Integrante", "Bruno Díaz");

    await expectShownActivities(["Revisión de código"]);
    expect(lastEffortQuery(fetchMock)).toEqual({ userId: bruno.id });
    expect(screen.getByText("Total: 4,5 h")).toBeTruthy();
  });

  // Caso alternativo: elegir un sprint pide los registros asociados a ese sprint (FR-039).
  it("Caso alternativo — filtro por un sprint concreto", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await choose("Sprint", "Sprint 2");

    await expectShownActivities([
      "Diseño de la pantalla",
      "Revisión de código",
    ]);
    expect(lastEffortQuery(fetchMock)).toEqual({ sprintId: sprintTwo.id });
    expect(screen.getByText("Total: 7,5 h")).toBeTruthy();
  });

  // Caso alternativo: el rango de fechas y la historia viajan juntos y se ven solo los registros
  // que cumplen ambas condiciones (FR-037).
  it("Caso alternativo — rango de fechas e historia a la vez", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await typeDate("Desde", "2026-10-10");
    await typeDate("Hasta", "2026-10-22");
    await choose("Historia", "Alta de clientes");

    await expectShownActivities(["Diseño de la pantalla", "Maquetado"]);
    expect(lastEffortQuery(fetchMock)).toEqual({
      storyId: clientsStory.id,
      from: "2026-10-10",
      to: "2026-10-22",
    });
    expect(screen.getByText("Total: 5 h")).toBeTruthy();
  });

  // Caso alternativo: "Sin sprint" viaja como sprintId=none y deja solo los registros cargados
  // cuando la historia estaba solo en el backlog (FR-039).
  it("Caso alternativo — filtro por sin sprint", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await choose("Sprint", "Sin sprint");

    await expectShownActivities(["Relevamiento inicial"]);
    expect(lastEffortQuery(fetchMock)).toEqual({ sprintId: "none" });
    expect(screen.getByText("Total: 2 h")).toBeTruthy();
  });

  // Caso límite: el mismo día como inicio y fin del rango pide ese único día (FR-038).
  it("Caso límite — rango de un solo día", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await typeDate("Desde", "2026-10-20");
    await typeDate("Hasta", "2026-10-20");

    await expectShownActivities(["Revisión de código"]);
    expect(lastEffortQuery(fetchMock)).toEqual({
      from: "2026-10-20",
      to: "2026-10-20",
    });
  });

  // Caso límite: una combinación que ninguna fila cumple muestra la lista vacía con total 0 y el
  // aviso de que nada coincide, que no es un error ni el aviso del proyecto sin esfuerzo (FR-042).
  it("Caso límite — combinación de filtros sin coincidencias", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await choose("Integrante", "Bruno Díaz");
    await choose("Sprint", "Sin sprint");

    expect(await screen.findByText(noMatchesMessage)).toBeTruthy();
    expect(lastEffortQuery(fetchMock)).toEqual({
      userId: bruno.id,
      sprintId: "none",
    });
    expect(screen.getByText("Total: 0 h")).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByText(noEffortMessage)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un proyecto donde nadie cargó horas muestra total 0 y el aviso propio, distinto
  // del de los filtros sin coincidencias.
  it("Caso límite — proyecto sin esfuerzo registrado", async () => {
    fetchMock.mockImplementation(fakeBackend([]));
    renderView();

    expect(await screen.findByText(noEffortMessage)).toBeTruthy();
    expect(screen.getByText("Total: 0 h")).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByText(noMatchesMessage)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: el selector de integrantes ofrece, sin repetir, a los integrantes vigentes (aun sin
  // registros) y a quien fue quitada del proyecto pero dejó registros; elegirla pide sus registros
  // (FR-040).
  it("Caso límite — filtro por una persona quitada del proyecto", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    const memberSelect = await screen.findByRole("combobox", {
      name: "Integrante",
    });
    await within(memberSelect).findByRole("option", { name: "Carla Ruiz" });
    const labels = within(memberSelect)
      .getAllByRole("option")
      .map((option) => option.textContent ?? "");
    for (const name of [
      "Ana Pérez",
      "Bruno Díaz",
      "Diego Sosa",
      "Carla Ruiz",
    ]) {
      expect(labels.filter((label) => label === name)).toHaveLength(1);
    }

    await choose("Integrante", "Carla Ruiz");

    await expectShownActivities(["Maquetado"]);
    expect(lastEffortQuery(fetchMock)).toEqual({ userId: carla.id });
    expect(screen.getByText("Total: 2 h")).toBeTruthy();
  });

  // Caso de error: un rango con el fin anterior al inicio no muestra registros y se informa con el
  // mensaje del rango invertido (FR-043).
  it("Caso de error — rango de fechas con el fin anterior al inicio", async () => {
    renderView();
    await expectShownActivities(projectEntries.map((item) => item.activity));

    await typeDate("Desde", "2026-10-20");
    await typeDate("Hasta", "2026-10-10");

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(invalidRangeMessage);
  });
});

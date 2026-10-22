import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { SprintDetailView } from "@/features/sprints/components/sprint-detail-view";
import { SprintSchedule } from "@/features/sprints/components/sprint-schedule";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintSchedule as SprintScheduleData,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";
import type { StoryListResponse } from "@/types/story";

// Pruebas de los días restantes del sprint Activo y de su integración en el detalle (T075). Los
// días los calcula el backend en la zona horaria del sistema (FR-053, RN-23) y llegan en schedule;
// acá se verifica que se muestran tal cual llegan, que el vencido se informa en palabras y nunca
// como un número negativo, y que el detalle del sprint Activo reúne todo en una pantalla (SC-003).
//
// Contrato que fijan estas pruebas para SprintSchedule:
// - Recibe el sprint (SprintDetail) y no hace pedidos. Si schedule es null (sprint Planificado o
//   Cerrado) no muestra nada.
// - Con remainingDays muestra "N días restantes" ("1 día restante" con un día y "0 días restantes"
//   el último día del sprint).
// - Con overdueDays muestra "Vencido hace N días" ("Vencido hace 1 día" con un día), sin ningún
//   número negativo y sin hablar de días restantes.
// - SprintDetailView integra SprintSchedule: el detalle de un sprint Activo muestra en la misma
//   pantalla el Sprint Goal, las fechas previstas, los días restantes y las historias comprometidas
//   con su estado.

// Sustituye el enrutador de Next.js porque la guardia de sesión del detalle lo usa: fuera del App
// Router no hay enrutador montado.
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
type ResponseBody = SprintDetail | StoryListResponse | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c9d-9e0f1a2b3c4d";
const sprintGoal = "Cerrar el alta de clientes";

// Historias comprometidas en el sprint Activo: una Completada y una Pendiente.
const completedStory: SprintStoryItem = {
  storyId: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  storyPoints: 5,
  status: "completed",
  completedAt: "2026-10-03T15:00:00Z",
  outcome: null,
};
const pendingStory: SprintStoryItem = {
  storyId: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "medium",
  storyPoints: 3,
  status: "pending",
  completedAt: null,
  outcome: null,
};

// Arma el sprint con el estado, el cronograma y las historias indicadas. Termina el 2026-10-14; los
// datos de inicio y cierre solo se completan donde el estado los tiene.
function sprintWith(
  status: SprintStatus,
  schedule: SprintScheduleData | null,
  stories: SprintStoryItem[] = [],
): SprintDetail {
  const closed = status === "closed";
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: sprintGoal,
    status,
    startDate: "2026-10-01",
    endDate: "2026-10-14",
    startedAt: status === "planned" ? null : "2026-10-01T12:00:00Z",
    closedAt: closed ? "2026-10-14T15:00:00Z" : null,
    createdBy: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    closedBy: closed
      ? { id: "5d2e8b1c-4a3f-4c6d-9e7a-2b1c0d9e8f7a", fullName: "Bruno Gómez" }
      : null,
    createdAt: "2026-09-28T12:00:00Z",
    updatedAt: "2026-10-01T12:00:00Z",
    stories,
    results: closed
      ? {
          committedStoryPoints: 8,
          plannedStoryPoints: 8,
          completedStoryPoints: 5,
          plannedStoryCount: 2,
          completedStoryCount: 1,
        }
      : null,
    hoursPerStoryPoint: closed ? 6.5 : null,
    schedule,
  };
}

// Cronograma con días restantes y con días de vencido.
function remaining(days: number): SprintScheduleData {
  return { remainingDays: days, overdueDays: null };
}
function overdue(days: number): SprintScheduleData {
  return { remainingDays: null, overdueDays: days };
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta y el método de una llamada a fetch.
function describeRequest(input: RequestInfo | URL, init?: RequestInit) {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const url = new URL(raw, "http://localhost");
  return {
    path: url.pathname,
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
  };
}

// Simula el backend del detalle: el sprint responde con el detalle dado y el backlog del proyecto,
// que pide el diálogo de compromiso, vacío. Cualquier otra ruta responde 404, para que una llamada
// inesperada no pase inadvertida.
function serve(sprint: SprintDetail) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (
      request.method === "GET" &&
      request.path === `/api/v1/sprints/${sprintId}`
    ) {
      return jsonResponse(200, sprint);
    }
    if (
      request.method === "GET" &&
      request.path === `/api/v1/projects/${projectId}/stories`
    ) {
      return jsonResponse(200, {
        stories: [],
        appliedFilters: { status: [], priority: [] },
        emptyReason: "backlog_empty",
      });
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Monta solo los días restantes del sprint dado y devuelve el texto mostrado.
function renderSchedule(sprint: SprintDetail): string {
  const { container } = render(<SprintSchedule sprint={sprint} />);
  return container.textContent ?? "";
}

describe("004/US6 - Consultar el historial de sprints", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña, por si monta el detalle completo.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(
      serve(
        sprintWith("active", remaining(10), [completedStory, pendingStory]),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: un sprint Activo que termina el 2026-10-14 consultado el 2026-10-04 llega con 10
  // días restantes y se muestran tal cual.
  it("Días restantes", () => {
    const text = renderSchedule(sprintWith("active", remaining(10)));

    expect(text).toContain("10 días restantes");
    expect(text).not.toMatch(/vencido/i);
  });

  // Caso límite: el último día del sprint llega con 0 días restantes, porque el día de fin se cuenta
  // y el sprint sigue en curso; no se presenta como vencido.
  it("Termina hoy", () => {
    const text = renderSchedule(sprintWith("active", remaining(0)));

    expect(text).toContain("0 días restantes");
    expect(text).not.toMatch(/vencido/i);
  });

  // Caso límite: un sprint Activo cuya fecha de fin pasó hace 3 días se muestra "Vencido hace 3
  // días", nunca como un número negativo ni como días restantes (RN-23).
  it("Vencido", () => {
    const text = renderSchedule(sprintWith("active", overdue(3)));

    expect(text).toContain("Vencido hace 3 días");
    expect(text).not.toMatch(/-\s*\d/);
    expect(text).not.toMatch(/restante/i);
  });

  // Caso alternativo: un sprint Planificado y uno Cerrado no informan días restantes ni vencido,
  // porque el dato solo aplica al sprint en curso.
  it("Sin días restantes fuera de Activo", () => {
    for (const status of ["planned", "closed"] as const) {
      const text = renderSchedule(sprintWith(status, null));
      expect(text, status).not.toMatch(/restante|vencido/i);
      cleanup();
    }
  });

  // Regla de la interfaz: con un solo día, el texto va en singular.
  it("usa el singular con un día", () => {
    expect(renderSchedule(sprintWith("active", remaining(1)))).toContain(
      "1 día restante",
    );
    cleanup();
    expect(renderSchedule(sprintWith("active", overdue(1)))).toContain(
      "Vencido hace 1 día",
    );
  });

  // SC-003: el detalle de un sprint Activo reúne en una sola pantalla el Sprint Goal, las fechas
  // previstas, los días restantes y las historias comprometidas con su estado.
  it("el sprint Activo concentra objetivo, fechas, días restantes e historias en una pantalla", async () => {
    render(
      <SessionProvider>
        <SprintDetailView sprintId={sprintId} />
      </SessionProvider>,
    );

    expect(await screen.findByText(sprintGoal)).toBeTruthy();
    const text = document.body.textContent ?? "";
    expect(text).toContain("01/10/2026");
    expect(text).toContain("14/10/2026");
    expect(text).toContain("10 días restantes");

    const completedItem = screen
      .getByText(completedStory.title)
      .closest("li") as HTMLElement;
    const pendingItem = screen
      .getByText(pendingStory.title)
      .closest("li") as HTMLElement;
    expect(within(completedItem).getByText(/Completada/)).toBeTruthy();
    expect(within(pendingItem).getByText(/Pendiente/)).toBeTruthy();
  });
});

// Pruebas de las acciones de modificación y eliminación en el detalle del sprint (T083).
// Atraviesan la vista de detalle y el cliente HTTP real; solo se sustituyen fetch y el enrutador
// de Next.js. La confirmación, el DELETE y el regreso al listado los prueba
// delete-sprint-button.test.tsx; acá solo se verifica qué ofrece el detalle.
//
// Contrato que fijan estas pruebas para T087:
// - SprintDetailView muestra el enlace "Editar sprint" a /projects/{projectId}/sprints/{sprintId}/edit
//   solo cuando el sprint está Planificado; en uno Activo o Cerrado no lo muestra (FR-012, FR-016).
// - SprintDetailView integra DeleteSprintButton: el botón "Eliminar sprint" aparece en el detalle
//   de un sprint Planificado y no en el de uno Activo o Cerrado (FR-048).
describe("004/US7 - Corregir o descartar un sprint planificado", () => {
  // Cada prueba arranca con una sesión válida en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Monta el detalle del sprint dado y espera a que llegue su Sprint Goal desde el backend.
  async function renderDetail(sprint: SprintDetail) {
    vi.stubGlobal("fetch", vi.fn<typeof fetch>(serve(sprint)));
    render(
      <SessionProvider>
        <SprintDetailView sprintId={sprintId} />
      </SessionProvider>,
    );
    await screen.findByText(sprintGoal);
  }

  // Sprints que ya arrancaron, en los que el detalle no ofrece modificar ni eliminar.
  const startedSprints: Array<[SprintStatus, SprintScheduleData | null]> = [
    ["active", remaining(10)],
    ["closed", null],
  ];

  // Editar solo se ofrece en un sprint Planificado: el enlace lleva a la pantalla de modificación
  // del sprint, y en uno Activo o Cerrado no aparece.
  it("Editar solo se ofrece en un sprint Planificado", async () => {
    await renderDetail(sprintWith("planned", null));
    const link = screen.getByRole("link", { name: "Editar sprint" });
    expect(link.getAttribute("href")).toBe(
      `/projects/${projectId}/sprints/${sprintId}/edit`,
    );
    cleanup();

    for (const [status, schedule] of startedSprints) {
      await renderDetail(sprintWith(status, schedule));
      expect(
        screen.queryByRole("link", { name: "Editar sprint" }),
        status,
      ).toBeNull();
      cleanup();
    }
  });

  // Eliminar solo se ofrece en un sprint Planificado: el detalle muestra el botón de eliminación, y
  // en uno Activo o Cerrado no aparece.
  it("Eliminar solo se ofrece en un sprint Planificado", async () => {
    await renderDetail(sprintWith("planned", null));
    expect(
      screen.getByRole("button", { name: "Eliminar sprint" }),
    ).not.toBeNull();
    cleanup();

    for (const [status, schedule] of startedSprints) {
      await renderDetail(sprintWith(status, schedule));
      expect(
        screen.queryByRole("button", { name: "Eliminar sprint" }),
        status,
      ).toBeNull();
      cleanup();
    }
  });
});

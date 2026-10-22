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
} from "@testing-library/react";
import { SprintLifecycleActions } from "@/features/sprints/components/sprint-lifecycle-actions";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintRef,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";

// Pruebas de la acción de iniciar un sprint (T044). Atraviesan el componente, la llamada de inicio
// de api.ts (T051) y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el
// backend. Las reglas del inicio (RN-02, RN-06, FR-027 a FR-032) las resuelve el backend: acá se
// verifica cuándo se ofrece la acción, qué envía y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para SprintLifecycleActions:
// - Recibe el sprint (SprintDetail) y onChanged, que recibe el SprintDetail devuelto por el backend
//   para que la pantalla de detalle lo muestre.
// - Solo en un sprint Planificado muestra el botón "Iniciar sprint"; en uno Activo o Cerrado no lo
//   muestra. El botón no depende de que haya historias ni de la fecha de inicio prevista.
// - El botón envía POST /api/v1/sprints/{sprintId}/start con la sesión de la pestaña y sin cuerpo,
//   y no hace ninguna otra llamada: el estado del proyecto no cambia al iniciar (FR-029).
// - Con un 200 llama una vez a onChanged con el SprintDetail recibido.
// - Con un rechazo muestra el mensaje del backend con role="alert" y no llama a onChanged; el 409
//   de otro sprint activo muestra el nombre de ese sprint.

// Sustituye el enrutador de Next.js por si el componente o la guardia de sesión lo usan: fuera del
// App Router no hay enrutador montado.
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
type ChangedMock = Mock<(sprint: SprintDetail) => void>;

// Rechazo 409 con el sprint en conflicto (SprintConflictResponse de contracts/openapi.yaml).
type SprintConflictBody = {
  error: string;
  sprint: SprintRef;
};

type ResponseBody = SprintDetail | ErrorResponse | SprintConflictBody;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";
const startPath = `/api/v1/sprints/${sprintId}/start`;
const startButtonName = /iniciar sprint/i;
const startedAt = "2026-10-15T12:00:00Z";

// Mensajes de contracts/openapi.yaml y de las pruebas de backend para los rechazos del inicio.
const activeSprint: SprintRef = {
  id: "7c1d2e9a-5b8f-4b7e-9c0a-1f2e3d4c5b6a",
  name: "Sprint 1",
};
const otherActiveMessage = `El proyecto ya tiene el sprint activo «${activeSprint.name}». Cerralo antes de iniciar otro.`;
const transitionMessage =
  "Transición no permitida: solo se inicia un sprint Planificado.";
const closedSprintMessage = "El sprint está cerrado y no admite cambios.";
const readOnlyMessage =
  "El proyecto está finalizado y sus sprints son de solo lectura.";

// Historias comprometidas en el sprint (US3, escenario 1).
const committedStories: SprintStoryItem[] = [
  {
    storyId: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
    title: "Exportar el reporte de velocidad a PDF",
    priority: "high",
    storyPoints: 5,
    status: "pending",
    completedAt: null,
    outcome: null,
  },
  {
    storyId: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
    title: "Registrar el esfuerzo diario",
    priority: "medium",
    storyPoints: 3,
    status: "pending",
    completedAt: null,
    outcome: null,
  },
];

// Arma el sprint con el estado, las historias y la fecha de inicio prevista indicadas.
function sprintWith(
  status: SprintStatus,
  stories: SprintStoryItem[] = committedStories,
  startDate = "2026-10-15",
): SprintDetail {
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate,
    endDate: "2099-12-31",
    startedAt: status === "planned" ? null : startedAt,
    closedAt: status === "closed" ? "2026-10-28T20:00:00Z" : null,
    createdBy: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    closedBy: null,
    createdAt: "2026-10-10T12:00:00Z",
    updatedAt: "2026-10-10T12:00:00Z",
    stories,
    results: null,
    hoursPerStoryPoint: null,
    schedule: null,
  };
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Hace que fetch responda siempre con el estado y el cuerpo indicados; cada llamada recibe una
// respuesta nueva porque el cuerpo de una Response solo se puede leer una vez.
function respondWith(fetchMock: FetchMock, status: number, body: ResponseBody) {
  fetchMock.mockImplementation(async () => jsonResponse(status, body));
}

// Extrae la ruta, el método, la cabecera Authorization y el cuerpo de la llamada número index a
// fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
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
    authorization: new Headers(init?.headers).get("Authorization"),
    body: init?.body,
  };
}

// Monta las acciones dentro del proveedor de sesión, como en el layout raíz.
function renderActions(sprint: SprintDetail, onChanged: ChangedMock) {
  return render(
    <SessionProvider>
      <SprintLifecycleActions sprint={sprint} onChanged={onChanged} />
    </SessionProvider>,
  );
}

// Pulsa el botón de inicio, que debe estar habilitado.
async function clickStart() {
  const button = await screen.findByRole("button", { name: startButtonName });
  expect(button.hasAttribute("disabled")).toBe(false);
  fireEvent.click(button);
}

// Verifica que fetch recibió una sola llamada y que es el inicio del sprint con la sesión y sin
// cuerpo: ninguna otra ruta, en particular ninguna del proyecto, se toca al iniciar.
function expectOnlyStartRequest(fetchMock: FetchMock) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const request = requestAt(fetchMock, 0);
  expect(request.method).toBe("POST");
  expect(request.path).toBe(startPath);
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  expect(request.body).toBeUndefined();
}

// Hace que el inicio responda 409 con el cuerpo dado, monta un sprint Planificado, pulsa el botón y
// verifica que el alert muestra el mensaje y que la pantalla no recibe ningún detalle. Devuelve el
// alert para las verificaciones propias de cada prueba.
async function expectStartRejected(
  fetchMock: FetchMock,
  onChanged: ChangedMock,
  body: ErrorResponse | SprintConflictBody,
  message: string,
): Promise<HTMLElement> {
  respondWith(fetchMock, 409, body);
  renderActions(sprintWith("planned"), onChanged);

  await clickStart();

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain(message);
  expect(onChanged).not.toHaveBeenCalled();
  return alert;
}

describe("004/US3 - Iniciar un sprint", () => {
  let fetchMock: FetchMock;
  let onChanged: ChangedMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que responde 200 con el
  // sprint ya Activo.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    onChanged = vi.fn<(sprint: SprintDetail) => void>();
    fetchMock = vi.fn<typeof fetch>();
    respondWith(fetchMock, 200, sprintWith("active"));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: en un sprint Planificado con historias, el botón envía el inicio con la sesión y
  // entrega a la pantalla el detalle Activo con su momento de inicio real.
  it("Inicio con historias comprometidas", async () => {
    const started = sprintWith("active");
    renderActions(sprintWith("planned"), onChanged);

    await clickStart();

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(onChanged).toHaveBeenCalledWith(started);
    expect(onChanged.mock.calls[0][0].startedAt).toBe(startedAt);
    expectOnlyStartRequest(fetchMock);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: iniciar en un proyecto Planificado solo llama al inicio del sprint; la
  // interfaz no pide ningún cambio de estado del proyecto.
  it("Proyecto Planificado no cambia", async () => {
    renderActions(sprintWith("planned"), onChanged);

    await clickStart();

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expectOnlyStartRequest(fetchMock);
  });

  // Caso alternativo: en un proyecto En curso el inicio tampoco toca el proyecto; la pantalla recibe
  // el sprint Activo y no se muestra ningún error.
  it("Proyecto En curso no cambia", async () => {
    renderActions(sprintWith("planned"), onChanged);

    await clickStart();

    await waitFor(() =>
      expect(onChanged).toHaveBeenCalledWith(sprintWith("active")),
    );
    expectOnlyStartRequest(fetchMock);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un sprint Planificado sin historias ofrece el inicio igual y la pantalla recibe el
  // sprint Activo y vacío, sin error.
  it("Sprint vacío", async () => {
    const startedEmpty = sprintWith("active", []);
    respondWith(fetchMock, 200, startedEmpty);
    renderActions(sprintWith("planned", []), onChanged);

    await clickStart();

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(startedEmpty));
    expectOnlyStartRequest(fetchMock);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un sprint cuya fecha de inicio prevista todavía no llegó ofrece el inicio
  // habilitado, porque el arranque lo decide el equipo y no el calendario.
  it("Inicio antes de la fecha", async () => {
    const futureStart = "2099-12-01";
    const started = sprintWith("active", committedStories, futureStart);
    respondWith(fetchMock, 200, started);
    renderActions(
      sprintWith("planned", committedStories, futureStart),
      onChanged,
    );

    await clickStart();

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(started));
    expectOnlyStartRequest(fetchMock);
  });

  // Caso de error: con otro sprint Activo en el proyecto, el 409 se muestra con el nombre de ese
  // sprint y la indicación de cerrarlo primero; la pantalla no recibe ningún detalle.
  it("Otro sprint activo", async () => {
    const alert = await expectStartRejected(
      fetchMock,
      onChanged,
      { error: otherActiveMessage, sprint: activeSprint },
      otherActiveMessage,
    );
    expect(alert.textContent).toContain(activeSprint.name);
    expect(screen.getByRole("button", { name: startButtonName })).toBeTruthy();
  });

  // Caso de error: un sprint Activo no ofrece el inicio; y si el backend rechaza por transición
  // inválida un sprint que ya arrancó en otra pestaña, se muestra el motivo y nada cambia.
  it("Sprint ya activo", async () => {
    renderActions(sprintWith("active"), onChanged);
    expect(screen.queryByRole("button", { name: startButtonName })).toBeNull();
    cleanup();

    await expectStartRejected(
      fetchMock,
      onChanged,
      { error: transitionMessage },
      transitionMessage,
    );
  });

  // Caso de error: un sprint Cerrado no ofrece el inicio; y si el backend rechaza porque el sprint
  // se cerró después de abrir la pantalla, se muestra el motivo sin entregar ningún detalle.
  it("Sprint cerrado", async () => {
    renderActions(sprintWith("closed"), onChanged);
    expect(screen.queryByRole("button", { name: startButtonName })).toBeNull();
    cleanup();

    await expectStartRejected(
      fetchMock,
      onChanged,
      { error: closedSprintMessage },
      closedSprintMessage,
    );
  });

  // Caso de error: en un proyecto Finalizado el backend rechaza el inicio por solo lectura; se
  // muestra ese mensaje y la pantalla no recibe ningún detalle.
  it("Proyecto finalizado", async () => {
    await expectStartRejected(
      fetchMock,
      onChanged,
      { error: readOnlyMessage },
      readOnlyMessage,
    );
  });

  // Regla de la interfaz (RN-02): el botón de inicio aparece solo en un sprint Planificado; en uno
  // Activo o Cerrado no se ofrece y no se envía nada al backend.
  it("muestra el botón de inicio solo en un sprint Planificado", async () => {
    renderActions(sprintWith("planned"), onChanged);
    expect(
      await screen.findByRole("button", { name: startButtonName }),
    ).toBeTruthy();
    cleanup();

    for (const status of ["active", "closed"] as const) {
      renderActions(sprintWith(status), onChanged);
      expect(
        screen.queryByRole("button", { name: startButtonName }),
        status,
      ).toBeNull();
      cleanup();
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

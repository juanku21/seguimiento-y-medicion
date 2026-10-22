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
import { SprintStoryList } from "@/features/sprints/components/sprint-story-list";
import { StoryProgressControl } from "@/features/sprints/components/story-progress-control";
import { isRecord } from "@/lib/api-client";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintStatus,
  SprintStoryItem,
  StoryProgressResponse,
} from "@/types/sprint";
import type { StoryStatus } from "@/types/story";

// Pruebas del registro de avance de las historias del sprint (T056). Atraviesan el control, la
// llamada de avance de api.ts (T061) y el cliente HTTP real; solo se sustituye fetch, que es la
// frontera con el backend. Las reglas del avance (RN-11, RN-12, FR-033 a FR-038) las resuelve el
// backend: acá se verifica qué transiciones ofrece la interfaz, qué envía y qué muestra con cada
// respuesta.
//
// Contrato que fijan estas pruebas:
// - StoryProgressControl recibe story (SprintStoryItem) y onChanged, que recibe el
//   StoryProgressResponse devuelto por el backend.
// - Ofrece un único botón con la transición admitida desde el estado actual: con Pendiente, "Pasar
//   a En progreso"; con En progreso, "Marcar como completada"; con Completada, "Volver a En
//   progreso". Nunca ofrece el salto de Pendiente a Completada ni volver a Pendiente.
// - El botón envía PUT /api/v1/stories/{storyId}/status con la sesión de la pestaña y el cuerpo
//   { status }, sin pedir confirmación. Con un 200 llama una vez a onChanged con la respuesta; con
//   un rechazo muestra el mensaje del backend con role="alert" y no llama a onChanged.
// - SprintStoryList muestra el control en cada fila solo cuando el sprint está Activo; en uno
//   Planificado o Cerrado no ofrece avance. Con el avance registrado entrega a su onChanged el
//   sprint con el estado y el completedAt de esa historia reemplazados por los recibidos.

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
type ProgressMock = Mock<(progress: StoryProgressResponse) => void>;
type SprintChangedMock = Mock<(sprint: SprintDetail) => void>;
type ResponseBody = StoryProgressResponse | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";

// Botones de cada transición admitida (RN-11).
const startButtonName = /pasar a en progreso/i;
const completeButtonName = /marcar como completada/i;
const reopenButtonName = /volver a en progreso/i;
const progressButtonNames = [
  startButtonName,
  completeButtonName,
  reopenButtonName,
];

// Instantes de completado que devuelve el backend; el segundo es el de un nuevo completado.
const firstCompletedAt = "2026-10-20T15:00:00Z";
const lastCompletedAt = "2026-10-22T18:30:00Z";

// Mensajes de los 409 de contracts/openapi.yaml para PUT /stories/{storyId}/status.
const outsideActiveMessage =
  "Solo se registra avance sobre historias del sprint activo.";
const closedSprintMessage = "El sprint está cerrado y no admite cambios.";
const transitionMessage =
  "Transición no permitida. Las admitidas son: Pendiente → En progreso, En progreso → Completada y Completada → En progreso.";

// Historias comprometidas en el sprint, una por estado.
const pendingStory: SprintStoryItem = {
  storyId: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  storyPoints: 5,
  status: "pending",
  completedAt: null,
  outcome: null,
};
const inProgressStory: SprintStoryItem = {
  storyId: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "medium",
  storyPoints: 3,
  status: "in_progress",
  completedAt: null,
  outcome: null,
};
const completedStory: SprintStoryItem = {
  storyId: "3c4d5e6f-7a8b-4c9d-8e0f-2a3b4c5d6e7f",
  title: "Ver el tablero del sprint",
  priority: "low",
  storyPoints: 8,
  status: "completed",
  completedAt: firstCompletedAt,
  outcome: null,
};

// Fecha local de hoy en AAAA-MM-DD, para el sprint que termina hoy.
function todayDate(): string {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

// Arma el sprint con el estado, las historias y la fecha de fin indicadas.
function sprintWith(
  status: SprintStatus,
  stories: SprintStoryItem[],
  endDate = "2026-10-28",
): SprintDetail {
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate: "2026-10-15",
    endDate,
    startedAt: status === "planned" ? null : "2026-10-15T12:00:00Z",
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

// Lee el estado destino del cuerpo JSON enviado; devuelve null si el cuerpo no tiene esa forma.
function statusOf(body: BodyInit | null | undefined): StoryStatus | null {
  if (typeof body !== "string") {
    return null;
  }
  const parsed: unknown = JSON.parse(body);
  if (!isRecord(parsed)) {
    return null;
  }
  const { status } = parsed;
  return status === "pending" ||
    status === "in_progress" ||
    status === "completed"
    ? status
    : null;
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

// Simula al backend que acepta el avance: responde 200 con la historia de la ruta, el estado
// pedido y, al completar, el siguiente instante de completedAts (el último se repite).
function acceptProgress(fetchMock: FetchMock, completedAts: string[]) {
  let completions = 0;
  fetchMock.mockImplementation(async (input, init) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const storyId = new URL(raw, "http://localhost").pathname.split("/")[4];
    const status = statusOf(init?.body);
    if (status === null) {
      return jsonResponse(400, { error: "Cuerpo inesperado en la prueba." });
    }
    let completedAt: string | null = null;
    if (status === "completed") {
      completedAt =
        completedAts[Math.min(completions, completedAts.length - 1)];
      completions++;
    }
    return jsonResponse(200, { storyId, sprintId, status, completedAt });
  });
}

// Verifica que la llamada número index a fetch es el avance de la historia al estado dado, con la
// sesión de la pestaña.
function expectProgressRequest(
  fetchMock: FetchMock,
  index: number,
  storyId: string,
  status: StoryStatus,
) {
  const request = requestAt(fetchMock, index);
  expect(request.method).toBe("PUT");
  expect(request.path).toBe(`/api/v1/stories/${storyId}/status`);
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  expect(statusOf(request.body)).toBe(status);
}

// Monta el control dentro del proveedor de sesión, como en el layout raíz.
function renderControl(story: SprintStoryItem, onChanged: ProgressMock) {
  return render(
    <SessionProvider>
      <StoryProgressControl story={story} onChanged={onChanged} />
    </SessionProvider>,
  );
}

// Monta la lista de historias del sprint con un onChanged que solo registra las llamadas.
function renderList(sprint: SprintDetail, onChanged: SprintChangedMock) {
  return render(
    <SessionProvider>
      <SprintStoryList sprint={sprint} onChanged={onChanged} />
    </SessionProvider>,
  );
}

// Pantalla mínima de detalle: guarda el sprint y lo reemplaza por el que entrega la lista, como
// hace la vista de detalle. Cada sprint recibido también se informa a onChanged.
function SprintScreen({
  initial,
  onChanged,
}: {
  initial: SprintDetail;
  onChanged: (sprint: SprintDetail) => void;
}) {
  const [sprint, setSprint] = useState(initial);
  return (
    <SprintStoryList
      sprint={sprint}
      onChanged={(updated) => {
        setSprint(updated);
        onChanged(updated);
      }}
    />
  );
}

// Monta la pantalla mínima de detalle dentro del proveedor de sesión.
function renderScreen(sprint: SprintDetail, onChanged: SprintChangedMock) {
  return render(
    <SessionProvider>
      <SprintScreen initial={sprint} onChanged={onChanged} />
    </SessionProvider>,
  );
}

// Devuelve el elemento de lista que muestra la historia dada.
function itemOf(story: SprintStoryItem): HTMLElement {
  const item = screen
    .getAllByRole("listitem")
    .find((element) => element.textContent?.includes(story.title));
  expect(item, story.title).toBeDefined();
  return item as HTMLElement;
}

// Devuelve la historia del último sprint entregado a onChanged.
function lastStoryOf(
  onChanged: SprintChangedMock,
  storyId: string,
): SprintStoryItem | undefined {
  const calls = onChanged.mock.calls;
  return calls[calls.length - 1]?.[0].stories.find(
    (story) => story.storyId === storyId,
  );
}

// Verifica que el contenedor no ofrece ningún botón de avance.
function expectNoProgressButtons(container: HTMLElement) {
  for (const name of progressButtonNames) {
    expect(within(container).queryByRole("button", { name })).toBeNull();
  }
}

// Hace que el avance responda 409 con el mensaje dado, monta el control con la historia, pulsa su
// único botón de avance y verifica que se muestra el mensaje sin entregar ningún avance.
async function expectProgressRejected(
  fetchMock: FetchMock,
  onChanged: ProgressMock,
  story: SprintStoryItem,
  buttonName: RegExp,
  message: string,
) {
  respondWith(fetchMock, 409, { error: message });
  renderControl(story, onChanged);

  fireEvent.click(await screen.findByRole("button", { name: buttonName }));

  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain(message);
  expect(onChanged).not.toHaveBeenCalled();
  expect(fetchMock).toHaveBeenCalledTimes(1);
}

describe("004/US4 - Registrar el avance de las historias del sprint", () => {
  let fetchMock: FetchMock;
  let onProgress: ProgressMock;
  let onSprintChanged: SprintChangedMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un backend que acepta el avance.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    onProgress = vi.fn<(progress: StoryProgressResponse) => void>();
    onSprintChanged = vi.fn<(sprint: SprintDetail) => void>();
    fetchMock = vi.fn<typeof fetch>();
    acceptProgress(fetchMock, [firstCompletedAt, lastCompletedAt]);
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: una historia Pendiente pasa a En progreso y después a Completada; la pantalla
  // recibe la historia Completada con la fecha y hora de completado que registró el backend.
  it("Avance hasta Completada", async () => {
    renderScreen(sprintWith("active", [pendingStory]), onSprintChanged);

    fireEvent.click(
      within(itemOf(pendingStory)).getByRole("button", {
        name: startButtonName,
      }),
    );
    await waitFor(() =>
      expect(lastStoryOf(onSprintChanged, pendingStory.storyId)?.status).toBe(
        "in_progress",
      ),
    );
    fireEvent.click(
      await within(itemOf(pendingStory)).findByRole("button", {
        name: completeButtonName,
      }),
    );

    await waitFor(() =>
      expect(lastStoryOf(onSprintChanged, pendingStory.storyId)).toMatchObject({
        status: "completed",
        completedAt: firstCompletedAt,
      }),
    );
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expectProgressRequest(fetchMock, 0, pendingStory.storyId, "in_progress");
    expectProgressRequest(fetchMock, 1, pendingStory.storyId, "completed");
    expect(itemOf(pendingStory).textContent).toContain("Completada");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: una historia Completada vuelve a En progreso y la fecha y hora de completado
  // llega borrada.
  it("Desmarcar una Completada", async () => {
    renderControl(completedStory, onProgress);

    fireEvent.click(
      await screen.findByRole("button", { name: reopenButtonName }),
    );

    await waitFor(() => expect(onProgress).toHaveBeenCalledTimes(1));
    expect(onProgress).toHaveBeenCalledWith({
      storyId: completedStory.storyId,
      sprintId,
      status: "in_progress",
      completedAt: null,
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expectProgressRequest(fetchMock, 0, completedStory.storyId, "in_progress");
  });

  // Caso alternativo: una historia Completada se devuelve a En progreso y se completa de nuevo; la
  // pantalla queda con la fecha y hora del último completado.
  it("Recompletar registra el último", async () => {
    acceptProgress(fetchMock, [lastCompletedAt]);
    renderScreen(sprintWith("active", [completedStory]), onSprintChanged);

    fireEvent.click(
      within(itemOf(completedStory)).getByRole("button", {
        name: reopenButtonName,
      }),
    );
    await waitFor(() =>
      expect(
        lastStoryOf(onSprintChanged, completedStory.storyId),
      ).toMatchObject({ status: "in_progress", completedAt: null }),
    );
    fireEvent.click(
      await within(itemOf(completedStory)).findByRole("button", {
        name: completeButtonName,
      }),
    );

    await waitFor(() =>
      expect(
        lastStoryOf(onSprintChanged, completedStory.storyId),
      ).toMatchObject({ status: "completed", completedAt: lastCompletedAt }),
    );
    expectProgressRequest(fetchMock, 1, completedStory.storyId, "completed");
  });

  // Caso límite: una historia reestimada durante el sprint se completa igual; el avance solo envía
  // el estado, sin Story Points, y la pantalla conserva los Story Points vigentes.
  it("Reestimación durante el sprint", async () => {
    const reestimated: SprintStoryItem = { ...inProgressStory, storyPoints: 8 };
    renderScreen(sprintWith("active", [reestimated]), onSprintChanged);

    fireEvent.click(
      within(itemOf(reestimated)).getByRole("button", {
        name: completeButtonName,
      }),
    );

    await waitFor(() =>
      expect(lastStoryOf(onSprintChanged, reestimated.storyId)).toMatchObject({
        status: "completed",
        storyPoints: 8,
      }),
    );
    const { body } = requestAt(fetchMock, 0);
    expect(typeof body === "string" ? JSON.parse(body) : body).toEqual({
      status: "completed",
    });
  });

  // Caso límite: en el último día del sprint Activo el avance se ofrece y se acepta, porque el
  // sprint admite avance hasta que se cierra.
  it("Último día del sprint", async () => {
    renderScreen(
      sprintWith("active", [inProgressStory], todayDate()),
      onSprintChanged,
    );

    const button = within(itemOf(inProgressStory)).getByRole("button", {
      name: completeButtonName,
    });
    expect(button.hasAttribute("disabled")).toBe(false);
    fireEvent.click(button);

    await waitFor(() =>
      expect(
        lastStoryOf(onSprintChanged, inProgressStory.storyId),
      ).toMatchObject({ status: "completed", completedAt: firstCompletedAt }),
    );
    expectProgressRequest(fetchMock, 0, inProgressStory.storyId, "completed");
  });

  // Caso de error: en un sprint Planificado la lista no ofrece avance; y si el backend rechaza
  // porque la historia no está en el sprint activo, el control muestra el motivo.
  it("Sprint Planificado", async () => {
    renderList(
      sprintWith("planned", [pendingStory, inProgressStory]),
      onSprintChanged,
    );
    expectNoProgressButtons(screen.getByRole("list"));
    cleanup();

    await expectProgressRejected(
      fetchMock,
      onProgress,
      pendingStory,
      startButtonName,
      outsideActiveMessage,
    );
    expectProgressRequest(fetchMock, 0, pendingStory.storyId, "in_progress");
  });

  // Caso de error: si la historia salió del sprint (por ejemplo, la quitaron en otra pestaña), el
  // backend rechaza completarla y el control muestra que solo se completan historias del activo.
  it("Historia sin sprint", async () => {
    await expectProgressRejected(
      fetchMock,
      onProgress,
      inProgressStory,
      completeButtonName,
      outsideActiveMessage,
    );
    expectProgressRequest(fetchMock, 0, inProgressStory.storyId, "completed");
  });

  // Caso de error: un sprint Cerrado no ofrece avance en ninguna historia; y si el sprint se cerró
  // después de abrir la pantalla, el control muestra el rechazo del backend.
  it("Sprint Cerrado", async () => {
    renderList(
      sprintWith("closed", [
        { ...pendingStory, outcome: "not_completed" },
        { ...completedStory, outcome: "completed" },
      ]),
      onSprintChanged,
    );
    expectNoProgressButtons(screen.getByRole("list"));
    cleanup();

    await expectProgressRejected(
      fetchMock,
      onProgress,
      completedStory,
      reopenButtonName,
      closedSprintMessage,
    );
  });

  // Caso de error: una historia Pendiente no ofrece completarse directamente; y si el backend
  // rechaza un salto (la historia volvió a Pendiente en otra pestaña), se muestra la lista de
  // transiciones admitidas.
  it("Salto de Pendiente a Completada", async () => {
    renderControl(pendingStory, onProgress);
    expect(
      await screen.findByRole("button", { name: startButtonName }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: completeButtonName }),
    ).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
    cleanup();

    await expectProgressRejected(
      fetchMock,
      onProgress,
      inProgressStory,
      completeButtonName,
      transitionMessage,
    );
  });

  // Regla de la interfaz (RN-11): en un sprint Activo cada fila ofrece una sola acción de avance,
  // la transición admitida desde el estado actual, y nunca el salto ni la vuelta a Pendiente.
  it("ofrece solo la transición admitida desde el estado actual", () => {
    renderList(
      sprintWith("active", [pendingStory, inProgressStory, completedStory]),
      onSprintChanged,
    );

    const expected: [SprintStoryItem, RegExp][] = [
      [pendingStory, startButtonName],
      [inProgressStory, completeButtonName],
      [completedStory, reopenButtonName],
    ];
    for (const [story, offered] of expected) {
      const row = itemOf(story);
      for (const name of progressButtonNames) {
        const button = within(row).queryByRole("button", { name });
        if (name === offered) {
          expect(button, `${story.status}: ${offered}`).not.toBeNull();
        } else {
          expect(button, `${story.status}: ${name}`).toBeNull();
        }
      }
      expect(
        within(row).queryByRole("button", { name: /pendiente/i }),
        story.status,
      ).toBeNull();
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // SC-002: desde una historia Pendiente del sprint Activo, completarla lleva no más de 3 pasos de
  // la persona (cada pulsación cuenta como un paso, sin confirmaciones intermedias).
  it("completa una historia en no más de 3 pasos", async () => {
    renderScreen(sprintWith("active", [pendingStory]), onSprintChanged);
    let steps = 0;

    fireEvent.click(
      within(itemOf(pendingStory)).getByRole("button", {
        name: startButtonName,
      }),
    );
    steps++;
    fireEvent.click(
      await within(itemOf(pendingStory)).findByRole("button", {
        name: completeButtonName,
      }),
    );
    steps++;

    await waitFor(() =>
      expect(lastStoryOf(onSprintChanged, pendingStory.storyId)?.status).toBe(
        "completed",
      ),
    );
    expect(steps).toBeLessThanOrEqual(3);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});

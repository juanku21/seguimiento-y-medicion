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
import { CommitStoriesDialog } from "@/features/sprints/components/commit-stories-dialog";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintRef,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";
import type {
  ReadinessMissing,
  StoryListItem,
  StoryListResponse,
} from "@/types/story";

// Pruebas del diálogo de compromiso de historias en un sprint (T030). Atraviesan el componente, las
// llamadas de api.ts (listado de la 003 y asignación de T038) y el cliente HTTP real; solo se
// sustituye fetch, que es la frontera con el backend. La admisión (RN-08) y la atomicidad del lote
// las resuelve el backend: acá se verifica qué ofrece el diálogo, qué envía y qué muestra con cada
// respuesta.
//
// Contrato que fijan estas pruebas para CommitStoriesDialog:
// - Recibe el sprint abierto (SprintDetail) y onCommitted, que recibe el SprintDetail devuelto por
//   el backend para que la pantalla de detalle lo muestre.
// - Pide el backlog del proyecto del sprint con GET /api/v1/projects/{projectId}/stories y la
//   sesión de la pestaña, y ofrece como casilla de verificación (role="checkbox", con el título de
//   la historia como nombre accesible), dentro de un elemento de lista, solo las historias listas
//   para planificar, no Completadas y que no están ya en este sprint.
// - El botón "Comprometer historias" envía POST /api/v1/sprints/{sprintId}/stories con la sesión y
//   el cuerpo {"storyIds": [...]} con las historias elegidas.
// - Con un 200 llama una vez a onCommitted con el SprintDetail recibido.
// - Con un rechazo muestra el mensaje general con role="alert" y no llama a onCommitted. En el 409
//   de historias no admitidas muestra, además, el mensaje de cada motivo dentro del elemento de
//   lista de su historia.

// Sustituye el enrutador de Next.js por si el diálogo o la guardia de sesión lo usan: fuera del
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
type CommittedMock = Mock<(sprint: SprintDetail) => void>;

// Motivo de no admisión según AdmissionProblem de contracts/openapi.yaml. Se declara acá para armar
// el cuerpo del 409 sin depender de los tipos de T038.
type AdmissionProblemBody = {
  storyId: string;
  reason: "not_ready" | "completed" | "committed_elsewhere";
  missing?: ReadinessMissing[];
  sprint?: SprintRef;
  message: string;
};

// Rechazo 409 por historias no admisibles (StoriesNotAdmittedResponse).
type StoriesNotAdmittedBody = {
  error: string;
  stories: AdmissionProblemBody[];
};

type ResponseBody =
  StoryListResponse | SprintDetail | ErrorResponse | StoriesNotAdmittedBody;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";
const storiesPath = `/api/v1/projects/${projectId}/stories`;
const sprintStoriesPath = `/api/v1/sprints/${sprintId}/stories`;

// Mensajes de contracts/openapi.yaml y de data-model.md (sección 9) para los rechazos.
const notAdmittedError =
  "Algunas historias no se pueden comprometer en el sprint; no se asignó ninguna.";
const notReadyMessage =
  "No está lista para planificar: le faltan Story Points o criterios de aceptación.";
const completedMessage = "La historia ya está terminada.";
const otherSprint: SprintRef = {
  id: "7c1d2e9a-5b8f-4b7e-9c0a-1f2e3d4c5b6a",
  name: "Sprint 1",
};
const committedElsewhereMessage = `Ya está comprometida en el sprint «${otherSprint.name}».`;
const closedSprintMessage = "El sprint está cerrado y no admite cambios.";
const storyNotFoundMessage = "La historia no existe.";

// Historias del backlog listas para planificar y Pendientes (US2, escenario 1).
const readyA: StoryListItem = {
  id: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  status: "pending",
  storyPoints: 5,
  acceptanceCriteriaCount: 2,
  readyToPlan: true,
};
const readyB: StoryListItem = {
  id: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "medium",
  status: "pending",
  storyPoints: 0,
  acceptanceCriteriaCount: 1,
  readyToPlan: true,
};
const readyC: StoryListItem = {
  id: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  title: "Mostrar el burndown del sprint",
  priority: "low",
  status: "pending",
  storyPoints: 8,
  acceptanceCriteriaCount: 3,
  readyToPlan: true,
};

// Historias que el diálogo no ofrece: sin Story Points, sin criterios, Completada y ya comprometida
// en este sprint.
const withoutStoryPoints: StoryListItem = {
  id: "4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f70",
  title: "Revisar los permisos de los integrantes",
  priority: "high",
  status: "pending",
  storyPoints: null,
  acceptanceCriteriaCount: 2,
  readyToPlan: false,
};
const withoutCriteria: StoryListItem = {
  id: "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a81",
  title: "Enviar el aviso de sprint por vencer",
  priority: "medium",
  status: "pending",
  storyPoints: 3,
  acceptanceCriteriaCount: 0,
  readyToPlan: false,
};
const completedStory: StoryListItem = {
  id: "6f7a8b9c-0d1e-4f2a-9b3c-4d5e6f7a8b92",
  title: "Iniciar sesión con correo y contraseña",
  priority: "high",
  status: "completed",
  storyPoints: 5,
  acceptanceCriteriaCount: 2,
  readyToPlan: true,
};
const alreadyInSprint: StoryListItem = {
  id: "7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9ca3",
  title: "Cargar el backlog inicial",
  priority: "low",
  status: "pending",
  storyPoints: 2,
  acceptanceCriteriaCount: 1,
  readyToPlan: true,
};

// Arma un sprint abierto del proyecto, con el estado y las historias comprometidas indicadas.
function sprintWith(
  status: SprintStatus,
  stories: SprintStoryItem[] = [],
): SprintDetail {
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate: "2026-10-15",
    endDate: "2026-10-28",
    startedAt: status === "active" ? "2026-10-15T12:00:00Z" : null,
    closedAt: null,
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

// Convierte un elemento del backlog en la historia comprometida que muestra el detalle del sprint.
function asSprintStory(story: StoryListItem): SprintStoryItem {
  return {
    storyId: story.id,
    title: story.title,
    priority: story.priority,
    storyPoints: story.storyPoints ?? 0,
    status: story.status,
    completedAt: null,
    outcome: null,
  };
}

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta, el método, la cabecera Authorization y el cuerpo de una llamada a fetch.
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
    authorization: new Headers(init?.headers).get("Authorization"),
    body: init?.body === undefined ? undefined : String(init.body),
  };
}

// Simula el backend: el listado del backlog responde con las historias dadas y la asignación con
// el estado y el cuerpo indicados. Cualquier otra ruta responde 404, para que una llamada
// inesperada no pase inadvertida.
function serve(
  backlog: StoryListItem[],
  assignStatus: number,
  assignBody: ResponseBody,
) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "GET" && request.path === storiesPath) {
      return jsonResponse(200, {
        stories: backlog,
        appliedFilters: { status: [], priority: [] },
        emptyReason: backlog.length === 0 ? "backlog_empty" : null,
      });
    }
    if (request.method === "POST" && request.path === sprintStoriesPath) {
      return jsonResponse(assignStatus, assignBody);
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Devuelve las llamadas de asignación que recibió fetch, con su cuerpo ya interpretado.
function assignRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .map(([input, init]) => describeRequest(input, init))
    .filter(
      (request) =>
        request.method === "POST" && request.path === sprintStoriesPath,
    )
    .map((request) => ({
      ...request,
      storyIds: (JSON.parse(String(request.body)) as { storyIds: string[] })
        .storyIds,
    }));
}

// Monta el diálogo dentro del proveedor de sesión, como en el layout raíz.
function renderDialog(sprint: SprintDetail, onCommitted: CommittedMock) {
  return render(
    <SessionProvider>
      <CommitStoriesDialog sprint={sprint} onCommitted={onCommitted} />
    </SessionProvider>,
  );
}

// Espera a que el diálogo ofrezca las historias dadas, las elige y envía la asignación.
async function chooseAndCommit(stories: StoryListItem[]) {
  for (const story of stories) {
    fireEvent.click(await screen.findByRole("checkbox", { name: story.title }));
  }
  fireEvent.click(
    screen.getByRole("button", { name: /comprometer historias/i }),
  );
}

// Devuelve el elemento de lista en el que el diálogo ofrece la historia dada.
function offeredItem(story: StoryListItem): HTMLElement {
  const item = screen
    .getByRole("checkbox", { name: story.title })
    .closest("li");
  expect(item, story.title).not.toBeNull();
  return item as HTMLElement;
}

// Cuerpo del 409 de historias no admitidas con los motivos dados.
function notAdmitted(stories: AdmissionProblemBody[]): StoriesNotAdmittedBody {
  return { error: notAdmittedError, stories };
}

describe("004/US2 - Comprometer historias en un sprint", () => {
  let fetchMock: FetchMock;
  let onCommitted: CommittedMock;

  // Cada prueba arranca con una sesión válida en la pestaña, un backlog con tres historias listas y
  // una asignación que responde 200 con el sprint y las tres comprometidas.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    onCommitted = vi.fn<(sprint: SprintDetail) => void>();
    fetchMock = vi.fn<typeof fetch>(
      serve(
        [readyA, readyB, readyC],
        200,
        sprintWith("planned", [readyA, readyB, readyC].map(asSprintStory)),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: el diálogo pide el backlog del proyecto del sprint con la sesión, envía las tres
  // historias elegidas en un solo POST y entrega a la pantalla el detalle con las tres comprometidas.
  it("Asignación de tres historias", async () => {
    const updated = sprintWith(
      "planned",
      [readyA, readyB, readyC].map(asSprintStory),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA, readyB, readyC]);

    await waitFor(() => expect(onCommitted).toHaveBeenCalledTimes(1));
    expect(onCommitted).toHaveBeenCalledWith(updated);

    const listRequest = fetchMock.mock.calls
      .map(([input, init]) => describeRequest(input, init))
      .find(
        (request) => request.method === "GET" && request.path === storiesPath,
      );
    expect(listRequest?.authorization).toBe(`Bearer ${sessionToken}`);

    const requests = assignRequests(fetchMock);
    expect(requests).toHaveLength(1);
    expect(requests[0].authorization).toBe(`Bearer ${sessionToken}`);
    expect([...requests[0].storyIds].sort()).toEqual(
      [readyA.id, readyB.id, readyC.id].sort(),
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: en un sprint Activo el diálogo ofrece y compromete historias igual que en uno
  // Planificado, porque el alcance puede ajustarse mientras el sprint corre.
  it("Asignación en sprint Activo", async () => {
    const updated = sprintWith("active", [asSprintStory(readyA)]);
    fetchMock.mockImplementation(serve([readyA, readyB], 200, updated));
    renderDialog(sprintWith("active"), onCommitted);

    await chooseAndCommit([readyA]);

    await waitFor(() => expect(onCommitted).toHaveBeenCalledWith(updated));
    expect(assignRequests(fetchMock)[0].storyIds).toEqual([readyA.id]);
  });

  // Filtro del diálogo (RN-08, FR-018 a FR-020): de todo el backlog ofrece solo las historias listas
  // para planificar, no Completadas y que no están ya comprometidas en este sprint abierto.
  it("ofrece solo historias del backlog listas, no completadas y sin sprint abierto", async () => {
    fetchMock.mockImplementation(
      serve(
        [
          readyA,
          withoutStoryPoints,
          withoutCriteria,
          completedStory,
          alreadyInSprint,
          readyB,
        ],
        200,
        sprintWith("planned"),
      ),
    );
    renderDialog(
      sprintWith("planned", [asSprintStory(alreadyInSprint)]),
      onCommitted,
    );

    await screen.findByRole("checkbox", { name: readyA.title });
    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
    expect(screen.getByRole("checkbox", { name: readyB.title })).toBeTruthy();
    for (const story of [
      withoutStoryPoints,
      withoutCriteria,
      completedStory,
      alreadyInSprint,
    ]) {
      expect(
        screen.queryByRole("checkbox", { name: story.title }),
        story.title,
      ).toBeNull();
    }
  });

  // Caso de error: una historia sin Story Points o sin criterios no se ofrece; si el backend la
  // rechaza igual por no estar lista (cambió después de abrir el diálogo), se muestra el motivo junto
  // a la historia y no se compromete nada.
  it("Historia no lista", async () => {
    fetchMock.mockImplementation(
      serve(
        [readyA, withoutStoryPoints, withoutCriteria],
        409,
        notAdmitted([
          {
            storyId: readyA.id,
            reason: "not_ready",
            missing: ["acceptance_criteria"],
            message: notReadyMessage,
          },
        ]),
      ),
    );
    renderDialog(sprintWith("planned"), onCommitted);
    await screen.findByRole("checkbox", { name: readyA.title });
    expect(
      screen.queryByRole("checkbox", { name: withoutStoryPoints.title }),
    ).toBeNull();
    expect(
      screen.queryByRole("checkbox", { name: withoutCriteria.title }),
    ).toBeNull();

    await chooseAndCommit([readyA]);

    expect(await screen.findByRole("alert")).toBeTruthy();
    await waitFor(() =>
      expect(offeredItem(readyA).textContent).toContain(notReadyMessage),
    );
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Caso de error: una historia Completada no se ofrece; si el backend la rechaza por terminada
  // (se completó después de abrir el diálogo), se muestra el motivo junto a la historia.
  it("Historia Completada", async () => {
    fetchMock.mockImplementation(
      serve(
        [readyA, completedStory],
        409,
        notAdmitted([
          {
            storyId: readyA.id,
            reason: "completed",
            message: completedMessage,
          },
        ]),
      ),
    );
    renderDialog(sprintWith("planned"), onCommitted);
    await screen.findByRole("checkbox", { name: readyA.title });
    expect(
      screen.queryByRole("checkbox", { name: completedStory.title }),
    ).toBeNull();

    await chooseAndCommit([readyA]);

    await waitFor(() =>
      expect(offeredItem(readyA).textContent).toContain(completedMessage),
    );
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Caso de error: una historia comprometida en otro sprint abierto se rechaza indicando en qué
  // sprint está; el diálogo lo muestra junto a la historia.
  it("Comprometida en otro sprint", async () => {
    fetchMock.mockImplementation(
      serve(
        [readyA],
        409,
        notAdmitted([
          {
            storyId: readyA.id,
            reason: "committed_elsewhere",
            sprint: otherSprint,
            message: committedElsewhereMessage,
          },
        ]),
      ),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA]);

    await waitFor(() =>
      expect(offeredItem(readyA).textContent).toContain(
        committedElsewhereMessage,
      ),
    );
    expect(offeredItem(readyA).textContent).toContain(otherSprint.name);
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Caso de error: el backend rechaza la asignación en un sprint Cerrado; el diálogo muestra el
  // motivo y no entrega ningún detalle.
  it("Sprint Cerrado", async () => {
    fetchMock.mockImplementation(
      serve([readyA], 409, { error: closedSprintMessage }),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA]);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(closedSprintMessage);
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Caso de error: en un lote de tres con una no lista, el rechazo es entero; el motivo aparece solo
  // junto a la historia que falla, las otras dos no muestran motivo y la pantalla no recibe ningún
  // detalle, porque ninguna quedó asignada.
  it("Lote parcialmente inválido", async () => {
    fetchMock.mockImplementation(
      serve(
        [readyA, readyB, readyC],
        409,
        notAdmitted([
          {
            storyId: readyB.id,
            reason: "not_ready",
            missing: ["story_points"],
            message: notReadyMessage,
          },
        ]),
      ),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA, readyB, readyC]);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notAdmittedError);
    await waitFor(() =>
      expect(offeredItem(readyB).textContent).toContain(notReadyMessage),
    );
    expect(offeredItem(readyA).textContent).not.toContain(notReadyMessage);
    expect(offeredItem(readyC).textContent).not.toContain(notReadyMessage);
    expect(assignRequests(fetchMock)).toHaveLength(1);
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Caso de error: una historia que el backend no reconoce en el proyecto responde 404 de historia
  // inexistente; el diálogo muestra ese mensaje.
  it("Historia de otro proyecto", async () => {
    fetchMock.mockImplementation(
      serve([readyA], 404, { error: storyNotFoundMessage }),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA]);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(storyNotFoundMessage);
    expect(onCommitted).not.toHaveBeenCalled();
  });

  // Cada motivo del 409 (data-model.md sección 9) se muestra junto a su historia: no lista,
  // Completada y comprometida en otro sprint, cada uno en el elemento de lista que le corresponde.
  it("muestra cada motivo del 409 junto a su historia", async () => {
    fetchMock.mockImplementation(
      serve(
        [readyA, readyB, readyC],
        409,
        notAdmitted([
          {
            storyId: readyA.id,
            reason: "not_ready",
            missing: ["story_points", "acceptance_criteria"],
            message: notReadyMessage,
          },
          {
            storyId: readyB.id,
            reason: "completed",
            message: completedMessage,
          },
          {
            storyId: readyC.id,
            reason: "committed_elsewhere",
            sprint: otherSprint,
            message: committedElsewhereMessage,
          },
        ]),
      ),
    );
    renderDialog(sprintWith("planned"), onCommitted);

    await chooseAndCommit([readyA, readyB, readyC]);

    await waitFor(() =>
      expect(offeredItem(readyA).textContent).toContain(notReadyMessage),
    );
    const itemB = within(offeredItem(readyB));
    expect(itemB.getByText(completedMessage, { exact: false })).toBeTruthy();
    expect(itemB.queryByText(notReadyMessage, { exact: false })).toBeNull();
    const itemC = within(offeredItem(readyC));
    expect(
      itemC.getByText(committedElsewhereMessage, { exact: false }),
    ).toBeTruthy();
    expect(itemC.queryByText(completedMessage, { exact: false })).toBeNull();
    expect(offeredItem(readyA).textContent).not.toContain(completedMessage);
    expect(onCommitted).not.toHaveBeenCalled();
  });
});

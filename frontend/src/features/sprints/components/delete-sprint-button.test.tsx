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
import { DeleteSprintButton } from "@/features/sprints/components/delete-sprint-button";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";

// Pruebas del botón de eliminación de un sprint (T083). Atraviesan el componente, la llamada de
// eliminación de api.ts (T087) y el cliente HTTP real; solo se sustituyen fetch, que es la frontera
// con el backend, y el enrutador de Next.js. Las reglas de la eliminación (sprint ya iniciado,
// esfuerzo registrado, sprint inexistente; FR-048, RN-20) las resuelve el backend: acá se verifica
// cuándo se ofrece la acción, qué envía y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para T087:
// - DeleteSprintButton recibe el sprint (SprintDetail). Solo en un sprint Planificado muestra el
//   botón "Eliminar sprint"; en uno Activo o Cerrado no lo muestra.
// - "Eliminar sprint" no envía nada: primero pide confirmación con la pregunta "¿Eliminar este
//   sprint?" y los botones "Confirmar eliminación" y "Cancelar". Cancelar cierra la confirmación sin
//   enviar nada.
// - "Confirmar eliminación" envía DELETE /api/v1/sprints/{sprintId} con la sesión de la pestaña y
//   sin cuerpo.
// - Con un 204 regresa al listado de sprints del proyecto con
//   router.replace("/projects/{projectId}/sprints"), para que volver atrás no lleve al detalle de un
//   sprint que ya no existe.
// - Con un rechazo (409 o 404) muestra el mensaje del backend con role="alert", no regresa al
//   listado y vuelve a ofrecer "Eliminar sprint" para reintentar.

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador montado, y así la prueba
// observa a qué ruta regresa el botón al eliminar.
const router = vi.hoisted(() => ({
  push: vi.fn<(href: string) => void>(),
  replace: vi.fn<(href: string) => void>(),
  refresh: vi.fn<() => void>(),
  back: vi.fn<() => void>(),
  forward: vi.fn<() => void>(),
  prefetch: vi.fn<(href: string) => void>(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
  useSearchParams: () => new URLSearchParams(""),
}));

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";
const sprintPath = `/api/v1/sprints/${sprintId}`;
const sprintsListPath = `/projects/${projectId}/sprints`;

const deleteButtonName = "Eliminar sprint";
const confirmButtonName = "Confirmar eliminación";
const confirmQuestion = "¿Eliminar este sprint?";

// Mensajes de contracts/openapi.yaml para los rechazos de la eliminación.
const alreadyStartedMessage =
  "Solo se puede eliminar un sprint que nunca fue iniciado.";
const effortMessage =
  "El sprint tiene esfuerzo registrado. Primero hay que eliminar esos registros.";
const notFoundMessage = "El sprint no existe.";

// Historias comprometidas en el sprint (US7, escenario 2).
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

// Arma el sprint con el estado y las historias indicadas.
function sprintWith(
  status: SprintStatus,
  stories: SprintStoryItem[] = committedStories,
): SprintDetail {
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate: "2026-10-15",
    endDate: "2026-10-28",
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

// Hace que fetch responda una vez con el estado indicado; sin cuerpo para el 204.
function respondOnce(
  fetchMock: FetchMock,
  status: number,
  body?: ErrorResponse,
) {
  fetchMock.mockImplementationOnce(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers:
          body === undefined ? {} : { "Content-Type": "application/json" },
      }),
  );
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
  return {
    path: new URL(raw, "http://localhost").pathname,
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: new Headers(init?.headers).get("Authorization"),
    body: init?.body,
  };
}

// Monta el botón dentro del proveedor de sesión, como en el layout raíz.
function renderButton(sprint: SprintDetail) {
  render(
    <SessionProvider>
      <DeleteSprintButton sprint={sprint} />
    </SessionProvider>,
  );
}

// Pide eliminar y confirma: los dos pasos del flujo de eliminación.
function deleteAndConfirm() {
  fireEvent.click(screen.getByRole("button", { name: deleteButtonName }));
  fireEvent.click(screen.getByRole("button", { name: confirmButtonName }));
}

// Verifica que cada llamada a fetch fue el DELETE del sprint con la sesión y sin cuerpo, y que hubo
// exactamente las indicadas.
function expectDeleteRequests(fetchMock: FetchMock, count: number) {
  expect(fetchMock).toHaveBeenCalledTimes(count);
  for (let index = 0; index < count; index++) {
    const request = requestAt(fetchMock, index);
    expect(request.path).toBe(sprintPath);
    expect(request.method).toBe("DELETE");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request.body).toBeUndefined();
  }
}

// Verifica que el botón regresó al listado de sprints del proyecto sin mostrar ningún rechazo.
async function expectBackToList() {
  await waitFor(() => expect(router.replace).toHaveBeenCalledTimes(1));
  expect(router.replace).toHaveBeenCalledWith(sprintsListPath);
  expect(router.push).not.toHaveBeenCalled();
  expect(screen.queryByRole("alert")).toBeNull();
}

// Verifica que la alerta muestra el mensaje indicado, que el botón no regresó al listado y que la
// eliminación vuelve a ofrecerse para reintentar.
async function expectRejectedWith(message: string) {
  const alert = await screen.findByRole("alert");
  expect(alert.textContent).toContain(message);
  expect(router.replace).not.toHaveBeenCalled();
  expect(router.push).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: deleteButtonName })).not.toBeNull();
}

describe("004/US7 - Corregir o descartar un sprint planificado", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña, un fetch sustituido y el enrutador
  // sin llamadas registradas.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
    vi.clearAllMocks();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // La eliminación solo se ofrece en un sprint Planificado: en uno Activo o Cerrado no aparece el
  // botón ni se envía nada (FR-048).
  it("Eliminar solo se ofrece en un sprint Planificado", () => {
    renderButton(sprintWith("planned"));
    expect(
      screen.getByRole("button", { name: deleteButtonName }),
    ).not.toBeNull();
    cleanup();

    for (const status of ["active", "closed"] as const) {
      renderButton(sprintWith(status));
      expect(
        screen.queryByRole("button", { name: deleteButtonName }),
        status,
      ).toBeNull();
      cleanup();
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // La eliminación es definitiva: el botón pide confirmación antes de enviar nada, y cancelar
  // cierra la confirmación sin eliminar.
  it("Pide confirmación antes de eliminar", () => {
    renderButton(sprintWith("planned"));

    fireEvent.click(screen.getByRole("button", { name: deleteButtonName }));

    expect(screen.getByText(confirmQuestion)).not.toBeNull();
    expect(
      screen.getByRole("button", { name: confirmButtonName }),
    ).not.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancelar" }));

    expect(screen.queryByText(confirmQuestion)).toBeNull();
    expect(
      screen.queryByRole("button", { name: confirmButtonName }),
    ).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  // Caso normal (escenario 2): un sprint Planificado con dos historias se elimina; el botón envía el
  // DELETE y con el 204 regresa al listado de sprints. Devolver las historias al backlog y liberar
  // el período lo resuelve el backend.
  it("Eliminación con historias", async () => {
    respondOnce(fetchMock, 204);
    renderButton(sprintWith("planned"));

    deleteAndConfirm();

    await expectBackToList();
    expectDeleteRequests(fetchMock, 1);
  });

  // Caso límite (escenario 6): un sprint Planificado sin historias se elimina igual y regresa al
  // listado.
  it("Eliminación sin historias", async () => {
    respondOnce(fetchMock, 204);
    renderButton(sprintWith("planned", []));

    deleteAndConfirm();

    await expectBackToList();
    expectDeleteRequests(fetchMock, 1);
  });

  // Caso límite (escenario 8): el primer intento se rechaza por esfuerzo registrado; después de que
  // se elimina ese registro, un nuevo intento desde el mismo botón se acepta y regresa al listado.
  it("Esfuerzo eliminado antes", async () => {
    respondOnce(fetchMock, 409, { error: effortMessage });
    respondOnce(fetchMock, 204);
    renderButton(sprintWith("planned"));

    deleteAndConfirm();
    await expectRejectedWith(effortMessage);

    deleteAndConfirm();
    await waitFor(() => expect(router.replace).toHaveBeenCalledTimes(1));
    expect(router.replace).toHaveBeenCalledWith(sprintsListPath);
    expectDeleteRequests(fetchMock, 2);
  });

  // Caso de error (escenario 13): si el sprint mostrado como Planificado ya fue iniciado, el backend
  // responde 409 y el botón muestra que solo se elimina un sprint que nunca fue iniciado, sin
  // regresar al listado.
  it("Eliminar activo o cerrado", async () => {
    respondOnce(fetchMock, 409, { error: alreadyStartedMessage });
    renderButton(sprintWith("planned"));

    deleteAndConfirm();

    await expectRejectedWith(alreadyStartedMessage);
    expectDeleteRequests(fetchMock, 1);
  });

  // Caso de error (escenario 14): con esfuerzo registrado sobre sus historias la eliminación se
  // rechaza y el mensaje explica qué hay que hacer primero: eliminar esos registros (RN-20).
  it("Esfuerzo registrado", async () => {
    respondOnce(fetchMock, 409, { error: effortMessage });
    renderButton(sprintWith("planned"));

    deleteAndConfirm();

    await expectRejectedWith(effortMessage);
    expect(screen.getByRole("alert").textContent).toContain(
      "Primero hay que eliminar esos registros.",
    );
    expectDeleteRequests(fetchMock, 1);
  });

  // Caso de error (escenario 15): un sprint ya eliminado responde 404 de sprint inexistente y el
  // botón muestra ese mensaje sin regresar al listado.
  it("Sprint eliminado", async () => {
    respondOnce(fetchMock, 404, { error: notFoundMessage });
    renderButton(sprintWith("planned"));

    deleteAndConfirm();

    await expectRejectedWith(notFoundMessage);
    expectDeleteRequests(fetchMock, 1);
  });
});

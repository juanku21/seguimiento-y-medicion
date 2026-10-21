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
import { DeleteStoryButton } from "@/features/backlog/components/delete-story-button";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";

// Pruebas del botón de eliminación de una historia (T068). Atraviesan el componente, la llamada de
// eliminación de api.ts (T072) y el cliente HTTP real; solo se sustituyen fetch, que es la frontera
// con el backend, y el enrutador de Next.js. Las reglas (historial de sprint, esfuerzo, defectos,
// sesiones de estimación, proyecto Finalizado, historia ajena o eliminada) las resuelve el backend:
// acá se verifica qué envía el botón y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para T072:
// - DeleteStoryButton recibe storyId y projectId. Muestra el botón "Eliminar historia", que no envía
//   nada: primero pide confirmación con la pregunta "¿Eliminar esta historia?" y los botones
//   "Confirmar eliminación" y "Cancelar". Cancelar cierra la confirmación sin enviar nada.
// - "Confirmar eliminación" envía DELETE /api/v1/stories/{storyId} con la sesión de la pestaña.
// - Con un 204 regresa al listado del backlog con router.replace("/projects/{projectId}/backlog"),
//   para que volver atrás no lleve al detalle de una historia que ya no existe.
// - Con un 409 de historial (DeletionBlockedResponse) muestra con role="alert" el mensaje general y
//   una lista con **cada** motivo, con el mensaje que envía el backend y en el orden recibido.
// - Con un 409 de solo lectura o un 404 (ErrorResponse) muestra el mensaje con role="alert", sin
//   lista de motivos. En ningún rechazo regresa al listado.

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

// Códigos de los motivos de bloqueo de la eliminación (data-model.md sección 7).
type DeletionBlockedCode =
  "sprint_history" | "effort" | "defects" | "estimation_sessions";

// Cuerpo del 409 por historial asociado, según contracts/openapi.yaml.
type DeletionBlockedBody = {
  error: string;
  reasons: { code: DeletionBlockedCode; message: string }[];
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const projectId = "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35";
const storyId = "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f";
const backlogPath = `/projects/${projectId}/backlog`;

// Mensajes de contracts/openapi.yaml para los rechazos de la eliminación.
const blockedMessage =
  "La historia tiene historial asociado y no se puede eliminar.";
const readOnlyMessage =
  "El proyecto está finalizado y su backlog es de solo lectura.";
const notFoundMessage = "La historia no existe.";

// Mensajes de cada motivo de bloqueo, en el orden de T005 (data-model.md sección 7).
const reasonMessages: Record<DeletionBlockedCode, string> = {
  sprint_history: "Estuvo asignada a un sprint.",
  effort: "Tiene esfuerzo registrado.",
  defects: "Tiene defectos asociados.",
  estimation_sessions: "Tiene sesiones de estimación por Planning Poker.",
};

// Arma el cuerpo del 409 por historial con los motivos indicados, en el orden recibido.
function blocked(...codes: DeletionBlockedCode[]): DeletionBlockedBody {
  return {
    error: blockedMessage,
    reasons: codes.map((code) => ({ code, message: reasonMessages[code] })),
  };
}

// Hace que fetch responda una vez con el estado indicado; sin cuerpo para el 204.
function respondOnce(
  fetchMock: FetchMock,
  status: number,
  body?: ErrorResponse | DeletionBlockedBody,
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

// Extrae la URL, el método y la cabecera Authorization de la última llamada a fetch.
function lastRequest(fetchMock: FetchMock) {
  const [input, init] = fetchMock.mock.calls[fetchMock.mock.calls.length - 1];
  const url =
    input instanceof Request
      ? input.url
      : input instanceof URL
        ? input.href
        : input;
  return {
    url,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Monta el botón de eliminación de la historia del proyecto de prueba.
function renderButton() {
  render(<DeleteStoryButton storyId={storyId} projectId={projectId} />);
}

// Pide eliminar y confirma: los dos pasos del flujo de eliminación.
function deleteAndConfirm() {
  fireEvent.click(screen.getByRole("button", { name: "Eliminar historia" }));
  fireEvent.click(
    screen.getByRole("button", { name: "Confirmar eliminación" }),
  );
}

// Verifica que se envió una única vez el DELETE de la historia con la sesión de la pestaña.
function expectDeleteRequest(fetchMock: FetchMock) {
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const request = lastRequest(fetchMock);
  expect(request.url.endsWith(`/api/v1/stories/${storyId}`)).toBe(true);
  expect(request.method).toBe("DELETE");
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
}

// Espera la alerta del rechazo y devuelve los textos de los motivos listados, en orden.
async function rejectionReasons(): Promise<string[]> {
  const alert = await screen.findByRole("alert");
  return within(alert)
    .queryAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

// Verifica que se envió el DELETE, que la alerta muestra el mensaje general y exactamente los
// motivos indicados en ese orden, y que el botón no regresó al listado.
async function expectBlocked(
  fetchMock: FetchMock,
  ...codes: DeletionBlockedCode[]
) {
  const reasons = await rejectionReasons();
  expect(screen.getByRole("alert").textContent).toContain(blockedMessage);
  expect(reasons).toEqual(codes.map((code) => reasonMessages[code]));
  expectDeleteRequest(fetchMock);
  expect(router.replace).not.toHaveBeenCalled();
  expect(router.push).not.toHaveBeenCalled();
}

// Verifica que se envió el DELETE, que la alerta muestra el mensaje indicado sin lista de motivos y
// que el botón no regresó al listado.
async function expectRejectedWith(fetchMock: FetchMock, message: string) {
  const reasons = await rejectionReasons();
  expect(screen.getByRole("alert").textContent).toContain(message);
  expect(reasons).toEqual([]);
  expectDeleteRequest(fetchMock);
  expect(router.replace).not.toHaveBeenCalled();
  expect(router.push).not.toHaveBeenCalled();
}

// Verifica que se envió el DELETE y que el botón regresó al listado del backlog.
async function expectDeleted(fetchMock: FetchMock) {
  await waitFor(() => expect(router.replace).toHaveBeenCalledTimes(1));
  expect(router.replace).toHaveBeenCalledWith(backlogPath);
  expectDeleteRequest(fetchMock);
  expect(screen.queryByRole("alert")).toBeNull();
}

describe("003/US6 - Eliminar una historia que nunca se trabajó", () => {
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

  // Caso normal: una historia recién creada sin historial se elimina; el botón envía el DELETE y
  // regresa al backlog, donde la historia ya no aparece (FR-041, FR-044).
  it("Eliminación exitosa", async () => {
    respondOnce(fetchMock, 204);
    renderButton();

    deleteAndConfirm();

    await expectDeleted(fetchMock);
  });

  // Caso alternativo: estar lista para planificar no impide eliminarla; el botón no decide por el
  // estado de la historia y con el 204 regresa al backlog.
  it("Lista para planificar se elimina", async () => {
    respondOnce(fetchMock, 204);
    renderButton();

    deleteAndConfirm();

    await expectDeleted(fetchMock);
  });

  // Caso alternativo: cualquier integrante puede eliminar cualquier historia (FR-042); el botón se
  // ofrece sin depender del rol ni del autor, y con el 204 regresa al backlog.
  it("Integrante no propietario", async () => {
    respondOnce(fetchMock, 204);
    renderButton();

    deleteAndConfirm();

    await expectDeleted(fetchMock);
  });

  // Caso límite: una historia que estuvo en un sprint y fue quitada no se elimina; el botón muestra
  // el motivo de historial de sprint y no regresa al listado.
  it("Quitada de un sprint", async () => {
    respondOnce(fetchMock, 409, blocked("sprint_history"));
    renderButton();

    deleteAndConfirm();

    await expectBlocked(fetchMock, "sprint_history");
  });

  // Caso límite: eliminar la única historia se acepta y regresa al backlog, que lo informará como
  // backlog sin historias (el listado vacío lo cubren las pruebas de US2).
  it("Única historia", async () => {
    respondOnce(fetchMock, 204);
    renderButton();

    deleteAndConfirm();

    await expectDeleted(fetchMock);
  });

  // Caso de error: con esfuerzo registrado o defectos asociados se rechaza indicando qué historial
  // lo impide; con los dos a la vez se listan los dos, en el orden recibido (FR-043).
  it("Esfuerzo o defectos", async () => {
    respondOnce(fetchMock, 409, blocked("effort"));
    renderButton();
    deleteAndConfirm();
    await expectBlocked(fetchMock, "effort");
    cleanup();
    fetchMock.mockClear();

    respondOnce(fetchMock, 409, blocked("defects"));
    renderButton();
    deleteAndConfirm();
    await expectBlocked(fetchMock, "defects");
    cleanup();
    fetchMock.mockClear();

    respondOnce(fetchMock, 409, blocked("effort", "defects"));
    renderButton();
    deleteAndConfirm();
    await expectBlocked(fetchMock, "effort", "defects");
  });

  // Caso de error: con sesiones de Planning Poker se rechaza indicando el historial de estimación.
  it("Sesiones de estimación", async () => {
    respondOnce(fetchMock, 409, blocked("estimation_sessions"));
    renderButton();

    deleteAndConfirm();

    await expectBlocked(fetchMock, "estimation_sessions");
  });

  // Caso de error: en un proyecto Finalizado el backlog es de solo lectura; el botón muestra el
  // motivo y no regresa al listado (FR-048, RN-15).
  it("Proyecto finalizado", async () => {
    respondOnce(fetchMock, 409, { error: readOnlyMessage });
    renderButton();

    deleteAndConfirm();

    await expectRejectedWith(fetchMock, readOnlyMessage);
  });

  // Caso de error: la historia de un proyecto ajeno responde como inexistente (FR-047).
  it("Historia ajena", async () => {
    respondOnce(fetchMock, 404, { error: notFoundMessage });
    renderButton();

    deleteAndConfirm();

    await expectRejectedWith(fetchMock, notFoundMessage);
  });

  // Caso de error: una historia ya eliminada responde como inexistente (FR-044).
  it("Ya eliminada", async () => {
    respondOnce(fetchMock, 404, { error: notFoundMessage });
    renderButton();

    deleteAndConfirm();

    await expectRejectedWith(fetchMock, notFoundMessage);
  });

  // La eliminación es definitiva: el botón pide confirmación antes de enviar nada, y cancelar
  // cierra la confirmación sin eliminar.
  it("Pide confirmación antes de eliminar", () => {
    renderButton();

    fireEvent.click(screen.getByRole("button", { name: "Eliminar historia" }));

    expect(screen.getByText("¿Eliminar esta historia?")).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Confirmar eliminación" }),
    ).not.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancelar" }));

    expect(screen.queryByText("¿Eliminar esta historia?")).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Confirmar eliminación" }),
    ).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  // El rechazo muestra cada motivo que informa el backend: con los cuatro a la vez se listan los
  // cuatro, en el orden de T005 (FR-043, RN-16).
  it("Muestra cada motivo del rechazo", async () => {
    respondOnce(
      fetchMock,
      409,
      blocked("sprint_history", "effort", "defects", "estimation_sessions"),
    );
    renderButton();

    deleteAndConfirm();

    await expectBlocked(
      fetchMock,
      "sprint_history",
      "effort",
      "defects",
      "estimation_sessions",
    );
  });
});

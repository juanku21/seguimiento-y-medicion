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
import { SprintEditView } from "@/features/sprints/components/sprint-edit-view";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintInput,
  SprintRef,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";

// Pruebas de la vista de modificación de un sprint (T083). Atraviesan la vista, el SprintForm
// precargado, la llamada de modificación de api.ts (T087) y el cliente HTTP real; solo se sustituye
// fetch, que es la frontera con el backend. Las reglas de la modificación (superposición excluyendo
// al propio sprint, sprint no Planificado, sprint inexistente; FR-012 a FR-016) las resuelve el
// backend: acá se verifica cuándo se ofrece el formulario, qué envía y qué muestra con cada
// respuesta.
//
// Contrato que fijan estas pruebas para T087:
// - SprintEditView recibe sprintId y pide el sprint con GET /api/v1/sprints/{sprintId} usando la
//   sesión de la pestaña. Si el GET falla (404 del sprint inexistente o eliminado) muestra solo el
//   mensaje con role="alert", sin formulario.
// - Solo en un sprint Planificado monta el formulario precargado con sus valores: "Nombre",
//   "Sprint Goal", "Fecha de inicio" y "Fecha de fin" (AAAA-MM-DD), y el botón "Guardar cambios".
//   En un sprint Activo muestra con role="alert" "Solo se puede modificar un sprint Planificado." y
//   en uno Cerrado "El sprint está cerrado y no admite cambios.", sin formulario.
// - "Guardar cambios" envía PUT /api/v1/sprints/{sprintId} con la sesión y un cuerpo con
//   **exactamente** name, goal, startDate y endDate (SprintInput): nunca estado ni historias
//   (FR-015). Se envía aunque el nombre no haya cambiado.
// - Las validaciones del formulario son las de la creación: un campo inválido se marca con
//   aria-invalid="true", no se envía nada y no hay cambio parcial (FR-014). Un 400 del backend se
//   ubica junto al campo que informa.
// - Con un 200 confirma "Sprint actualizado" con role="status", con el nombre, el estado y las
//   fechas (DD/MM/AAAA) devueltos por el backend.
// - Con un rechazo (409 o 404) muestra el mensaje del backend con role="alert" y no confirma nada;
//   el 409 de superposición nombra el sprint en conflicto.

// Sustituye el enrutador de Next.js que usa la guardia de sesión: fuera del App Router no hay
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

// Rechazo 409 con el sprint en conflicto (SprintConflictResponse de contracts/openapi.yaml).
type SprintConflictBody = {
  error: string;
  sprint: SprintRef;
};

// Respuesta que el backend simulado da a un pedido.
type Reply = {
  status: number;
  body:
    SprintDetail | ErrorResponse | ValidationErrorResponse | SprintConflictBody;
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";
const sprintPath = `/api/v1/sprints/${sprintId}`;

// Mensajes de contracts/openapi.yaml para los rechazos de la modificación.
const notPlannedMessage = "Solo se puede modificar un sprint Planificado.";
const closedMessage = "El sprint está cerrado y no admite cambios.";
const notFoundMessage = "El sprint no existe.";
const invalidFieldsMessage = "Hay campos inválidos en la solicitud.";
const otherSprint: SprintRef = {
  id: "1d2c3b4a-5f6e-4a7b-8c9d-0e1f2a3b4c5d",
  name: "Sprint 1",
};
const overlapMessage = `El período se superpone con el del sprint «${otherSprint.name}».`;

// Historias comprometidas en el sprint, que la modificación no altera (FR-015).
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

// Valores vigentes del sprint antes de modificarlo.
const currentValues: SprintInput = {
  name: "Sprint 2",
  goal: "Cerrar el alta de clientes",
  startDate: "2026-10-15",
  endDate: "2026-10-28",
};

// Arma el sprint con el estado indicado y los valores vigentes.
function sprintWith(status: SprintStatus): SprintDetail {
  return {
    id: sprintId,
    projectId,
    ...currentValues,
    status,
    startedAt: status === "planned" ? null : "2026-10-15T12:00:00Z",
    closedAt: status === "closed" ? "2026-10-28T20:00:00Z" : null,
    createdBy: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    closedBy: null,
    createdAt: "2026-10-10T12:00:00Z",
    updatedAt: "2026-10-10T12:00:00Z",
    stories: committedStories,
    results: null,
    hoursPerStoryPoint: null,
    schedule: null,
  };
}

// Reconoce el cuerpo de un PUT con los cuatro campos de SprintInput como texto.
function isSprintInput(value: unknown): value is SprintInput {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const fields: Record<string, unknown> = { ...value };
  return ["name", "goal", "startDate", "endDate"].every(
    (key) => typeof fields[key] === "string",
  );
}

// Arma una respuesta JSON con el estado y el cuerpo indicados.
function jsonResponse({ status, body }: Reply): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Lee la ruta, el método, la cabecera Authorization y el cuerpo JSON de una llamada a fetch.
function readCall(input: RequestInfo | URL, init: RequestInit | undefined) {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return {
    path: new URL(raw, "http://localhost").pathname,
    method: init?.method ?? "GET",
    authorization: new Headers(init?.headers).get("Authorization"),
    body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
  };
}

// Simula el backend del sprint: el GET responde getReply (por defecto, el sprint Planificado con
// 200) y cada PUT responde lo que decida onPut a partir del cuerpo recibido.
function serve(
  fetchMock: FetchMock,
  onPut: (body: unknown) => Reply,
  getReply: Reply = { status: 200, body: sprintWith("planned") },
) {
  fetchMock.mockImplementation(async (input, init) => {
    const { method, body } = readCall(input, init);
    if (method === "GET") {
      return jsonResponse(getReply);
    }
    if (method === "PUT") {
      return jsonResponse(onPut(body));
    }
    return jsonResponse({ status: 500, body: { error: "inesperado" } });
  });
}

// Simula un backend que acepta el PUT y devuelve el sprint con los datos recibidos, conservando su
// estado y sus historias (FR-015).
function serveAccepting(fetchMock: FetchMock) {
  serve(fetchMock, (body) => {
    if (!isSprintInput(body)) {
      return { status: 400, body: { error: invalidFieldsMessage } };
    }
    return {
      status: 200,
      body: {
        ...sprintWith("planned"),
        name: body.name,
        goal: body.goal,
        startDate: body.startDate,
        endDate: body.endDate,
        updatedAt: "2026-10-12T09:00:00Z",
      },
    };
  });
}

// Simula un backend que rechaza cada PUT con la respuesta indicada.
function serveRejecting(fetchMock: FetchMock, reply: Reply) {
  serve(fetchMock, () => reply);
}

// Devuelve los PUT enviados, en orden.
function putRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .map(([input, init]) => readCall(input, init))
    .filter((call) => call.method === "PUT");
}

// Verifica que se envió un único PUT del sprint, con la sesión, y devuelve su cuerpo.
function singlePutBody(fetchMock: FetchMock): unknown {
  const requests = putRequests(fetchMock);
  expect(requests).toHaveLength(1);
  const [request] = requests;
  expect(request.path).toBe(sprintPath);
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  return request.body;
}

// Monta la vista dentro del proveedor de sesión, como en el layout raíz.
function renderEditView() {
  render(
    <SessionProvider>
      <SprintEditView sprintId={sprintId} />
    </SessionProvider>,
  );
}

// Monta la vista y espera a que el formulario precargado esté disponible; devuelve sus campos.
async function openForm() {
  renderEditView();
  await screen.findByRole("button", { name: "Guardar cambios" });
  return {
    name: screen.getByLabelText<HTMLInputElement>(/^nombre/i),
    goal: screen.getByLabelText<HTMLInputElement | HTMLTextAreaElement>(
      /^sprint goal/i,
    ),
    startDate: screen.getByLabelText<HTMLInputElement>(/^fecha de inicio/i),
    endDate: screen.getByLabelText<HTMLInputElement>(/^fecha de fin/i),
  };
}

type Fields = Awaited<ReturnType<typeof openForm>>;

// Reemplaza los valores indicados en el formulario y guarda los cambios.
function changeAndSave(fields: Fields, changes: Partial<SprintInput>) {
  const order = ["name", "goal", "startDate", "endDate"] as const;
  for (const field of order) {
    const value = changes[field];
    if (value !== undefined) {
      fireEvent.change(fields[field], { target: { value } });
    }
  }
  fireEvent.click(screen.getByRole("button", { name: "Guardar cambios" }));
}

// Devuelve el texto de todos los elementos que describen a un campo vía aria-describedby.
function describedText(element: HTMLElement): string {
  const ids = (element.getAttribute("aria-describedby") ?? "")
    .split(/\s+/)
    .filter(Boolean);
  return ids
    .map((id) => document.getElementById(id)?.textContent ?? "")
    .join(" ");
}

// Espera la confirmación de la modificación y devuelve su texto completo.
async function confirmationText(): Promise<string> {
  const title = await screen.findByText("Sprint actualizado");
  const confirmation = title.closest('[role="status"]');
  expect(confirmation).not.toBeNull();
  return confirmation?.textContent ?? "";
}

// Verifica que la vista no ofrece el formulario de modificación.
function expectNoForm() {
  expect(screen.queryByRole("button", { name: "Guardar cambios" })).toBeNull();
  expect(screen.queryByLabelText(/^nombre/i)).toBeNull();
}

describe("004/US7 - Corregir o descartar un sprint planificado", () => {
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

  // Caso normal (escenario 1): el formulario arranca con los valores vigentes; al cambiar nombre,
  // fechas y Sprint Goal envía solo esos cuatro campos, sin estado ni historias, y confirma el
  // sprint actualizado, que sigue Planificado (FR-012, FR-015).
  it("Modificación completa", async () => {
    serveAccepting(fetchMock);
    const fields = await openForm();
    expect(fields.name.value).toBe(currentValues.name);
    expect(fields.goal.value).toBe(currentValues.goal);
    expect(fields.startDate.value).toBe(currentValues.startDate);
    expect(fields.endDate.value).toBe(currentValues.endDate);

    const changes: SprintInput = {
      name: "Sprint 2 - Clientes",
      goal: "Publicar el alta y la baja de clientes",
      startDate: "2026-10-16",
      endDate: "2026-10-30",
    };
    changeAndSave(fields, changes);

    const text = await confirmationText();
    expect(text).toContain(changes.name);
    expect(text).toContain("Planificado");
    expect(singlePutBody(fetchMock)).toEqual(changes);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo (escenario 3): con historias comprometidas, correr las fechas a otro período
  // libre envía las fechas nuevas y la confirmación las muestra.
  it("Mover fechas", async () => {
    serveAccepting(fetchMock);
    const fields = await openForm();

    changeAndSave(fields, { startDate: "2026-11-02", endDate: "2026-11-15" });

    const text = await confirmationText();
    expect(text).toContain("02/11/2026");
    expect(text).toContain("15/11/2026");
    expect(singlePutBody(fetchMock)).toEqual({
      ...currentValues,
      startDate: "2026-11-02",
      endDate: "2026-11-15",
    });
  });

  // Caso límite (escenario 4): guardar con el mismo nombre que ya tenía no se bloquea como cambio
  // vacío; se envía y se confirma.
  it("Mismo nombre", async () => {
    serveAccepting(fetchMock);
    const fields = await openForm();

    changeAndSave(fields, { name: currentValues.name });

    const text = await confirmationText();
    expect(text).toContain(currentValues.name);
    expect(singlePutBody(fetchMock)).toEqual(currentValues);
  });

  // Caso límite (escenario 5): mover el sprint para que empiece el día en que termina el anterior no
  // se bloquea en el formulario; se envía y se confirma.
  it("Fechas contiguas", async () => {
    serveAccepting(fetchMock);
    const fields = await openForm();

    changeAndSave(fields, { startDate: "2026-10-14", endDate: "2026-10-27" });

    await confirmationText();
    expect(singlePutBody(fetchMock)).toEqual({
      ...currentValues,
      startDate: "2026-10-14",
      endDate: "2026-10-27",
    });
  });

  // Caso de error (escenario 9): nombre o Sprint Goal vacíos, fin igual o anterior al inicio y un
  // Sprint Goal de 501 caracteres se rechazan antes de enviar, junto al campo y sin cambio parcial;
  // un 400 del backend se ubica junto al campo que informa.
  it("Datos inválidos", async () => {
    serveAccepting(fetchMock);
    const cases: Array<
      [string, Partial<SprintInput>, keyof SprintInput, RegExp]
    > = [
      ["nombre vacío", { name: "   " }, "name", /nombre/i],
      ["Sprint Goal vacío", { goal: "" }, "goal", /sprint goal/i],
      [
        "fin igual al inicio",
        { startDate: "2026-10-15", endDate: "2026-10-15" },
        "endDate",
        /posterior/i,
      ],
      [
        "fin anterior al inicio",
        { startDate: "2026-10-15", endDate: "2026-10-01" },
        "endDate",
        /posterior/i,
      ],
      ["Sprint Goal de 501", { goal: "g".repeat(501) }, "goal", /500/],
    ];
    for (const [label, changes, field, motive] of cases) {
      const fields = await openForm();
      changeAndSave(fields, changes);
      await waitFor(() =>
        expect(fields[field].getAttribute("aria-invalid"), label).toBe("true"),
      );
      expect(describedText(fields[field]), label).toMatch(motive);
      expect(putRequests(fetchMock), label).toHaveLength(0);
      expect(screen.queryByText("Sprint actualizado"), label).toBeNull();
      cleanup();
    }

    serveRejecting(fetchMock, {
      status: 400,
      body: {
        error: invalidFieldsMessage,
        fields: [{ field: "goal", message: "El Sprint Goal es obligatorio." }],
      },
    });
    const fields = await openForm();
    changeAndSave(fields, {});
    await waitFor(() =>
      expect(fields.goal.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.goal)).toContain(
      "El Sprint Goal es obligatorio.",
    );
    expect(screen.queryByText("Sprint actualizado")).toBeNull();
  });

  // Caso de error (escenario 10): si el período nuevo se superpone con otro sprint, el backend
  // responde 409 y la vista indica con cuál, sin confirmar la modificación.
  it("Superposición", async () => {
    serveRejecting(fetchMock, {
      status: 409,
      body: { error: overlapMessage, sprint: otherSprint },
    });
    const fields = await openForm();

    changeAndSave(fields, { startDate: "2026-10-05", endDate: "2026-10-20" });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(otherSprint.name);
    expect(putRequests(fetchMock)).toHaveLength(1);
    expect(screen.queryByText("Sprint actualizado")).toBeNull();
  });

  // Caso de error (escenario 11): en un sprint Activo la modificación no se ofrece y se indica que
  // solo se modifica un sprint Planificado; si el sprint arrancó después de abrir el formulario, el
  // 409 del backend muestra el mismo motivo.
  it("Modificar activo", async () => {
    serve(fetchMock, () => ({ status: 500, body: { error: "inesperado" } }), {
      status: 200,
      body: sprintWith("active"),
    });
    renderEditView();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notPlannedMessage);
    expectNoForm();
    expect(putRequests(fetchMock)).toHaveLength(0);
    cleanup();

    serveRejecting(fetchMock, {
      status: 409,
      body: { error: notPlannedMessage },
    });
    const fields = await openForm();
    changeAndSave(fields, { name: "Sprint 2 - Clientes" });
    const rejection = await screen.findByRole("alert");
    expect(rejection.textContent).toContain(notPlannedMessage);
    expect(screen.queryByText("Sprint actualizado")).toBeNull();
  });

  // Caso de error (escenario 12): en un sprint Cerrado la modificación no se ofrece y se indica que
  // un sprint cerrado no admite cambios; el 409 del backend muestra el mismo motivo.
  it("Modificar cerrado", async () => {
    serve(fetchMock, () => ({ status: 500, body: { error: "inesperado" } }), {
      status: 200,
      body: sprintWith("closed"),
    });
    renderEditView();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(closedMessage);
    expectNoForm();
    expect(putRequests(fetchMock)).toHaveLength(0);
    cleanup();

    serveRejecting(fetchMock, { status: 409, body: { error: closedMessage } });
    const fields = await openForm();
    changeAndSave(fields, { name: "Sprint 2 - Clientes" });
    const rejection = await screen.findByRole("alert");
    expect(rejection.textContent).toContain(closedMessage);
    expect(screen.queryByText("Sprint actualizado")).toBeNull();
  });

  // Caso de error (escenario 15): un sprint eliminado responde 404 de sprint inexistente; la vista
  // muestra solo ese mensaje, sin formulario, y si se eliminó después de abrirla el PUT muestra el
  // mismo mensaje.
  it("Sprint eliminado", async () => {
    serve(fetchMock, () => ({ status: 500, body: { error: "inesperado" } }), {
      status: 404,
      body: { error: notFoundMessage },
    });
    renderEditView();
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notFoundMessage);
    expectNoForm();
    cleanup();

    serveRejecting(fetchMock, {
      status: 404,
      body: { error: notFoundMessage },
    });
    const fields = await openForm();
    changeAndSave(fields, { name: "Sprint 2 - Clientes" });
    const rejection = await screen.findByRole("alert");
    expect(rejection.textContent).toContain(notFoundMessage);
    expect(screen.queryByText("Sprint actualizado")).toBeNull();
  });
});

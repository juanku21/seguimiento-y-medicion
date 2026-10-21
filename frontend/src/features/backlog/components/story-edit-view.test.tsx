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
  queries,
  waitFor,
  type BoundFunctions,
  within,
} from "@testing-library/react";
import { StoryEditView } from "@/features/backlog/components/story-edit-view";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type { StoryDetail, StoryPriority } from "@/types/story";

// Pruebas de la vista de refinamiento de una historia (T060). Atraviesan la vista, el StoryForm
// precargado, la llamada de modificación de api.ts y el cliente HTTP real; solo se sustituye fetch,
// que es la frontera con el backend. Las reglas (historia Completada, comprometida, proyecto
// Finalizado, historia ajena) las resuelve el backend: acá se verifica qué envía la vista y qué
// muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para T065:
// - StoryEditView recibe storyId, pide la historia con GET /api/v1/stories/{storyId} usando la sesión
//   de la pestaña y monta el formulario precargado con sus valores: "Título", "Descripción",
//   "Prioridad" (selector) y un campo "Criterio N" por criterio, en orden. Si el GET falla (404 de la
//   historia ajena o inexistente) muestra solo el mensaje con role="alert", sin formulario.
// - El formulario no ofrece cambiar el estado ni los Story Points: el único selector es "Prioridad"
//   y no hay control ni acción de estado (FR-027; la estimación tiene su propio control, US4).
// - El botón "Guardar cambios" envía PATCH /api/v1/stories/{storyId} con la sesión y un cuerpo que
//   incluye **solo los campos que la persona cambió** (FR-020, FR-025): title, description,
//   priority o acceptanceCriteria. La lista de criterios, si cambió, viaja completa (FR-022), y una
//   lista vaciada viaja como []. Nunca viajan status, storyPoints ni los campos de solo lectura.
// - Las validaciones del formulario son las de la creación: un campo inválido se marca con
//   aria-invalid="true", no se envía nada y no hay cambio parcial (FR-021, FR-023).
// - Con un 200 se confirma "Historia actualizada" con role="status", el título y el estado devueltos
//   por el backend y si quedó lista para planificar.
// - Con un rechazo se muestra el motivo con role="alert" (en un 400, también el detalle junto al
//   campo) y no se confirma nada.

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

// Ámbito de las consultas: la pantalla entera o el contenedor de una de las vistas montadas.
type Scope = BoundFunctions<typeof queries>;

// Cuerpo del PATCH tal como lo envía la vista; se lee sin suponer su forma.
type PatchBody = Record<string, unknown>;

// Respuesta que el backend simulado da a un pedido.
type Reply = {
  status: number;
  body: StoryDetail | ErrorResponse | ValidationErrorResponse;
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";

// Mensajes de contracts/openapi.yaml para los rechazos de la modificación.
const notFoundMessage = "La historia no existe.";
const committedMessage =
  "La historia está comprometida en un sprint abierto; primero hay que quitarla del sprint.";
const completedMessage =
  "La historia está completada y no se puede modificar ni reestimar.";
const readOnlyMessage =
  "El proyecto está finalizado y su backlog es de solo lectura.";
const invalidFieldsMessage = "Hay campos inválidos en la solicitud.";

const firstCriterion =
  "El PDF incluye la velocidad de los últimos cinco sprints.";
const secondCriterion = "Al pie figura la fecha de generación.";
const thirdCriterion = "El archivo se descarga con el nombre del proyecto.";

// Historia Pendiente, estimada y con dos criterios, creada por Ana (US5-1).
const pendingStory: StoryDetail = {
  id: "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f",
  projectId: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
  title: "Exportar el reporte de velocidad a PDF",
  description: "El equipo necesita compartir la velocidad con la gerencia.",
  priority: "low",
  status: "pending",
  storyPoints: 5,
  acceptanceCriteria: [firstCriterion, secondCriterion],
  readiness: { ready: true, missing: [] },
  createdBy: {
    id: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
    fullName: "Ana Pérez",
  },
  createdAt: "2026-10-10T14:30:00Z",
  updatedAt: "2026-10-10T14:30:00Z",
  everAssignedToSprint: false,
};

// Reconoce un cuerpo de PATCH: un objeto JSON, no una lista ni null.
function isPatchBody(value: unknown): value is PatchBody {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

// Reconoce una prioridad de la API en un valor recibido sin forma conocida.
function isPriority(value: unknown): value is StoryPriority {
  return value === "high" || value === "medium" || value === "low";
}

// Reconoce una lista de criterios en un valor recibido sin forma conocida.
function isCriteria(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === "string")
  );
}

// Arma la respuesta 200 del backend: aplica solo los campos presentes en el cuerpo, como el PATCH
// real, recalcula la condición de lista para planificar y avanza el momento de actualización. El
// identificador, el estado, el autor y la creación se conservan (FR-024).
function updated(
  story: StoryDetail,
  body: PatchBody,
  updatedAt = "2026-10-16T11:30:00Z",
): StoryDetail {
  const next: StoryDetail = { ...story, updatedAt };
  if (typeof body.title === "string") {
    next.title = body.title;
  }
  if ("description" in body) {
    const description = body.description;
    next.description =
      typeof description === "string" && description.trim() !== ""
        ? description
        : null;
  }
  if (isPriority(body.priority)) {
    next.priority = body.priority;
  }
  if (isCriteria(body.acceptanceCriteria)) {
    next.acceptanceCriteria = body.acceptanceCriteria;
  }
  const missing: StoryDetail["readiness"]["missing"] = [];
  if (next.storyPoints === null) {
    missing.push("story_points");
  }
  if (next.acceptanceCriteria.length === 0) {
    missing.push("acceptance_criteria");
  }
  next.readiness = { ready: missing.length === 0, missing };
  return next;
}

// Arma una respuesta JSON con el estado y el cuerpo indicados.
function jsonResponse({ status, body }: Reply): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Lee el método y el cuerpo JSON de una llamada a fetch.
function readCall(init: RequestInit | undefined) {
  return {
    method: init?.method ?? "GET",
    body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
  };
}

// Simula el backend de la historia: el GET responde getReply (por defecto, la historia con 200) y
// cada PATCH responde lo que decida onPatch a partir del cuerpo recibido.
function serve(
  fetchMock: FetchMock,
  story: StoryDetail,
  onPatch: (body: PatchBody) => Reply,
  getReply: Reply = { status: 200, body: story },
) {
  fetchMock.mockImplementation(async (_input, init) => {
    const { method, body } = readCall(init);
    if (method === "GET") {
      return jsonResponse(getReply);
    }
    if (method === "PATCH") {
      return jsonResponse(onPatch(body));
    }
    return jsonResponse({ status: 500, body: { error: "inesperado" } });
  });
}

// Simula un backend que aplica cada PATCH sobre la historia vigente, campo por campo, y devuelve la
// historia resultante. Devuelve la lectura del estado vigente, para verificar qué quedó guardado.
function serveApplying(fetchMock: FetchMock, story: StoryDetail) {
  let current = story;
  serve(fetchMock, story, (body) => {
    current = updated(current, body);
    return { status: 200, body: current };
  });
  return () => current;
}

// Devuelve los PATCH enviados, en orden, con su URL, la cabecera Authorization y el cuerpo.
function patchRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .filter(([, init]) => readCall(init).method === "PATCH")
    .map(([input, init]) => ({
      url:
        input instanceof Request
          ? input.url
          : input instanceof URL
            ? input.href
            : input,
      authorization: new Headers(init?.headers).get("Authorization"),
      body: readCall(init).body,
    }));
}

// Verifica que se envió un único PATCH de la historia, con la sesión, y devuelve su cuerpo.
function singlePatchBody(fetchMock: FetchMock, storyId: string): unknown {
  const requests = patchRequests(fetchMock);
  expect(requests).toHaveLength(1);
  const [request] = requests;
  expect(request.url.endsWith(`/api/v1/stories/${storyId}`)).toBe(true);
  expect(request.authorization).toBe(`Bearer ${sessionToken}`);
  return request.body;
}

// Monta una vista de refinamiento dentro del proveedor de sesión y devuelve su ámbito de consultas.
function renderEditView(storyId: string): Scope {
  const { container } = render(
    <SessionProvider>
      <StoryEditView storyId={storyId} />
    </SessionProvider>,
  );
  return within(container);
}

// Monta la vista y espera a que el formulario precargado esté disponible.
async function openForm(storyId: string): Promise<Scope> {
  const scope = renderEditView(storyId);
  await scope.findByLabelText("Título");
  return scope;
}

// Devuelve el control de formulario con la etiqueta indicada.
function control(
  scope: Scope,
  label: string,
): HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement {
  const element = scope.getByLabelText(label);
  if (
    element instanceof HTMLInputElement ||
    element instanceof HTMLTextAreaElement ||
    element instanceof HTMLSelectElement
  ) {
    return element;
  }
  throw new Error(`${label} debe ser un campo de formulario`);
}

// Escribe o elige un valor en el control con la etiqueta indicada.
function change(scope: Scope, label: string, value: string) {
  fireEvent.change(control(scope, label), { target: { value } });
}

// Guarda los cambios del formulario.
function save(scope: Scope) {
  fireEvent.click(scope.getByRole("button", { name: "Guardar cambios" }));
}

// Espera la confirmación de la modificación y devuelve su texto completo.
async function confirmationText(scope: Scope): Promise<string> {
  const title = await scope.findByText("Historia actualizada");
  const confirmation = title.closest('[role="status"]');
  expect(confirmation).not.toBeNull();
  return confirmation?.textContent ?? "";
}

describe("003/US5 - Refinar una historia", () => {
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

  // Caso normal: el formulario arranca con los valores vigentes; al cambiar título, descripción,
  // prioridad y criterios se envían esos cuatro campos, y nada más, y se confirma la historia
  // devuelta, que conserva su estado (FR-020, FR-024).
  it("Refinamiento completo", async () => {
    serveApplying(fetchMock, pendingStory);
    const view = await openForm(pendingStory.id);
    expect(control(view, "Título").value).toBe(pendingStory.title);
    expect(control(view, "Descripción").value).toBe(pendingStory.description);
    expect(control(view, "Prioridad").value).toBe("low");
    expect(control(view, "Criterio 1").value).toBe(firstCriterion);
    expect(control(view, "Criterio 2").value).toBe(secondCriterion);

    const title = "Exportar a PDF el reporte de velocidad del equipo";
    const description = "Incluye los sprints cerrados del último trimestre.";
    change(view, "Título", title);
    change(view, "Descripción", description);
    change(view, "Prioridad", "high");
    change(view, "Criterio 2", thirdCriterion);
    fireEvent.click(view.getByRole("button", { name: "Agregar criterio" }));
    change(view, "Criterio 3", secondCriterion);
    save(view);

    const text = await confirmationText(view);
    expect(text).toContain(title);
    expect(text).toContain("Pendiente");
    expect(singlePatchBody(fetchMock, pendingStory.id)).toEqual({
      title,
      description,
      priority: "high",
      acceptanceCriteria: [firstCriterion, thirdCriterion, secondCriterion],
    });
    expect(view.queryByRole("alert")).toBeNull();
  });

  // Caso alternativo: la historia la creó otro integrante; la vista igual ofrece el formulario y
  // envía el cambio, porque cualquier integrante puede refinar cualquier historia (FR-020).
  it("Historia de otro integrante", async () => {
    const story: StoryDetail = {
      ...pendingStory,
      createdBy: {
        id: "1e2f3a4b-5c6d-4e7f-8a9b-0c1d2e3f4a5b",
        fullName: "Carla Gómez",
      },
    };
    serveApplying(fetchMock, story);
    const view = await openForm(story.id);

    change(view, "Prioridad", "medium");
    save(view);

    await confirmationText(view);
    expect(singlePatchBody(fetchMock, story.id)).toEqual({
      priority: "medium",
    });
  });

  // Caso alternativo: con tres criterios, reordenarlos envía la lista completa en el nuevo orden,
  // y solo la lista (FR-022).
  it("Reorden de criterios", async () => {
    const story: StoryDetail = {
      ...pendingStory,
      acceptanceCriteria: [firstCriterion, secondCriterion, thirdCriterion],
    };
    serveApplying(fetchMock, story);
    const view = await openForm(story.id);

    fireEvent.click(view.getByRole("button", { name: "Bajar criterio 1" }));
    save(view);

    await confirmationText(view);
    expect(singlePatchBody(fetchMock, story.id)).toEqual({
      acceptanceCriteria: [secondCriterion, firstCriterion, thirdCriterion],
    });
  });

  // Caso límite: quitar los dos criterios envía la lista vacía y la confirmación muestra que la
  // historia dejó de estar lista para planificar (FR-022, FR-028).
  it("Vaciar criterios", async () => {
    serveApplying(fetchMock, pendingStory);
    const view = await openForm(pendingStory.id);

    fireEvent.click(view.getByRole("button", { name: "Quitar criterio 1" }));
    fireEvent.click(view.getByRole("button", { name: "Quitar criterio 1" }));
    save(view);

    const text = await confirmationText(view);
    expect(text).toContain("No lista para planificar");
    expect(singlePatchBody(fetchMock, pendingStory.id)).toEqual({
      acceptanceCriteria: [],
    });
  });

  // Caso límite: una historia En progreso sigue siendo editable; corregir la descripción envía solo
  // la descripción y la confirmación conserva el estado En progreso (RN-14).
  it("Historia en progreso", async () => {
    const story: StoryDetail = { ...pendingStory, status: "in_progress" };
    serveApplying(fetchMock, story);
    const view = await openForm(story.id);

    const description = "Corrige el alcance: solo los sprints cerrados.";
    change(view, "Descripción", description);
    save(view);

    const text = await confirmationText(view);
    expect(text).toContain("En progreso");
    expect(singlePatchBody(fetchMock, story.id)).toEqual({ description });
  });

  // Caso límite: dos integrantes editan a la vez, uno la prioridad y otro la descripción. Cada vista
  // envía solo el campo que su integrante cambió, de modo que ninguna reescribe el campo de la otra
  // con el valor viejo que tenía precargado y la historia queda con los dos cambios (FR-025).
  it("Edición simultánea de campos distintos", async () => {
    const stored = serveApplying(fetchMock, pendingStory);
    const first = await openForm(pendingStory.id);
    const second = await openForm(pendingStory.id);

    const description = "Incluye los sprints cerrados del último trimestre.";
    change(first, "Prioridad", "high");
    change(second, "Descripción", description);
    save(first);
    save(second);

    await confirmationText(first);
    await confirmationText(second);
    const bodies = patchRequests(fetchMock).map((request) => request.body);
    expect(bodies).toHaveLength(2);
    expect(bodies).toContainEqual({ priority: "high" });
    expect(bodies).toContainEqual({ description });
    expect(stored().priority).toBe("high");
    expect(stored().description).toBe(description);
  });

  // Caso límite: dos integrantes reescriben a la vez los criterios. Cada vista envía su lista
  // completa, nunca un agregado parcial, de modo que prevalece entera la del último y las listas no
  // se fusionan; ninguna vista avisa de conflicto (FR-025).
  it("Edición simultánea de criterios", async () => {
    const stored = serveApplying(fetchMock, pendingStory);
    const first = await openForm(pendingStory.id);
    const second = await openForm(pendingStory.id);

    fireEvent.click(first.getByRole("button", { name: "Bajar criterio 1" }));
    change(second, "Criterio 2", thirdCriterion);
    save(first);
    await confirmationText(first);
    save(second);
    await confirmationText(second);

    const bodies = patchRequests(fetchMock).map((request) => request.body);
    expect(bodies).toEqual([
      { acceptanceCriteria: [secondCriterion, firstCriterion] },
      { acceptanceCriteria: [firstCriterion, thirdCriterion] },
    ]);
    expect(stored().acceptanceCriteria).toEqual([
      firstCriterion,
      thirdCriterion,
    ]);
    expect(first.queryByRole("alert")).toBeNull();
    expect(second.queryByRole("alert")).toBeNull();
  });

  // Caso de error: una historia comprometida en un sprint abierto no queda sin criterios; la vista
  // envía la lista vacía, muestra que primero hay que quitarla del sprint y no confirma nada
  // (FR-022, RN-18).
  it("Comprometida no queda sin criterios", async () => {
    const story: StoryDetail = { ...pendingStory, everAssignedToSprint: true };
    serve(fetchMock, story, () => ({
      status: 409,
      body: { error: committedMessage },
    }));
    const view = await openForm(story.id);

    fireEvent.click(view.getByRole("button", { name: "Quitar criterio 1" }));
    fireEvent.click(view.getByRole("button", { name: "Quitar criterio 1" }));
    save(view);

    const alert = await view.findByRole("alert");
    expect(alert.textContent).toContain(committedMessage);
    expect(singlePatchBody(fetchMock, story.id)).toEqual({
      acceptanceCriteria: [],
    });
    expect(view.queryByText("Historia actualizada")).toBeNull();
  });

  // Caso de error: un título vacío o de 151 caracteres, una descripción de 2001 o un criterio de 501
  // se marcan en el formulario junto a un cambio válido de prioridad, y no se envía nada: ningún
  // cambio parcial (FR-021, FR-023). La prioridad solo se elige entre Alta, Media y Baja, y si el
  // backend rechaza el cuerpo con 400 se muestran el motivo y el detalle junto al campo.
  it("Entrada inválida sin cambio parcial", async () => {
    const invalidInputs: { label: string; value: string }[] = [
      { label: "Título", value: "" },
      { label: "Título", value: "a".repeat(151) },
      { label: "Descripción", value: "d".repeat(2001) },
      { label: "Criterio 1", value: "c".repeat(501) },
    ];
    serveApplying(fetchMock, pendingStory);

    for (const { label, value } of invalidInputs) {
      const view = await openForm(pendingStory.id);
      change(view, "Prioridad", "high");
      change(view, label, value);
      save(view);

      await waitFor(() =>
        expect(control(view, label).getAttribute("aria-invalid")).toBe("true"),
      );
      expect(patchRequests(fetchMock)).toHaveLength(0);
      expect(view.queryByText("Historia actualizada")).toBeNull();
      cleanup();
    }

    const view = await openForm(pendingStory.id);
    const offered = Array.from(
      view
        .getByRole("combobox", { name: "Prioridad" })
        .querySelectorAll("option"),
    )
      .map((option) => option.value)
      .filter((value) => value !== "");
    expect(offered).toEqual(["high", "medium", "low"]);

    const priorityMessage =
      "La prioridad es obligatoria. Los valores admitidos son Alta, Media y Baja.";
    serve(fetchMock, pendingStory, () => ({
      status: 400,
      body: {
        error: invalidFieldsMessage,
        fields: [{ field: "priority", message: priorityMessage }],
      },
    }));
    change(view, "Prioridad", "medium");
    save(view);

    const alert = await view.findByRole("alert");
    expect(alert.textContent).toContain(invalidFieldsMessage);
    await waitFor(() =>
      expect(control(view, "Prioridad").getAttribute("aria-invalid")).toBe(
        "true",
      ),
    );
    expect(view.getAllByText(priorityMessage).length).toBeGreaterThan(0);
    expect(view.queryByText("Historia actualizada")).toBeNull();
  });

  // Caso de error: una historia Completada no se modifica; la vista muestra el motivo del rechazo y
  // no confirma nada (FR-030, RN-14).
  it("Historia completada", async () => {
    const story: StoryDetail = { ...pendingStory, status: "completed" };
    serve(fetchMock, story, () => ({
      status: 409,
      body: { error: completedMessage },
    }));
    const view = await openForm(story.id);

    change(view, "Título", "Exportar el reporte de velocidad a PDF y CSV");
    save(view);

    const alert = await view.findByRole("alert");
    expect(alert.textContent).toContain(completedMessage);
    expect(view.queryByText("Historia actualizada")).toBeNull();
  });

  // Caso de error: en un proyecto Finalizado el backlog es de solo lectura; la vista muestra el
  // motivo y no confirma nada (FR-048, RN-15).
  it("Proyecto finalizado", async () => {
    serve(fetchMock, pendingStory, () => ({
      status: 409,
      body: { error: readOnlyMessage },
    }));
    const view = await openForm(pendingStory.id);

    change(view, "Prioridad", "high");
    save(view);

    const alert = await view.findByRole("alert");
    expect(alert.textContent).toContain(readOnlyMessage);
    expect(view.queryByText("Historia actualizada")).toBeNull();
  });

  // Caso de error: la historia de un proyecto ajeno llega con el 404 de historia inexistente; la
  // vista muestra solo ese mensaje, sin formulario ni datos, y no envía ninguna modificación
  // (FR-047).
  it("Historia ajena", async () => {
    serve(
      fetchMock,
      pendingStory,
      () => ({ status: 500, body: { error: "inesperado" } }),
      { status: 404, body: { error: notFoundMessage } },
    );
    const view = renderEditView(pendingStory.id);

    const alert = await view.findByRole("alert");
    expect(alert.textContent).toContain(notFoundMessage);
    expect(view.queryByLabelText("Título")).toBeNull();
    expect(view.queryByRole("button", { name: "Guardar cambios" })).toBeNull();
    expect(view.queryByText(pendingStory.title)).toBeNull();
    expect(patchRequests(fetchMock)).toHaveLength(0);
  });

  // Caso de error: el formulario no ofrece pasar la historia a En progreso ni a Completada: no hay
  // control ni acción de estado y el único selector es la prioridad. Guardar un cambio de título
  // envía solo el título, nunca el estado (FR-027, RN-10).
  it("Cambio de estado no disponible", async () => {
    serveApplying(fetchMock, pendingStory);
    const view = await openForm(pendingStory.id);

    expect(view.queryByLabelText("Estado")).toBeNull();
    expect(view.getAllByRole("combobox").map((element) => element.id)).toEqual([
      control(view, "Prioridad").id,
    ]);
    expect(
      view.queryByRole("button", { name: /en progreso|completada|estado/i }),
    ).toBeNull();

    const title = "Exportar a PDF el reporte de velocidad del equipo";
    change(view, "Título", title);
    save(view);

    const text = await confirmationText(view);
    expect(text).toContain("Pendiente");
    expect(singlePatchBody(fetchMock, pendingStory.id)).toEqual({ title });
  });

  // Lo que hace posible FR-025 desde la interfaz: cada campo viaja solo cuando la persona lo cambió,
  // también al borrar la descripción, que viaja vacía o en null (FR-020).
  it("Envía solo los campos que la persona cambió", async () => {
    const edits: {
      apply: (view: Scope) => void;
      expected: (body: unknown) => void;
    }[] = [
      {
        apply: (view) => change(view, "Título", "Reporte de velocidad en PDF"),
        expected: (body) =>
          expect(body).toEqual({ title: "Reporte de velocidad en PDF" }),
      },
      {
        apply: (view) => change(view, "Prioridad", "medium"),
        expected: (body) => expect(body).toEqual({ priority: "medium" }),
      },
      {
        apply: (view) => change(view, "Criterio 1", thirdCriterion),
        expected: (body) =>
          expect(body).toEqual({
            acceptanceCriteria: [thirdCriterion, secondCriterion],
          }),
      },
      {
        apply: (view) => change(view, "Descripción", ""),
        expected: (body) => {
          if (!isPatchBody(body)) {
            throw new Error("El cuerpo del PATCH debe ser un objeto");
          }
          expect(Object.keys(body)).toEqual(["description"]);
          expect([null, ""]).toContain(body.description);
        },
      },
    ];

    for (const { apply, expected } of edits) {
      fetchMock.mockClear();
      serveApplying(fetchMock, pendingStory);
      const view = await openForm(pendingStory.id);

      apply(view);
      save(view);

      await confirmationText(view);
      expected(singlePatchBody(fetchMock, pendingStory.id));
      cleanup();
    }
  });
});

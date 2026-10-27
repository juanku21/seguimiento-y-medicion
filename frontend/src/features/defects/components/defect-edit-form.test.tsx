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
import { DefectEditForm } from "@/features/defects/components/defect-edit-form";
import { writeToken } from "@/lib/session-storage";
import type { Defect, DefectStatus } from "@/types/defect";
import type { SprintListItem, SprintStatus } from "@/types/sprint";

// Pruebas del formulario de corrección de un defecto (T050). Atraviesan el componente y el cliente
// HTTP real; solo se sustituye fetch, que es la frontera con el backend.
//
// Contrato que fijan estas pruebas para DefectEditForm:
// - Recibe defect (el defecto vigente) y onUpdated (se avisa con el defecto devuelto por el PATCH).
// - Campos por etiqueta: "Descripción", "Severidad", "Historia" y "Sprint de detección", con los
//   valores actuales del defecto. Historia y sprint se ofrecen con las historias del backlog y los
//   sprints Activos y Cerrados (mismas consultas que el alta).
// - Descripción y severidad están habilitadas en Abierto, En progreso y Resuelto; historia y sprint
//   de detección solo en Abierto (disabled en los demás estados).
// - El botón "Guardar cambios" envía PATCH /api/v1/defects/{id} solo con los campos cambiados.
// - El 400 se muestra junto al campo (aria-describedby); el 404 y el 409 links_frozen, con
//   role="alert" y el mensaje del backend. Ningún rechazo borra lo escrito.

const route = vi.hoisted(() => ({ query: "" }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    refresh: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(route.query),
}));

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const defectId = "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a";
const storyOneId = "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10";
const storyTwoId = "5a7b9c1d-2e3f-4a5b-8c6d-7e8f9a0b1c2d";
const originalDescription = "El total del reporte no suma los descuentos";
const newDescription = "El total no suma los descuentos por volumen";

function sprint(
  id: string,
  name: string,
  status: SprintStatus,
): SprintListItem {
  return {
    id,
    name,
    goal: "Objetivo",
    status,
    startDate: "2026-10-01",
    endDate: "2026-10-14",
    startedAt: null,
    closedAt: null,
    storyCount: 0,
    results: null,
  };
}

const activeSprint = sprint(
  "a1111111-0000-4000-8000-000000000001",
  "Sprint 3",
  "active",
);
const closedSprint = sprint(
  "c2222222-0000-4000-8000-000000000002",
  "Sprint 2",
  "closed",
);

const stories = [
  {
    id: storyOneId,
    title: "Reporte de ventas mensual",
    priority: "high",
    status: "completed",
    storyPoints: 5,
    acceptanceCriteriaCount: 2,
    readyToPlan: true,
  },
  {
    id: storyTwoId,
    title: "Alta de clientes",
    priority: "medium",
    status: "pending",
    storyPoints: 3,
    acceptanceCriteriaCount: 1,
    readyToPlan: true,
  },
];

const user = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};

// Defecto vigente en el estado indicado, con la historia uno y el sprint Activo.
function defectIn(status: DefectStatus): Defect {
  return {
    id: defectId,
    projectId,
    description: originalDescription,
    severity: "high",
    status,
    story: { id: storyOneId, title: "Reporte de ventas mensual" },
    detectedSprint: {
      id: activeSprint.id,
      name: activeSprint.name,
      status: "active",
      startDate: "2026-10-01",
    },
    resolvedSprint: null,
    createdBy: user,
    updatedBy: user,
    statusChangedAt: "2026-10-24T13:00:00Z",
    statusChangedBy: user,
    createdAt: "2026-10-24T13:00:00Z",
    updatedAt: "2026-10-24T13:00:00Z",
  };
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Backend simulado: historias, sprints y el PATCH, cuya respuesta reemplaza cada prueba.
function backend(onPatch: (body: Record<string, unknown>) => Response) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    if (init?.method === "PATCH") {
      return onPatch(JSON.parse(String(init.body)) as Record<string, unknown>);
    }
    if (url.pathname.endsWith("/sprints")) {
      const wanted = url.searchParams.getAll("status");
      const listed = [activeSprint, closedSprint].filter(
        (item) => wanted.length === 0 || wanted.includes(item.status),
      );
      return jsonResponse(200, {
        sprints: listed,
        appliedFilters: { status: wanted },
        emptyReason: null,
      });
    }
    return jsonResponse(200, {
      stories,
      appliedFilters: { status: [], priority: [] },
      emptyReason: null,
    });
  };
}

// Respuesta 200 del PATCH: el defecto vigente con lo enviado aplicado.
function updatedBy(status: DefectStatus) {
  return (body: Record<string, unknown>) =>
    jsonResponse(200, {
      ...defectIn(status),
      ...(body.description !== undefined
        ? { description: body.description }
        : {}),
      ...(body.severity !== undefined ? { severity: body.severity } : {}),
    });
}

function describedText(element: HTMLElement): string {
  const ids = (element.getAttribute("aria-describedby") ?? "")
    .split(/\s+/)
    .filter(Boolean);
  return ids
    .map((id) => document.getElementById(id)?.textContent ?? "")
    .join(" ");
}

function getFields() {
  return {
    description: screen.getByLabelText<HTMLTextAreaElement>(/^descripci[oó]n/i),
    severity: screen.getByLabelText<HTMLSelectElement>(/^severidad/i),
    story: screen.getByLabelText<HTMLSelectElement>(/^historia/i),
    sprint: screen.getByLabelText<HTMLSelectElement>(
      /^sprint de detecci[oó]n/i,
    ),
    submit: screen.getByRole<HTMLButtonElement>("button", {
      name: /guardar cambios/i,
    }),
  };
}

// Monta el formulario y espera a que se carguen las opciones de historia y sprint.
async function renderLoaded(status: DefectStatus = "open") {
  const onUpdated = vi.fn();
  render(<DefectEditForm defect={defectIn(status)} onUpdated={onUpdated} />);
  await waitFor(() => {
    const fields = getFields();
    expect(fields.story.options.length).toBeGreaterThan(stories.length);
    expect(fields.sprint.options.length).toBeGreaterThan(1);
  });
  return { fields: getFields(), onUpdated };
}

// Extrae la URL, la sesión y el cuerpo JSON de la llamada PATCH, o undefined si no hubo ninguna.
function patchRequest(fetchMock: FetchMock) {
  const call = fetchMock.mock.calls.find(
    ([, init]) => init?.method === "PATCH",
  );
  if (call === undefined) {
    return undefined;
  }
  const [input, init] = call;
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return {
    url,
    authorization: new Headers(init?.headers).get("Authorization"),
    body: JSON.parse(String(init?.body)) as Record<string, unknown>,
  };
}

describe("007/US4 - Corregir los datos de un defecto", () => {
  let fetchMock: FetchMock;

  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    route.query = "";
    fetchMock = vi.fn<typeof fetch>(backend(updatedBy("open")));
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: descripción y severidad se corrigen en Abierto, En progreso y Resuelto.
  it("la descripción y la severidad son editables en Abierto, En progreso y Resuelto", async () => {
    for (const status of ["open", "in_progress", "resolved"] as const) {
      const { fields } = await renderLoaded(status);
      expect(fields.description.disabled).toBe(false);
      expect(fields.severity.disabled).toBe(false);
      expect(fields.description.value).toBe(originalDescription);
      expect(fields.severity.value).toBe("high");
      cleanup();
    }
  });

  // Escenario: historia y sprint de detección se corrigen solo mientras el defecto está Abierto.
  it("la historia y el sprint de detección son editables solo en Abierto", async () => {
    const open = await renderLoaded("open");
    expect(open.fields.story.disabled).toBe(false);
    expect(open.fields.sprint.disabled).toBe(false);
    expect(open.fields.story.value).toBe(storyOneId);
    expect(open.fields.sprint.value).toBe(activeSprint.id);
    cleanup();

    for (const status of ["in_progress", "resolved"] as const) {
      const { fields } = await renderLoaded(status);
      expect(fields.story.disabled).toBe(true);
      expect(fields.sprint.disabled).toBe(true);
      cleanup();
    }
  });

  // Escenario: se corrige solo la descripción y viaja únicamente ese campo, con la sesión.
  it("envía solo la descripción cuando es el único campo cambiado", async () => {
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.description, {
      target: { value: newDescription },
    });
    fireEvent.click(fields.submit);

    await waitFor(() => expect(onUpdated).toHaveBeenCalledTimes(1));
    const request = patchRequest(fetchMock);
    expect(request?.url.endsWith(`/api/v1/defects/${defectId}`)).toBe(true);
    expect(request?.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request?.body).toEqual({ description: newDescription });
    expect(onUpdated).toHaveBeenCalledWith(
      expect.objectContaining({ description: newDescription }),
    );
  });

  // Escenario: se corrigen severidad e historia de un defecto Abierto y viajan solo esos campos.
  it("envía solo los campos cambiados cuando se corrigen severidad e historia", async () => {
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.severity, { target: { value: "critical" } });
    fireEvent.change(fields.story, { target: { value: storyTwoId } });
    fireEvent.click(fields.submit);

    await waitFor(() => expect(onUpdated).toHaveBeenCalledTimes(1));
    expect(patchRequest(fetchMock)?.body).toEqual({
      severity: "critical",
      storyId: storyTwoId,
    });
  });

  // Escenario: corregir el sprint de detección por uno Cerrado envía solo ese campo.
  it("envía solo el sprint de detección cuando se corrige por un sprint Cerrado", async () => {
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.sprint, { target: { value: closedSprint.id } });
    fireEvent.click(fields.submit);

    await waitFor(() => expect(onUpdated).toHaveBeenCalledTimes(1));
    expect(patchRequest(fetchMock)?.body).toEqual({
      detectedSprintId: closedSprint.id,
    });
  });

  // Escenario: la descripción de un defecto Resuelto se corrige sin enviar los vínculos.
  it("corrige la descripción de un defecto Resuelto sin enviar los vínculos", async () => {
    fetchMock.mockImplementation(backend(updatedBy("resolved")));
    const { fields, onUpdated } = await renderLoaded("resolved");
    fireEvent.change(fields.description, {
      target: { value: newDescription },
    });
    fireEvent.click(fields.submit);

    await waitFor(() => expect(onUpdated).toHaveBeenCalledTimes(1));
    expect(patchRequest(fetchMock)?.body).toEqual({
      description: newDescription,
    });
  });

  // Escenario: el 400 se muestra junto al campo y lo escrito se conserva.
  it("el mensaje del 400 se muestra junto al campo sin perder lo escrito", async () => {
    fetchMock.mockImplementation(
      backend(() =>
        jsonResponse(400, {
          error: "Hay campos inválidos en la solicitud.",
          fields: [
            {
              field: "description",
              message: "La descripción admite como máximo 2000 caracteres.",
            },
          ],
        }),
      ),
    );
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.description, {
      target: { value: newDescription },
    });
    fireEvent.change(fields.severity, { target: { value: "low" } });
    fireEvent.click(fields.submit);

    await waitFor(() =>
      expect(fields.description.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.description)).toContain(
      "La descripción admite como máximo 2000 caracteres.",
    );
    expect(fields.description.value).toBe(newDescription);
    expect(fields.severity.value).toBe("low");
    expect(onUpdated).not.toHaveBeenCalled();
  });

  // Escenario: el 404 de una historia inexistente se muestra y lo escrito se conserva.
  it("el mensaje del 404 se muestra sin perder lo escrito", async () => {
    fetchMock.mockImplementation(
      backend(() => jsonResponse(404, { error: "La historia no existe." })),
    );
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.story, { target: { value: storyTwoId } });
    fireEvent.change(fields.description, {
      target: { value: newDescription },
    });
    fireEvent.click(fields.submit);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("La historia no existe.");
    expect(fields.story.value).toBe(storyTwoId);
    expect(fields.description.value).toBe(newDescription);
    expect(onUpdated).not.toHaveBeenCalled();
  });

  // Escenario: el 409 links_frozen se muestra y lo escrito se conserva.
  it("el mensaje de links_frozen se muestra sin perder lo escrito", async () => {
    const message =
      "La historia relacionada y el sprint de detección solo se corrigen mientras el defecto está Abierto.";
    fetchMock.mockImplementation(
      backend(() =>
        jsonResponse(409, {
          code: "links_frozen",
          error: message,
          currentStatus: "in_progress",
          allowedStatuses: ["resolved", "discarded"],
        }),
      ),
    );
    const { fields, onUpdated } = await renderLoaded("open");
    fireEvent.change(fields.story, { target: { value: storyTwoId } });
    fireEvent.change(fields.description, {
      target: { value: newDescription },
    });
    fireEvent.click(fields.submit);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(fields.story.value).toBe(storyTwoId);
    expect(fields.description.value).toBe(newDescription);
    expect(onUpdated).not.toHaveBeenCalled();
  });
});

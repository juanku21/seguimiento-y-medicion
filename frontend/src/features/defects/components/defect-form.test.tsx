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
import { DefectForm } from "@/features/defects/components/defect-form";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type { Defect, DefectConflictCode } from "@/types/defect";
import type { SprintListItem, SprintStatus } from "@/types/sprint";

// Pruebas del formulario de registro de un defecto (T013). Atraviesan el componente, la validación
// y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el backend.
//
// Contrato de accesibilidad y de llamadas que fijan estas pruebas para DefectForm:
// - Recibe projectId y onCreated; la historia preseleccionada llega en el parámetro de consulta
//   ?storyId= (useSearchParams).
// - Campos por etiqueta: "Descripción" (texto), "Severidad" (select con Crítica, Alta, Media y Baja),
//   "Historia" (select con el título de cada historia del backlog) y "Sprint de detección" (select
//   con el nombre de cada sprint). Los selects de historia y sprint tienen una primera opción vacía
//   sin valor. No hay ningún campo de autor (FR-056).
// - Carga las historias con GET /api/v1/projects/{id}/stories y los sprints con
//   GET /api/v1/projects/{id}/sprints?status=active&status=closed (sección 14 de research.md).
// - Propone el sprint Activo si existe; sin Activo no preselecciona ninguno.
// - Sin sprints Activos ni Cerrados muestra el aviso "Todavía no hay ningún sprint Activo ni Cerrado"
//   y deshabilita el botón de envío.
// - El envío se hace con el botón "Registrar defecto", con POST /api/v1/projects/{id}/defects.
// - Los errores de cada campo, propios o del 400 del backend, se asocian al campo con
//   aria-describedby; un campo con error lleva aria-invalid="true".
// - Los rechazos generales (404, 409) se muestran con role="alert" con el mensaje del backend.
// - El alta exitosa se informa con onCreated y el defecto devuelto por el backend.

// Estado del enrutador de la prueba: el parámetro de consulta de la página que monta el formulario.
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
const defectsPath = `/api/v1/projects/${projectId}/defects`;

const storyOneId = "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10";
const storyTwoId = "5a7b9c1d-2e3f-4a5b-8c6d-7e8f9a0b1c2d";

// Datos del caso normal de la spec (US1, escenario 1).
const normalDescription = "El total del reporte no suma los descuentos";

// Construye un sprint del listado de la 004 con los datos mínimos que usa el formulario.
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
const closedOne = sprint(
  "c2222222-0000-4000-8000-000000000002",
  "Sprint 2",
  "closed",
);
const closedTwo = sprint(
  "c3333333-0000-4000-8000-000000000003",
  "Sprint 1",
  "closed",
);
const plannedSprint = sprint(
  "b4444444-0000-4000-8000-000000000004",
  "Sprint 4",
  "planned",
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

// Respuesta 201 según Defect de contracts/openapi.yaml, armada con el cuerpo recibido.
function createdDefect(body: Record<string, unknown>): Defect {
  const user = {
    id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
    fullName: "Ana Pérez",
  };
  return {
    id: "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a",
    projectId,
    description: String(body.description).trim(),
    severity: body.severity as Defect["severity"],
    status: "open",
    story: { id: String(body.storyId), title: "Reporte de ventas mensual" },
    detectedSprint: {
      id: String(body.detectedSprintId),
      name: "Sprint 3",
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

// Rechazo 409 según DefectConflictResponse de contracts/openapi.yaml para los códigos sin estado.
type ConflictBody = {
  code: DefectConflictCode;
  error: string;
  currentStatus: null;
  allowedStatuses: never[];
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body:
    | Defect
    | ErrorResponse
    | ValidationErrorResponse
    | ConflictBody
    | Record<string, unknown>,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Backend simulado: devuelve las historias, los sprints que cumplen el filtro status de la consulta
// (como lo hace la 004) y acepta el alta. Las pruebas reemplazan el alta cuando verifican un rechazo.
function backend(
  sprints: SprintListItem[],
  onCreate: (body: Record<string, unknown>) => Response = (body) =>
    jsonResponse(201, createdDefect(body)),
) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    if (init?.method === "POST") {
      return onCreate(JSON.parse(String(init.body)) as Record<string, unknown>);
    }
    if (url.pathname.endsWith("/sprints")) {
      const wanted = url.searchParams.getAll("status");
      const listed = sprints.filter(
        (item) => wanted.length === 0 || wanted.includes(item.status),
      );
      return jsonResponse(200, {
        sprints: listed,
        appliedFilters: { status: wanted },
        emptyReason: listed.length === 0 ? "no_matches" : null,
      });
    }
    return jsonResponse(200, {
      stories,
      appliedFilters: { status: [], priority: [] },
      emptyReason: null,
    });
  };
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

// Localiza los campos y el botón de envío del formulario.
function getFields() {
  return {
    description: screen.getByLabelText<HTMLInputElement | HTMLTextAreaElement>(
      /^descripci[oó]n/i,
    ),
    severity: screen.getByLabelText<HTMLSelectElement>(/^severidad/i),
    story: screen.getByLabelText<HTMLSelectElement>(/^historia/i),
    sprint: screen.getByLabelText<HTMLSelectElement>(
      /^sprint de detecci[oó]n/i,
    ),
    submit: screen.getByRole<HTMLButtonElement>("button", {
      name: /registrar defecto/i,
    }),
  };
}

type Fields = ReturnType<typeof getFields>;

// Nombres de las opciones con valor de un select, sin la opción vacía inicial.
function optionNames(select: HTMLSelectElement): string[] {
  return Array.from(select.options)
    .filter((option) => option.value !== "")
    .map((option) => option.textContent ?? "");
}

// Monta el formulario y espera a que terminen de cargar las historias y los sprints.
async function renderLoaded(expectedSprints: number) {
  const onCreated = vi.fn();
  render(<DefectForm projectId={projectId} onCreated={onCreated} />);
  await waitFor(() => {
    const fields = getFields();
    expect(optionNames(fields.story)).toHaveLength(stories.length);
    expect(optionNames(fields.sprint)).toHaveLength(expectedSprints);
  });
  return { fields: getFields(), onCreated };
}

// Completa descripción, severidad y, si se indica, historia, y envía el formulario.
function fillAndSubmit(
  fields: Fields,
  values: { description: string; severity: string; story?: string },
) {
  fireEvent.change(fields.description, {
    target: { value: values.description },
  });
  fireEvent.change(fields.severity, { target: { value: values.severity } });
  if (values.story !== undefined) {
    fireEvent.change(fields.story, { target: { value: values.story } });
  }
  fireEvent.click(fields.submit);
}

// Extrae la URL, la cabecera Authorization y el cuerpo JSON de la llamada de alta, o undefined si
// el formulario no llegó a enviar nada.
function createRequest(fetchMock: FetchMock) {
  const call = fetchMock.mock.calls.find(([, init]) => init?.method === "POST");
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

describe("007/US1 - Registrar un defecto", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña, sin parámetros de consulta y con el
  // backend simulado del caso normal (un sprint Activo y dos Cerrados).
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    route.query = "";
    fetchMock = vi.fn<typeof fetch>(
      backend([activeSprint, closedOne, closedTwo]),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: con el sprint Activo propuesto, se escribe la descripción, se elige la severidad y
  // la historia y se confirma (SC-001: cinco pasos). Se envía POST con la sesión y solo los cuatro
  // campos del defecto, sin autor, y se avisa el alta con el defecto del backend.
  it("el integrante registra un defecto con severidad Alta en el sprint Activo", async () => {
    const { fields, onCreated } = await renderLoaded(3);
    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "high",
      story: storyOneId,
    });

    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    const request = createRequest(fetchMock);
    expect(request?.url.endsWith(defectsPath)).toBe(true);
    expect(request?.authorization).toBe(`Bearer ${sessionToken}`);
    expect(request?.body).toEqual({
      description: normalDescription,
      severity: "high",
      storyId: storyOneId,
      detectedSprintId: activeSprint.id,
    });
    expect(onCreated).toHaveBeenCalledWith(createdDefect(request?.body ?? {}));
  });

  // Caso alternativo: el formulario pide solo los sprints Activos y Cerrados, propone el Activo y
  // permite cambiarlo por un Cerrado.
  it("el sprint Activo viene propuesto por defecto y se puede cambiar por uno Cerrado", async () => {
    const { fields, onCreated } = await renderLoaded(3);
    const sprintsCall = fetchMock.mock.calls.find(([input]) =>
      String(input).includes("/sprints"),
    );
    const query = new URL(String(sprintsCall?.[0]), "http://localhost")
      .searchParams;
    expect(query.getAll("status").sort()).toEqual(["active", "closed"]);

    expect(fields.sprint.value).toBe(activeSprint.id);
    expect(optionNames(fields.sprint)).toEqual([
      "Sprint 3",
      "Sprint 2",
      "Sprint 1",
    ]);

    fireEvent.change(fields.sprint, { target: { value: closedOne.id } });
    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "low",
      story: storyOneId,
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(createRequest(fetchMock)?.body.detectedSprintId).toBe(closedOne.id);
  });

  // Caso de error: un sprint Planificado nunca es sprint de detección; el selector no lo ofrece
  // aunque el proyecto lo tenga.
  it("el selector de sprint de detección no ofrece ningún sprint Planificado", async () => {
    fetchMock.mockImplementation(
      backend([activeSprint, closedOne, plannedSprint]),
    );
    const { fields } = await renderLoaded(2);
    expect(optionNames(fields.sprint)).not.toContain(plannedSprint.name);
    expect(optionNames(fields.sprint)).toEqual(["Sprint 3", "Sprint 2"]);
  });

  // Caso alternativo: sin sprint Activo pero con dos Cerrados, no se propone ninguno y el
  // integrante elige uno de los Cerrados.
  it("sin sprint Activo no se propone ningún sprint y se elige uno Cerrado", async () => {
    fetchMock.mockImplementation(backend([closedOne, closedTwo]));
    const { fields, onCreated } = await renderLoaded(2);
    expect(fields.sprint.value).toBe("");

    fireEvent.change(fields.sprint, { target: { value: closedTwo.id } });
    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "critical",
      story: storyTwoId,
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(createRequest(fetchMock)?.body.detectedSprintId).toBe(closedTwo.id);
  });

  // Caso de error: un proyecto que nunca inició un sprint muestra el aviso y no permite enviar.
  it("sin sprints Activos ni Cerrados se muestra el aviso y el envío queda deshabilitado", async () => {
    fetchMock.mockImplementation(backend([plannedSprint]));
    render(<DefectForm projectId={projectId} />);

    expect(
      await screen.findByText(
        /todavía no hay ningún sprint activo ni cerrado/i,
      ),
    ).toBeTruthy();
    expect(getFields().submit.disabled).toBe(true);
    expect(createRequest(fetchMock)).toBeUndefined();
  });

  // Caso alternativo: al llegar desde una historia con ?storyId=, esa historia viene elegida y el
  // registro lleva un paso menos (SC-001).
  it("la historia viene preseleccionada cuando se llega con storyId", async () => {
    route.query = `storyId=${storyTwoId}`;
    const { fields, onCreated } = await renderLoaded(3);
    expect(fields.story.value).toBe(storyTwoId);

    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "medium",
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(createRequest(fetchMock)?.body.storyId).toBe(storyTwoId);
  });

  // Caso de error: el 400 del backend informa varios campos a la vez y cada mensaje queda junto a
  // su campo, sin avisar un alta.
  it("los mensajes de campo del 400 se muestran junto a cada campo", async () => {
    const sprintMessage =
      "Un defecto solo se detecta en un sprint Activo o Cerrado.";
    fetchMock.mockImplementation(
      backend([activeSprint, closedOne, closedTwo], () =>
        jsonResponse(400, {
          error: "Hay campos inválidos en la solicitud.",
          fields: [
            { field: "description", message: "La descripción es obligatoria." },
            { field: "detectedSprintId", message: sprintMessage },
          ],
        }),
      ),
    );
    const { fields, onCreated } = await renderLoaded(3);
    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "high",
      story: storyOneId,
    });

    await waitFor(() =>
      expect(fields.description.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.description)).toContain(
      "La descripción es obligatoria.",
    );
    expect(fields.sprint.getAttribute("aria-invalid")).toBe("true");
    expect(describedText(fields.sprint)).toContain(sprintMessage);
    expect(onCreated).not.toHaveBeenCalled();
  });

  // Caso de error: la descripción vacía o solo con espacios se rechaza antes de enviar, con el
  // mensaje junto al campo.
  it("la descripción vacía se rechaza antes de enviar", async () => {
    const { fields, onCreated } = await renderLoaded(3);
    fillAndSubmit(fields, {
      description: "   ",
      severity: "high",
      story: storyOneId,
    });

    await waitFor(() =>
      expect(fields.description.getAttribute("aria-invalid")).toBe("true"),
    );
    expect(describedText(fields.description)).toContain(
      "La descripción es obligatoria.",
    );
    expect(createRequest(fetchMock)).toBeUndefined();
    expect(onCreated).not.toHaveBeenCalled();
  });

  // Caso de error: en un proyecto Finalizado el backend responde 409 read_only y el formulario
  // muestra el mensaje de solo lectura, sin avisar un alta.
  it("el proyecto Finalizado muestra el mensaje de solo lectura", async () => {
    const message =
      "El proyecto está finalizado y sus defectos son de solo lectura.";
    fetchMock.mockImplementation(
      backend([activeSprint, closedOne, closedTwo], () =>
        jsonResponse(409, {
          code: "read_only",
          error: message,
          currentStatus: null,
          allowedStatuses: [],
        }),
      ),
    );
    const { fields, onCreated } = await renderLoaded(3);
    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "high",
      story: storyOneId,
    });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(message);
    expect(onCreated).not.toHaveBeenCalled();
  });

  // Caso de error: el formulario no ofrece forma de indicar a otra persona como autor; sus campos
  // son descripción, severidad, historia y sprint, y el cuerpo enviado no lleva datos de autoría.
  it("el formulario no tiene campo de autor", async () => {
    const { fields, onCreated } = await renderLoaded(3);
    expect(
      screen.queryByLabelText(
        /autor|integrante|persona|usuario|registrado por/i,
      ),
    ).toBeNull();
    expect(document.querySelectorAll("input, textarea, select")).toHaveLength(
      4,
    );

    fillAndSubmit(fields, {
      description: normalDescription,
      severity: "high",
      story: storyOneId,
    });
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1));
    expect(Object.keys(createRequest(fetchMock)?.body ?? {}).sort()).toEqual([
      "description",
      "detectedSprintId",
      "severity",
      "storyId",
    ]);
  });
});
